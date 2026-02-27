package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock Handler
// ---------------------------------------------------------------------------

type mockHandler struct {
	sendMessage func(ctx context.Context, req SendMessageRequest) (*Task, error)
	getTask     func(ctx context.Context, req GetTaskRequest) (*Task, error)
	listTasks   func(ctx context.Context, req ListTasksRequest) (*ListTasksResponse, error)
	cancelTask  func(ctx context.Context, req CancelTaskRequest) (*Task, error)
}

func (m *mockHandler) HandleSendMessage(ctx context.Context, req SendMessageRequest) (*Task, error) {
	if m.sendMessage != nil {
		return m.sendMessage(ctx, req)
	}
	return nil, fmt.Errorf("sendMessage not implemented")
}

func (m *mockHandler) HandleGetTask(ctx context.Context, req GetTaskRequest) (*Task, error) {
	if m.getTask != nil {
		return m.getTask(ctx, req)
	}
	return nil, fmt.Errorf("getTask not implemented")
}

func (m *mockHandler) HandleListTasks(ctx context.Context, req ListTasksRequest) (*ListTasksResponse, error) {
	if m.listTasks != nil {
		return m.listTasks(ctx, req)
	}
	return nil, fmt.Errorf("listTasks not implemented")
}

func (m *mockHandler) HandleCancelTask(ctx context.Context, req CancelTaskRequest) (*Task, error) {
	if m.cancelTask != nil {
		return m.cancelTask(ctx, req)
	}
	return nil, fmt.Errorf("cancelTask not implemented")
}

// ---------------------------------------------------------------------------
// Test helper
// ---------------------------------------------------------------------------

func startTestServer(t *testing.T, handler Handler, card AgentCard) (string, *Server) {
	t.Helper()

	srv := NewServer(card, handler)

	// Grab a random available port, then release it so the server can bind.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	require.NoError(t, srv.Start(context.Background(), addr))

	// Poll until the server is accepting connections (max 2 s).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.Dial("tcp", addr)
		if dialErr == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Cleanup(func() { srv.Stop(context.Background()) })
	return "http://" + addr, srv
}

func testCard() AgentCard {
	return AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		Version:     "0.1.0",
		Skills: []AgentSkill{
			{
				ID:          "echo",
				Name:        "echo",
				Description: "Echoes messages back",
				Tags:        []string{"test"},
			},
		},
	}
}

// postJSONRPC sends a JSON-RPC request and decodes the response.
func postJSONRPC(t *testing.T, baseURL string, method string, id any, params any) JSONRPCResponse {
	t.Helper()

	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		require.NoError(t, err)
		rawParams = b
	}

	reqBody := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rpcResp))
	return rpcResp
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestServerAgentCard(t *testing.T) {
	card := testCard()
	baseURL, _ := startTestServer(t, &mockHandler{}, card)

	resp, err := http.Get(baseURL + "/.well-known/agent-card.json")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var got AgentCard
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, card.Name, got.Name)
	assert.Equal(t, card.Description, got.Description)
	assert.Equal(t, card.Version, got.Version)
	require.Len(t, got.Skills, 1)
	assert.Equal(t, "echo", got.Skills[0].ID)
}

func TestServerSendMessage(t *testing.T) {
	handler := &mockHandler{
		sendMessage: func(ctx context.Context, req SendMessageRequest) (*Task, error) {
			return &Task{
				ID:        "task-1",
				ContextID: req.Message.ContextID,
				Status: TaskStatus{
					State:     TaskStateSubmitted,
					Timestamp: time.Now(),
				},
			}, nil
		},
	}

	baseURL, _ := startTestServer(t, handler, testCard())

	params := SendMessageRequest{
		Message: Message{
			MessageID: "msg-1",
			ContextID: "ctx-1",
			Role:      RoleUser,
			Parts:     []Part{TextPart("hello")},
		},
	}

	rpcResp := postJSONRPC(t, baseURL, MethodSendMessage, 1, params)

	assert.Equal(t, JSONRPCVersion, rpcResp.JSONRPC)
	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)

	var task Task
	require.NoError(t, json.Unmarshal(rpcResp.Result, &task))
	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "ctx-1", task.ContextID)
	assert.Equal(t, TaskStateSubmitted, task.Status.State)
}

func TestServerParseError(t *testing.T) {
	baseURL, _ := startTestServer(t, &mockHandler{}, testCard())

	resp, err := http.Post(baseURL+"/", "application/json", bytes.NewReader([]byte("{invalid json")))
	require.NoError(t, err)
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rpcResp))

	assert.Equal(t, JSONRPCVersion, rpcResp.JSONRPC)
	require.NotNil(t, rpcResp.Error)
	assert.Equal(t, ErrCodeParse, rpcResp.Error.Code)
	assert.Contains(t, rpcResp.Error.Message, "Parse error")
}

func TestServerMethodNotFound(t *testing.T) {
	baseURL, _ := startTestServer(t, &mockHandler{}, testCard())

	rpcResp := postJSONRPC(t, baseURL, "nonexistent/method", 1, nil)

	assert.Equal(t, JSONRPCVersion, rpcResp.JSONRPC)
	require.NotNil(t, rpcResp.Error)
	assert.Equal(t, ErrCodeMethodNotFound, rpcResp.Error.Code)
	assert.Contains(t, rpcResp.Error.Message, "Method not found")
}

func TestServerGetTask(t *testing.T) {
	var receivedID string

	handler := &mockHandler{
		getTask: func(ctx context.Context, req GetTaskRequest) (*Task, error) {
			receivedID = req.ID
			return &Task{
				ID:        req.ID,
				ContextID: "ctx-42",
				Status: TaskStatus{
					State:     TaskStateWorking,
					Timestamp: time.Now(),
				},
			}, nil
		},
	}

	baseURL, _ := startTestServer(t, handler, testCard())

	params := GetTaskRequest{ID: "task-99"}
	rpcResp := postJSONRPC(t, baseURL, MethodGetTask, 2, params)

	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)

	assert.Equal(t, "task-99", receivedID)

	var task Task
	require.NoError(t, json.Unmarshal(rpcResp.Result, &task))
	assert.Equal(t, "task-99", task.ID)
	assert.Equal(t, "ctx-42", task.ContextID)
	assert.Equal(t, TaskStateWorking, task.Status.State)
}

func TestServerListTasks(t *testing.T) {
	var receivedContextID string

	handler := &mockHandler{
		listTasks: func(ctx context.Context, req ListTasksRequest) (*ListTasksResponse, error) {
			receivedContextID = req.ContextID
			return &ListTasksResponse{
				Tasks: []Task{
					{ID: "t-1", ContextID: req.ContextID, Status: TaskStatus{State: TaskStateCompleted}},
					{ID: "t-2", ContextID: req.ContextID, Status: TaskStatus{State: TaskStateWorking}},
				},
				TotalSize: 2,
			}, nil
		},
	}

	baseURL, _ := startTestServer(t, handler, testCard())

	params := ListTasksRequest{ContextID: "ctx-list"}
	rpcResp := postJSONRPC(t, baseURL, MethodListTasks, 3, params)

	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)

	assert.Equal(t, "ctx-list", receivedContextID)

	var listResp ListTasksResponse
	require.NoError(t, json.Unmarshal(rpcResp.Result, &listResp))
	assert.Equal(t, 2, listResp.TotalSize)
	require.Len(t, listResp.Tasks, 2)
	assert.Equal(t, "t-1", listResp.Tasks[0].ID)
	assert.Equal(t, "t-2", listResp.Tasks[1].ID)
}

func TestServerCancelTask(t *testing.T) {
	var receivedID string

	handler := &mockHandler{
		cancelTask: func(ctx context.Context, req CancelTaskRequest) (*Task, error) {
			receivedID = req.ID
			return &Task{
				ID:        req.ID,
				ContextID: "ctx-cancel",
				Status: TaskStatus{
					State:     TaskStateCanceled,
					Timestamp: time.Now(),
				},
			}, nil
		},
	}

	baseURL, _ := startTestServer(t, handler, testCard())

	params := CancelTaskRequest{ID: "task-cancel-me"}
	rpcResp := postJSONRPC(t, baseURL, MethodCancelTask, 4, params)

	assert.Nil(t, rpcResp.Error)
	require.NotNil(t, rpcResp.Result)

	assert.Equal(t, "task-cancel-me", receivedID)

	var task Task
	require.NoError(t, json.Unmarshal(rpcResp.Result, &task))
	assert.Equal(t, "task-cancel-me", task.ID)
	assert.Equal(t, TaskStateCanceled, task.Status.State)
}

func TestServerGracefulShutdown(t *testing.T) {
	handler := &mockHandler{}
	card := testCard()
	srv := NewServer(card, handler)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	require.NoError(t, srv.Start(context.Background(), addr))

	// Wait until the server is accepting connections.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.Dial("tcp", addr)
		if dialErr == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify the server is alive by hitting the agent card endpoint.
	resp, err := http.Get("http://" + addr + "/.well-known/agent-card.json")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shut down.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, srv.Stop(ctx))

	// After shutdown, new connections should fail.
	// Give a small grace period for the OS to release the port.
	time.Sleep(50 * time.Millisecond)

	_, err = http.Get("http://" + addr + "/.well-known/agent-card.json")
	assert.Error(t, err, "expected connection error after shutdown")
}

func TestServerHandlerErrorReturnsInternalError(t *testing.T) {
	handler := &mockHandler{
		sendMessage: func(ctx context.Context, req SendMessageRequest) (*Task, error) {
			return nil, fmt.Errorf("something went wrong")
		},
	}

	baseURL, _ := startTestServer(t, handler, testCard())

	params := SendMessageRequest{
		Message: Message{
			MessageID: "msg-err",
			Role:      RoleUser,
			Parts:     []Part{TextPart("trigger error")},
		},
	}

	rpcResp := postJSONRPC(t, baseURL, MethodSendMessage, 5, params)

	assert.Equal(t, JSONRPCVersion, rpcResp.JSONRPC)
	require.NotNil(t, rpcResp.Error)
	assert.Equal(t, ErrCodeInternal, rpcResp.Error.Code)
	assert.Contains(t, rpcResp.Error.Message, "something went wrong")
	assert.Nil(t, rpcResp.Result)
}

func TestServerInvalidParamsError(t *testing.T) {
	baseURL, _ := startTestServer(t, &mockHandler{}, testCard())

	// Craft a raw JSON-RPC request with params that cannot unmarshal
	// into SendMessageRequest (e.g. params is a JSON string instead of object).
	reqBody := `{"jsonrpc":"2.0","id":6,"method":"message/send","params":"not-an-object"}`

	resp, err := http.Post(baseURL+"/", "application/json", bytes.NewReader([]byte(reqBody)))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var rpcResp JSONRPCResponse
	require.NoError(t, json.Unmarshal(body, &rpcResp))

	require.NotNil(t, rpcResp.Error)
	assert.Equal(t, ErrCodeInvalidParams, rpcResp.Error.Code)
	assert.Contains(t, rpcResp.Error.Message, "Invalid params")
}
