package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Compile-time interface check.
var _ Client = (*HTTPClient)(nil)

// ErrNotImplemented is returned for features that are not yet wired up.
var ErrNotImplemented = errors.New("a2a: not implemented")

// HTTPClient implements the Client interface using HTTP/JSON-RPC.
type HTTPClient struct {
	http      *http.Client
	requestID atomic.Int64
}

// ClientOption configures an HTTPClient.
type ClientOption func(*HTTPClient)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *HTTPClient) {
		c.http.Timeout = d
	}
}

// WithHTTPClient replaces the underlying *http.Client entirely.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *HTTPClient) {
		c.http = hc
	}
}

// NewHTTPClient creates a new A2A HTTP client.
func NewHTTPClient(opts ...ClientOption) *HTTPClient {
	c := &HTTPClient{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SendMessage sends a message to an agent via the message/send JSON-RPC method.
func (c *HTTPClient) SendMessage(ctx context.Context, endpoint string, req SendMessageRequest) (*Task, error) {
	var task Task
	if err := c.call(ctx, endpoint, MethodSendMessage, req, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTask retrieves a task by ID via the tasks/get JSON-RPC method.
func (c *HTTPClient) GetTask(ctx context.Context, endpoint string, req GetTaskRequest) (*Task, error) {
	var task Task
	if err := c.call(ctx, endpoint, MethodGetTask, req, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// ListTasks queries tasks via the tasks/list JSON-RPC method.
func (c *HTTPClient) ListTasks(ctx context.Context, endpoint string, req ListTasksRequest) (*ListTasksResponse, error) {
	var resp ListTasksResponse
	if err := c.call(ctx, endpoint, MethodListTasks, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelTask cancels a running task via the tasks/cancel JSON-RPC method.
func (c *HTTPClient) CancelTask(ctx context.Context, endpoint string, req CancelTaskRequest) (*Task, error) {
	var task Task
	if err := c.call(ctx, endpoint, MethodCancelTask, req, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// SubscribeToTask opens an SSE stream for task updates.
// This is a stub implementation; SSE streaming will be wired in T-04.05.
func (c *HTTPClient) SubscribeToTask(ctx context.Context, endpoint string, taskID string) (<-chan StreamEvent, error) {
	return nil, ErrNotImplemented
}

// DiscoverAgent fetches the Agent Card from the well-known URI.
func (c *HTTPClient) DiscoverAgent(ctx context.Context, baseURL string) (*AgentCard, error) {
	url := strings.TrimRight(baseURL, "/") + "/.well-known/agent-card.json"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("a2a: create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("a2a: discover agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("a2a: discover agent: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("a2a: decode agent card: %w", err)
	}
	return &card, nil
}

// nextID returns a monotonically increasing request ID for JSON-RPC calls.
func (c *HTTPClient) nextID() int64 {
	return c.requestID.Add(1)
}

// call performs a JSON-RPC 2.0 call over HTTP POST.
func (c *HTTPClient) call(ctx context.Context, endpoint, method string, params any, result any) error {
	// Marshal the params.
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("a2a: marshal params: %w", err)
	}

	// Build the JSON-RPC request envelope.
	rpcReq := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.nextID(),
		Method:  method,
		Params:  paramsJSON,
	}

	body, err := json.Marshal(rpcReq)
	if err != nil {
		return fmt.Errorf("a2a: marshal request: %w", err)
	}

	// Create the HTTP request.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("a2a: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Execute the HTTP request.
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("a2a: %s: %w", method, err)
	}
	defer resp.Body.Close()

	// Read the response body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("a2a: read response: %w", err)
	}

	// Check HTTP-level errors.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("a2a: %s: HTTP %d: %s", method, resp.StatusCode, string(respBody))
	}

	// Decode JSON-RPC response.
	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return fmt.Errorf("a2a: decode response: %w", err)
	}

	// Check JSON-RPC-level errors.
	if rpcResp.Error != nil {
		return &RPCError{
			Method:  method,
			Code:    rpcResp.Error.Code,
			Message: rpcResp.Error.Message,
			Data:    rpcResp.Error.Data,
		}
	}

	// Unmarshal the result into the caller's target.
	if result != nil && rpcResp.Result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("a2a: decode result: %w", err)
		}
	}

	return nil
}

// RPCError represents a JSON-RPC error returned by a remote agent.
type RPCError struct {
	Method  string
	Code    int
	Message string
	Data    json.RawMessage
}

// Error implements the error interface.
func (e *RPCError) Error() string {
	if len(e.Data) > 0 {
		return fmt.Sprintf("a2a: %s: rpc error %d: %s (data: %s)", e.Method, e.Code, e.Message, string(e.Data))
	}
	return fmt.Sprintf("a2a: %s: rpc error %d: %s", e.Method, e.Code, e.Message)
}
