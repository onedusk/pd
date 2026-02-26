package a2a

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    TaskState
		terminal bool
	}{
		{TaskStateUnspecified, false},
		{TaskStateSubmitted, false},
		{TaskStateWorking, false},
		{TaskStateCompleted, true},
		{TaskStateFailed, true},
		{TaskStateCanceled, true},
		{TaskStateInputRequired, false},
		{TaskStateRejected, true},
		{TaskStateAuthRequired, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			assert.Equal(t, tt.terminal, tt.state.IsTerminal())
		})
	}
}

func TestTask_RoundTrip(t *testing.T) {
	states := []TaskState{
		TaskStateSubmitted,
		TaskStateWorking,
		TaskStateCompleted,
		TaskStateFailed,
		TaskStateCanceled,
		TaskStateInputRequired,
		TaskStateRejected,
		TaskStateAuthRequired,
	}

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			original := Task{
				ID:        "task-123",
				ContextID: "ctx-456",
				Status: TaskStatus{
					State:     state,
					Timestamp: time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC),
				},
			}

			data, err := json.Marshal(original)
			require.NoError(t, err)

			var decoded Task
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, original.ID, decoded.ID)
			assert.Equal(t, original.ContextID, decoded.ContextID)
			assert.Equal(t, original.Status.State, decoded.Status.State)
		})
	}
}

func TestTask_WithArtifactsAndHistory(t *testing.T) {
	original := Task{
		ID:        "task-full",
		ContextID: "ctx-full",
		Status: TaskStatus{
			State:     TaskStateCompleted,
			Timestamp: time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC),
		},
		Artifacts: []Artifact{
			{
				ArtifactID:  "art-1",
				Name:        "design-pack",
				Description: "Stage 1 output",
				Parts:       []Part{TextPart("## Design Pack")},
			},
		},
		History: []Message{
			{
				MessageID: "msg-1",
				Role:      RoleUser,
				Parts:     []Part{TextPart("decompose this project")},
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Task
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Len(t, decoded.Artifacts, 1)
	assert.Equal(t, "art-1", decoded.Artifacts[0].ArtifactID)
	assert.Equal(t, "design-pack", decoded.Artifacts[0].Name)
	require.Len(t, decoded.Artifacts[0].Parts, 1)
	assert.Equal(t, "## Design Pack", decoded.Artifacts[0].Parts[0].Text)

	require.Len(t, decoded.History, 1)
	assert.Equal(t, RoleUser, decoded.History[0].Role)
}

func TestMessage_MultiPart(t *testing.T) {
	dataPart, err := DataPart(map[string]string{"key": "value"})
	require.NoError(t, err)

	original := Message{
		MessageID: "msg-multi",
		ContextID: "ctx-1",
		Role:      RoleAgent,
		Parts: []Part{
			TextPart("Here is the analysis"),
			dataPart,
		},
		ReferenceTaskIDs: []string{"task-ref-1", "task-ref-2"},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.MessageID, decoded.MessageID)
	assert.Equal(t, original.Role, decoded.Role)
	require.Len(t, decoded.Parts, 2)
	assert.Equal(t, "Here is the analysis", decoded.Parts[0].Text)
	assert.Equal(t, "text/plain", decoded.Parts[0].MediaType)
	assert.Equal(t, "application/json", decoded.Parts[1].MediaType)
	assert.NotNil(t, decoded.Parts[1].Data)
	assert.Equal(t, []string{"task-ref-1", "task-ref-2"}, decoded.ReferenceTaskIDs)
}

func TestAgentCard_RoundTrip(t *testing.T) {
	original := AgentCard{
		Name:        "research-agent",
		Description: "Platform investigation specialist",
		Version:     "0.1.0",
		Interfaces: []AgentInterface{
			{
				URL:             "http://localhost:9101",
				ProtocolBinding: "JSONRPC",
				ProtocolVersion: "0.4",
			},
		},
		Provider: &AgentProvider{
			Organization: "dusk-indust",
			URL:          "https://dusk.industries",
		},
		Capabilities: AgentCapabilities{
			Streaming:         true,
			PushNotifications: false,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/markdown"},
		Skills: []AgentSkill{
			{
				ID:          "research-platform",
				Name:        "Research Platform",
				Description: "Investigate a platform or SDK",
				Tags:        []string{"research", "platform"},
				Examples:    []string{"Research Go 1.26 features"},
			},
			{
				ID:          "verify-versions",
				Name:        "Verify Versions",
				Description: "Cross-check version numbers",
				Tags:        []string{"research", "verification"},
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded AgentCard
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Version, decoded.Version)
	require.Len(t, decoded.Interfaces, 1)
	assert.Equal(t, "JSONRPC", decoded.Interfaces[0].ProtocolBinding)
	require.NotNil(t, decoded.Provider)
	assert.Equal(t, "dusk-indust", decoded.Provider.Organization)
	assert.True(t, decoded.Capabilities.Streaming)
	assert.False(t, decoded.Capabilities.PushNotifications)
	require.Len(t, decoded.Skills, 2)
	assert.Equal(t, "research-platform", decoded.Skills[0].ID)
	require.Len(t, decoded.Skills[0].Examples, 1)
}

func TestSendMessageRequest_RoundTrip(t *testing.T) {
	t.Run("with configuration", func(t *testing.T) {
		histLen := 5
		original := SendMessageRequest{
			Message: Message{
				MessageID: "msg-1",
				Role:      RoleUser,
				Parts:     []Part{TextPart("hello")},
			},
			Configuration: &SendMessageConfig{
				AcceptedOutputModes: []string{"text/markdown"},
				HistoryLength:       &histLen,
				Blocking:            true,
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded SendMessageRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Equal(t, original.Message.MessageID, decoded.Message.MessageID)
		require.NotNil(t, decoded.Configuration)
		assert.True(t, decoded.Configuration.Blocking)
		require.NotNil(t, decoded.Configuration.HistoryLength)
		assert.Equal(t, 5, *decoded.Configuration.HistoryLength)
	})

	t.Run("without configuration", func(t *testing.T) {
		original := SendMessageRequest{
			Message: Message{
				MessageID: "msg-2",
				Role:      RoleUser,
				Parts:     []Part{TextPart("hello")},
			},
		}

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var decoded SendMessageRequest
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)

		assert.Nil(t, decoded.Configuration)
	})
}

func TestPart_ContentTypes(t *testing.T) {
	t.Run("text part", func(t *testing.T) {
		p := TextPart("hello world")
		assert.Equal(t, "hello world", p.Text)
		assert.Equal(t, "text/plain", p.MediaType)

		data, err := json.Marshal(p)
		require.NoError(t, err)
		var decoded Part
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, p.Text, decoded.Text)
	})

	t.Run("data part", func(t *testing.T) {
		p, err := DataPart(map[string]int{"count": 42})
		require.NoError(t, err)
		assert.Equal(t, "application/json", p.MediaType)
		assert.NotNil(t, p.Data)

		data, err := json.Marshal(p)
		require.NoError(t, err)
		var decoded Part
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.JSONEq(t, `{"count":42}`, string(decoded.Data))
	})

	t.Run("raw part", func(t *testing.T) {
		p := Part{
			Raw:       []byte{0x89, 0x50, 0x4E, 0x47},
			MediaType: "image/png",
		}

		data, err := json.Marshal(p)
		require.NoError(t, err)
		var decoded Part
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, p.Raw, decoded.Raw)
		assert.Equal(t, "image/png", decoded.MediaType)
	})

	t.Run("url part", func(t *testing.T) {
		p := Part{
			URL:       "https://example.com/file.pdf",
			MediaType: "application/pdf",
			Filename:  "report.pdf",
		}

		data, err := json.Marshal(p)
		require.NoError(t, err)
		var decoded Part
		require.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, "https://example.com/file.pdf", decoded.URL)
		assert.Equal(t, "report.pdf", decoded.Filename)
	})
}

func TestJSONRPC_RequestRoundTrip(t *testing.T) {
	params, _ := json.Marshal(SendMessageRequest{
		Message: Message{
			MessageID: "msg-1",
			Role:      RoleUser,
			Parts:     []Part{TextPart("hello")},
		},
	})

	original := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      float64(1), // JSON numbers decode as float64
		Method:  MethodSendMessage,
		Params:  params,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded JSONRPCRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, JSONRPCVersion, decoded.JSONRPC)
	assert.Equal(t, MethodSendMessage, decoded.Method)
	assert.NotNil(t, decoded.Params)
}

func TestJSONRPC_ResponseSuccess(t *testing.T) {
	result, _ := json.Marshal(Task{
		ID: "task-1",
		Status: TaskStatus{
			State:     TaskStateCompleted,
			Timestamp: time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC),
		},
	})

	original := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      float64(1),
		Result:  result,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded JSONRPCResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, JSONRPCVersion, decoded.JSONRPC)
	assert.NotNil(t, decoded.Result)
	assert.Nil(t, decoded.Error)
}

func TestJSONRPC_ResponseError(t *testing.T) {
	original := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      float64(1),
		Error: &JSONRPCError{
			Code:    ErrCodeTaskNotFound,
			Message: "task not found",
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded JSONRPCResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Nil(t, decoded.Result)
	require.NotNil(t, decoded.Error)
	assert.Equal(t, ErrCodeTaskNotFound, decoded.Error.Code)
	assert.Equal(t, "task not found", decoded.Error.Message)
}

func TestDataPart_InvalidInput(t *testing.T) {
	// Channels can't be marshaled to JSON.
	_, err := DataPart(make(chan int))
	assert.Error(t, err)
}
