package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rpcHandler is a convenience that decodes a JSONRPCRequest and writes back a JSONRPCResponse.
func rpcHandler(t *testing.T, fn func(req JSONRPCRequest) JSONRPCResponse) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "A2A always uses POST")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req JSONRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err, "server should be able to decode JSON-RPC request")

		assert.Equal(t, JSONRPCVersion, req.JSONRPC)

		resp := fn(req)
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}
}

func TestSendMessage_HappyPath(t *testing.T) {
	ts := httptest.NewServer(rpcHandler(t, func(req JSONRPCRequest) JSONRPCResponse {
		assert.Equal(t, MethodSendMessage, req.Method)

		// Verify the params contain the message we sent.
		var params SendMessageRequest
		require.NoError(t, json.Unmarshal(req.Params, &params))
		assert.Equal(t, RoleUser, params.Message.Role)
		assert.Equal(t, "hello", params.Message.Parts[0].Text)

		task := Task{
			ID:        "task-001",
			ContextID: "ctx-001",
			Status: TaskStatus{
				State:     TaskStateCompleted,
				Timestamp: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
			},
			Artifacts: []Artifact{
				{
					ArtifactID: "art-1",
					Name:       "response",
					Parts:      []Part{TextPart("world")},
				},
			},
		}
		result, err := json.Marshal(task)
		require.NoError(t, err)

		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}
	}))
	defer ts.Close()

	client := NewHTTPClient()
	task, err := client.SendMessage(context.Background(), ts.URL, SendMessageRequest{
		Message: Message{
			MessageID: "msg-1",
			Role:      RoleUser,
			Parts:     []Part{TextPart("hello")},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "task-001", task.ID)
	assert.Equal(t, "ctx-001", task.ContextID)
	assert.Equal(t, TaskStateCompleted, task.Status.State)
	require.Len(t, task.Artifacts, 1)
	assert.Equal(t, "art-1", task.Artifacts[0].ArtifactID)
	assert.Equal(t, "response", task.Artifacts[0].Name)
	assert.Equal(t, "world", task.Artifacts[0].Parts[0].Text)
}

func TestSendMessage_RPCError(t *testing.T) {
	ts := httptest.NewServer(rpcHandler(t, func(req JSONRPCRequest) JSONRPCResponse {
		assert.Equal(t, MethodSendMessage, req.Method)

		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    ErrCodeInvalidParams,
				Message: "missing required field: message",
				Data:    json.RawMessage(`{"field":"message"}`),
			},
		}
	}))
	defer ts.Close()

	client := NewHTTPClient()
	task, err := client.SendMessage(context.Background(), ts.URL, SendMessageRequest{})

	require.Error(t, err)
	assert.Nil(t, task)

	var rpcErr *RPCError
	require.ErrorAs(t, err, &rpcErr)
	assert.Equal(t, MethodSendMessage, rpcErr.Method)
	assert.Equal(t, ErrCodeInvalidParams, rpcErr.Code)
	assert.Equal(t, "missing required field: message", rpcErr.Message)
	assert.JSONEq(t, `{"field":"message"}`, string(rpcErr.Data))
}

func TestGetTask(t *testing.T) {
	ts := httptest.NewServer(rpcHandler(t, func(req JSONRPCRequest) JSONRPCResponse {
		assert.Equal(t, MethodGetTask, req.Method)

		var params GetTaskRequest
		require.NoError(t, json.Unmarshal(req.Params, &params))
		assert.Equal(t, "task-42", params.ID)

		task := Task{
			ID:        "task-42",
			ContextID: "ctx-7",
			Status: TaskStatus{
				State:     TaskStateCompleted,
				Timestamp: time.Date(2025, 3, 1, 9, 30, 0, 0, time.UTC),
			},
			Artifacts: []Artifact{
				{
					ArtifactID:  "art-a",
					Name:        "analysis",
					Description: "Code analysis output",
					Parts:       []Part{TextPart("analysis result")},
				},
				{
					ArtifactID: "art-b",
					Name:       "diff",
					Parts:      []Part{TextPart("--- a/file.go\n+++ b/file.go")},
				},
			},
			History: []Message{
				{
					MessageID: "msg-1",
					Role:      RoleUser,
					Parts:     []Part{TextPart("analyze this code")},
				},
			},
		}
		result, err := json.Marshal(task)
		require.NoError(t, err)

		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}
	}))
	defer ts.Close()

	client := NewHTTPClient()
	task, err := client.GetTask(context.Background(), ts.URL, GetTaskRequest{ID: "task-42"})

	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "task-42", task.ID)
	assert.Equal(t, "ctx-7", task.ContextID)
	assert.Equal(t, TaskStateCompleted, task.Status.State)

	require.Len(t, task.Artifacts, 2)
	assert.Equal(t, "art-a", task.Artifacts[0].ArtifactID)
	assert.Equal(t, "Code analysis output", task.Artifacts[0].Description)
	assert.Equal(t, "art-b", task.Artifacts[1].ArtifactID)
	assert.Equal(t, "diff", task.Artifacts[1].Name)

	require.Len(t, task.History, 1)
	assert.Equal(t, RoleUser, task.History[0].Role)
}

func TestListTasks(t *testing.T) {
	ts := httptest.NewServer(rpcHandler(t, func(req JSONRPCRequest) JSONRPCResponse {
		assert.Equal(t, MethodListTasks, req.Method)

		var params ListTasksRequest
		require.NoError(t, json.Unmarshal(req.Params, &params))
		assert.Equal(t, "ctx-100", params.ContextID)
		assert.Equal(t, 2, params.PageSize)

		resp := ListTasksResponse{
			Tasks: []Task{
				{
					ID:        "task-10",
					ContextID: "ctx-100",
					Status:    TaskStatus{State: TaskStateCompleted},
				},
				{
					ID:        "task-11",
					ContextID: "ctx-100",
					Status:    TaskStatus{State: TaskStateWorking},
				},
			},
			TotalSize:     5,
			NextPageToken: "page-2-token",
		}
		result, err := json.Marshal(resp)
		require.NoError(t, err)

		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}
	}))
	defer ts.Close()

	client := NewHTTPClient()
	resp, err := client.ListTasks(context.Background(), ts.URL, ListTasksRequest{
		ContextID: "ctx-100",
		PageSize:  2,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Tasks, 2)
	assert.Equal(t, "task-10", resp.Tasks[0].ID)
	assert.Equal(t, TaskStateCompleted, resp.Tasks[0].Status.State)
	assert.Equal(t, "task-11", resp.Tasks[1].ID)
	assert.Equal(t, TaskStateWorking, resp.Tasks[1].Status.State)
	assert.Equal(t, 5, resp.TotalSize)
	assert.Equal(t, "page-2-token", resp.NextPageToken)
}

func TestCancelTask(t *testing.T) {
	var receivedMethod string
	var receivedHTTPMethod string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHTTPMethod = r.Method

		var req JSONRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		receivedMethod = req.Method

		var params CancelTaskRequest
		require.NoError(t, json.Unmarshal(req.Params, &params))
		assert.Equal(t, "task-99", params.ID)

		task := Task{
			ID:        "task-99",
			ContextID: "ctx-50",
			Status: TaskStatus{
				State:     TaskStateCanceled,
				Timestamp: time.Now().UTC(),
			},
		}
		result, err := json.Marshal(task)
		require.NoError(t, err)

		resp := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  result,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer ts.Close()

	client := NewHTTPClient()
	task, err := client.CancelTask(context.Background(), ts.URL, CancelTaskRequest{ID: "task-99"})

	require.NoError(t, err)
	require.NotNil(t, task)

	assert.Equal(t, http.MethodPost, receivedHTTPMethod, "A2A uses POST for all JSON-RPC calls")
	assert.Equal(t, MethodCancelTask, receivedMethod)
	assert.Equal(t, "task-99", task.ID)
	assert.Equal(t, TaskStateCanceled, task.Status.State)
}

func TestDiscoverAgent(t *testing.T) {
	card := AgentCard{
		Name:        "Code Analyzer",
		Description: "Analyzes code for patterns and issues",
		Version:     "1.2.0",
		Interfaces: []AgentInterface{
			{
				URL:             "https://agent.example.com/a2a",
				ProtocolBinding: "jsonrpc+http",
				ProtocolVersion: "0.2.1",
			},
		},
		Provider: &AgentProvider{
			Organization: "Dusk Industries",
			URL:          "https://dusk.industries",
		},
		Capabilities: AgentCapabilities{
			Streaming:         true,
			PushNotifications: false,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain", "application/json"},
		Skills: []AgentSkill{
			{
				ID:          "analyze",
				Name:        "Code Analysis",
				Description: "Analyze code for patterns",
				Tags:        []string{"code", "analysis"},
				Examples:    []string{"Analyze this Go file"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "DiscoverAgent uses GET")
		assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(card)
		require.NoError(t, err)
	}))
	defer ts.Close()

	client := NewHTTPClient()
	result, err := client.DiscoverAgent(context.Background(), ts.URL)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Code Analyzer", result.Name)
	assert.Equal(t, "Analyzes code for patterns and issues", result.Description)
	assert.Equal(t, "1.2.0", result.Version)

	require.Len(t, result.Interfaces, 1)
	assert.Equal(t, "jsonrpc+http", result.Interfaces[0].ProtocolBinding)
	assert.Equal(t, "0.2.1", result.Interfaces[0].ProtocolVersion)

	require.NotNil(t, result.Provider)
	assert.Equal(t, "Dusk Industries", result.Provider.Organization)
	assert.Equal(t, "https://dusk.industries", result.Provider.URL)

	assert.True(t, result.Capabilities.Streaming)
	assert.False(t, result.Capabilities.PushNotifications)

	assert.Equal(t, []string{"text/plain"}, result.DefaultInputModes)
	assert.Equal(t, []string{"text/plain", "application/json"}, result.DefaultOutputModes)

	require.Len(t, result.Skills, 1)
	assert.Equal(t, "analyze", result.Skills[0].ID)
	assert.Equal(t, "Code Analysis", result.Skills[0].Name)
	assert.Equal(t, []string{"code", "analysis"}, result.Skills[0].Tags)
	assert.Equal(t, []string{"Analyze this Go file"}, result.Skills[0].Examples)
}

func TestDiscoverAgent_TrailingSlash(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path,
			"trailing slash on baseURL should not produce double slash")

		card := AgentCard{Name: "Test"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(card)
	}))
	defer ts.Close()

	client := NewHTTPClient()
	result, err := client.DiscoverAgent(context.Background(), ts.URL+"/")

	require.NoError(t, err)
	assert.Equal(t, "Test", result.Name)
}

func TestContextTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay longer than the context deadline to force a timeout.
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := NewHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	task, err := client.SendMessage(ctx, ts.URL, SendMessageRequest{
		Message: Message{
			MessageID: "msg-timeout",
			Role:      RoleUser,
			Parts:     []Part{TextPart("this will timeout")},
		},
	})

	require.Error(t, err)
	assert.Nil(t, task)
	assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
}

func TestNon200HTTPStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer ts.Close()

	client := NewHTTPClient()
	task, err := client.SendMessage(context.Background(), ts.URL, SendMessageRequest{
		Message: Message{
			MessageID: "msg-500",
			Role:      RoleUser,
			Parts:     []Part{TextPart("trigger error")},
		},
	})

	require.Error(t, err)
	assert.Nil(t, task)
	assert.Contains(t, err.Error(), "HTTP 500")
	assert.Contains(t, err.Error(), "internal server error")

	// Ensure it is NOT an RPCError -- it is an HTTP-level error.
	var rpcErr *RPCError
	assert.False(t, errors.As(err, &rpcErr), "HTTP-level errors should not be RPCError")
}

func TestNon200HTTPStatus_NotRPCError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("bad gateway"))
	}))
	defer ts.Close()

	client := NewHTTPClient()
	task, err := client.SendMessage(context.Background(), ts.URL, SendMessageRequest{
		Message: Message{
			MessageID: "msg-502",
			Role:      RoleUser,
			Parts:     []Part{TextPart("bad gateway")},
		},
	})

	require.Error(t, err)
	assert.Nil(t, task)
	assert.Contains(t, err.Error(), "HTTP 502")

	var rpcErr *RPCError
	assert.False(t, errors.As(err, &rpcErr), "HTTP-level errors should not be RPCError")
}

func TestGetTask_RPCErrorTaskNotFound(t *testing.T) {
	ts := httptest.NewServer(rpcHandler(t, func(req JSONRPCRequest) JSONRPCResponse {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    ErrCodeTaskNotFound,
				Message: "task not found: task-missing",
			},
		}
	}))
	defer ts.Close()

	client := NewHTTPClient()
	task, err := client.GetTask(context.Background(), ts.URL, GetTaskRequest{ID: "task-missing"})

	require.Error(t, err)
	assert.Nil(t, task)

	var rpcErr *RPCError
	require.ErrorAs(t, err, &rpcErr)
	assert.Equal(t, ErrCodeTaskNotFound, rpcErr.Code)
	assert.Contains(t, rpcErr.Message, "task-missing")
}

func TestWithTimeout_Option(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Set client-level timeout that is shorter than the mock delay.
	client := NewHTTPClient(WithTimeout(50 * time.Millisecond))

	task, err := client.SendMessage(context.Background(), ts.URL, SendMessageRequest{
		Message: Message{
			MessageID: "msg-client-timeout",
			Role:      RoleUser,
			Parts:     []Part{TextPart("timeout via option")},
		},
	})

	require.Error(t, err)
	assert.Nil(t, task)
}

func TestSubscribeToTask_NotImplemented(t *testing.T) {
	client := NewHTTPClient()
	ch, err := client.SubscribeToTask(context.Background(), "http://example.com", "task-1")

	require.Error(t, err)
	assert.Nil(t, ch)
	assert.ErrorIs(t, err, ErrNotImplemented)
}

func TestSendMessage_VerifiesJSONRPCVersion(t *testing.T) {
	var receivedVersion string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedVersion = req.JSONRPC

		task := Task{ID: "task-ver", Status: TaskStatus{State: TaskStateCompleted}}
		result, _ := json.Marshal(task)
		resp := JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Result: result}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewHTTPClient()
	_, err := client.SendMessage(context.Background(), ts.URL, SendMessageRequest{
		Message: Message{
			MessageID: "msg-ver",
			Role:      RoleUser,
			Parts:     []Part{TextPart("version check")},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "2.0", receivedVersion, "client should send JSON-RPC version 2.0")
}

func TestDiscoverAgent_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer ts.Close()

	client := NewHTTPClient()
	card, err := client.DiscoverAgent(context.Background(), ts.URL)

	require.Error(t, err)
	assert.Nil(t, card)
	assert.Contains(t, err.Error(), "HTTP 404")
}
