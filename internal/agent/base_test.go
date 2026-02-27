package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCard returns an AgentCard suitable for testing.
func testCard() a2a.AgentCard {
	return a2a.AgentCard{
		Name:        "test-agent",
		Description: "A test agent",
		Version:     "0.1.0",
		Skills: []a2a.AgentSkill{
			{
				ID:          "echo",
				Name:        "Echo",
				Description: "Echoes the input back",
				Tags:        []string{"test"},
			},
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
	}
}

// successProcess returns a ProcessFunc that produces a single text artifact.
func successProcess() ProcessFunc {
	return func(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
		return []a2a.Artifact{
			{
				ArtifactID: "art-1",
				Name:       "output",
				Parts:      []a2a.Part{a2a.TextPart("hello")},
			},
		}, nil
	}
}

// failProcess returns a ProcessFunc that always returns an error.
func failProcess() ProcessFunc {
	return func(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
		return nil, errors.New("processing failed")
	}
}

// testMessage returns a Message suitable for testing.
func testMessage() a2a.Message {
	return a2a.Message{
		MessageID: "msg-1",
		ContextID: "ctx-1",
		Role:      a2a.RoleUser,
		Parts:     []a2a.Part{a2a.TextPart("test input")},
	}
}

func TestBaseAgent_InterfaceCompliance(t *testing.T) {
	// Verify that NewBaseAgent returns something that satisfies Agent.
	var a Agent = NewBaseAgent(testCard(), successProcess())
	require.NotNil(t, a)
}

func TestBaseAgent_Card(t *testing.T) {
	card := testCard()
	agent := NewBaseAgent(card, successProcess())

	got := agent.Card()

	assert.Equal(t, card.Name, got.Name)
	assert.Equal(t, card.Description, got.Description)
	assert.Equal(t, card.Version, got.Version)
	require.Len(t, got.Skills, 1)
	assert.Equal(t, "echo", got.Skills[0].ID)
	assert.Equal(t, card.DefaultInputModes, got.DefaultInputModes)
	assert.Equal(t, card.DefaultOutputModes, got.DefaultOutputModes)
}

func TestBaseAgent_HandleTask_HappyPath(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	task := a2a.Task{
		ID:        a2a.NewTaskID(),
		ContextID: "ctx-1",
	}
	msg := testMessage()

	result, err := agent.HandleTask(ctx, task, msg)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, task.ID, result.ID)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	assert.False(t, result.Status.Timestamp.IsZero())
	require.Len(t, result.Artifacts, 1)
	assert.Equal(t, "art-1", result.Artifacts[0].ArtifactID)
	assert.Equal(t, "output", result.Artifacts[0].Name)
	require.Len(t, result.Artifacts[0].Parts, 1)
	assert.Equal(t, "hello", result.Artifacts[0].Parts[0].Text)
}

func TestBaseAgent_HandleTask_Failure(t *testing.T) {
	agent := NewBaseAgent(testCard(), failProcess())
	ctx := context.Background()

	task := a2a.Task{
		ID:        a2a.NewTaskID(),
		ContextID: "ctx-1",
	}
	msg := testMessage()

	result, err := agent.HandleTask(ctx, task, msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "processing failed")
	require.NotNil(t, result)
	assert.Equal(t, task.ID, result.ID)
	assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
	assert.False(t, result.Status.Timestamp.IsZero())
	// The failed task should have a status message with the error.
	require.NotNil(t, result.Status.Message)
	assert.Equal(t, a2a.RoleAgent, result.Status.Message.Role)
	require.Len(t, result.Status.Message.Parts, 1)
	assert.Contains(t, result.Status.Message.Parts[0].Text, "processing failed")
}

func TestBaseAgent_HandleTask_MultipleArtifacts(t *testing.T) {
	process := func(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
		return []a2a.Artifact{
			{ArtifactID: "a1", Name: "first", Parts: []a2a.Part{a2a.TextPart("one")}},
			{ArtifactID: "a2", Name: "second", Parts: []a2a.Part{a2a.TextPart("two")}},
		}, nil
	}
	agent := NewBaseAgent(testCard(), process)
	ctx := context.Background()

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "ctx-1"}
	result, err := agent.HandleTask(ctx, task, testMessage())
	require.NoError(t, err)
	require.Len(t, result.Artifacts, 2)
	assert.Equal(t, "a1", result.Artifacts[0].ArtifactID)
	assert.Equal(t, "a2", result.Artifacts[1].ArtifactID)
}

func TestBaseAgent_HandleSendMessage(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	req := a2a.SendMessageRequest{
		Message: testMessage(),
	}

	result, err := agent.HandleSendMessage(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Task ID should have been generated.
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "ctx-1", result.ContextID)
	assert.Equal(t, a2a.TaskStateCompleted, result.Status.State)
	require.Len(t, result.Artifacts, 1)
	assert.Equal(t, "hello", result.Artifacts[0].Parts[0].Text)
}

func TestBaseAgent_HandleGetTask(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	// Create a task via HandleSendMessage.
	sendReq := a2a.SendMessageRequest{Message: testMessage()}
	created, err := agent.HandleSendMessage(ctx, sendReq)
	require.NoError(t, err)

	// Now retrieve it via HandleGetTask.
	getReq := a2a.GetTaskRequest{ID: created.ID}
	retrieved, err := agent.HandleGetTask(ctx, getReq)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.ContextID, retrieved.ContextID)
	assert.Equal(t, a2a.TaskStateCompleted, retrieved.Status.State)
	require.Len(t, retrieved.Artifacts, 1)
	assert.Equal(t, "art-1", retrieved.Artifacts[0].ArtifactID)
}

func TestBaseAgent_HandleGetTask_NotFound(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	_, err := agent.HandleGetTask(ctx, a2a.GetTaskRequest{ID: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBaseAgent_HandleListTasks(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	// Create tasks with different context IDs.
	for i := 0; i < 3; i++ {
		msg := a2a.Message{
			MessageID: fmt.Sprintf("msg-%d", i),
			ContextID: "ctx-shared",
			Role:      a2a.RoleUser,
			Parts:     []a2a.Part{a2a.TextPart(fmt.Sprintf("input %d", i))},
		}
		_, err := agent.HandleSendMessage(ctx, a2a.SendMessageRequest{Message: msg})
		require.NoError(t, err)
	}

	// Create one task with a different context.
	diffMsg := a2a.Message{
		MessageID: "msg-diff",
		ContextID: "ctx-other",
		Role:      a2a.RoleUser,
		Parts:     []a2a.Part{a2a.TextPart("different context")},
	}
	_, err := agent.HandleSendMessage(ctx, a2a.SendMessageRequest{Message: diffMsg})
	require.NoError(t, err)

	t.Run("list all tasks", func(t *testing.T) {
		resp, err := agent.HandleListTasks(ctx, a2a.ListTasksRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 4, resp.TotalSize)
		assert.Len(t, resp.Tasks, 4)
	})

	t.Run("filter by context ID", func(t *testing.T) {
		resp, err := agent.HandleListTasks(ctx, a2a.ListTasksRequest{
			ContextID: "ctx-shared",
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 3, resp.TotalSize)
		assert.Len(t, resp.Tasks, 3)
		for _, task := range resp.Tasks {
			assert.Equal(t, "ctx-shared", task.ContextID)
		}
	})

	t.Run("filter by context ID no matches", func(t *testing.T) {
		resp, err := agent.HandleListTasks(ctx, a2a.ListTasksRequest{
			ContextID: "ctx-nonexistent",
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 0, resp.TotalSize)
		assert.Empty(t, resp.Tasks)
	})
}

func TestBaseAgent_HandleCancelTask_CompletedTaskUnchanged(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	// Create a completed task via HandleSendMessage.
	created, err := agent.HandleSendMessage(ctx, a2a.SendMessageRequest{Message: testMessage()})
	require.NoError(t, err)
	assert.Equal(t, a2a.TaskStateCompleted, created.Status.State)

	// Attempt to cancel the completed task.
	canceled, err := agent.HandleCancelTask(ctx, a2a.CancelTaskRequest{ID: created.ID})
	require.NoError(t, err)
	require.NotNil(t, canceled)

	// The task should remain completed since it is in a terminal state.
	assert.Equal(t, a2a.TaskStateCompleted, canceled.Status.State)
}

func TestBaseAgent_HandleCancelTask_FailedTaskUnchanged(t *testing.T) {
	agent := NewBaseAgent(testCard(), failProcess())
	ctx := context.Background()

	// Create a failed task.
	created, err := agent.HandleSendMessage(ctx, a2a.SendMessageRequest{Message: testMessage()})
	require.Error(t, err) // HandleSendMessage propagates the process error.
	// Even with the error, HandleTask returns the task in failed state.
	// However, HandleSendMessage returns via HandleTask which returns both task and error.
	// Looking at the code: HandleSendMessage calls HandleTask which returns (result, err).
	// On failure, HandleTask returns (task, err) where task is in FAILED state.
	// So created should be non-nil.
	require.NotNil(t, created)
	assert.Equal(t, a2a.TaskStateFailed, created.Status.State)

	// Attempt to cancel the failed task.
	canceled, err := agent.HandleCancelTask(ctx, a2a.CancelTaskRequest{ID: created.ID})
	require.NoError(t, err)
	require.NotNil(t, canceled)

	// The task should remain failed since it is in a terminal state.
	assert.Equal(t, a2a.TaskStateFailed, canceled.Status.State)
}

func TestBaseAgent_HandleCancelTask_NotFound(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	_, err := agent.HandleCancelTask(ctx, a2a.CancelTaskRequest{ID: "nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBaseAgent_StartStop(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	// Find a random available port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	// Start the server.
	err = agent.Start(ctx, addr)
	require.NoError(t, err)

	// Give the server a moment to start listening.
	time.Sleep(50 * time.Millisecond)

	// Verify the server is running by hitting the agent card endpoint.
	resp, err := http.Get(fmt.Sprintf("http://%s/.well-known/agent-card.json", addr))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stop the server.
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = agent.Stop(stopCtx)
	require.NoError(t, err)

	// Verify the server is no longer responding.
	_, err = http.Get(fmt.Sprintf("http://%s/.well-known/agent-card.json", addr))
	require.Error(t, err)
}

func TestBaseAgent_HandleTask_DuplicateID(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	taskID := a2a.NewTaskID()
	task := a2a.Task{ID: taskID, ContextID: "ctx-1"}
	msg := testMessage()

	// First call should succeed.
	_, err := agent.HandleTask(ctx, task, msg)
	require.NoError(t, err)

	// Second call with the same ID should fail because the task already exists.
	_, err = agent.HandleTask(ctx, task, msg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestBaseAgent_HandleTask_ContextCanceled(t *testing.T) {
	process := func(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
		// Check if context is already canceled.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return []a2a.Artifact{
				{ArtifactID: "a1", Name: "output", Parts: []a2a.Part{a2a.TextPart("done")}},
			}, nil
		}
	}

	agent := NewBaseAgent(testCard(), process)

	// Cancel the context before calling HandleTask.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := a2a.Task{ID: a2a.NewTaskID(), ContextID: "ctx-1"}
	result, err := agent.HandleTask(ctx, task, testMessage())
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, result)
	assert.Equal(t, a2a.TaskStateFailed, result.Status.State)
}

func TestBaseAgent_HandleSendMessage_GeneratesUniqueIDs(t *testing.T) {
	agent := NewBaseAgent(testCard(), successProcess())
	ctx := context.Background()

	ids := make(map[string]struct{})
	for i := 0; i < 10; i++ {
		msg := a2a.Message{
			MessageID: fmt.Sprintf("msg-%d", i),
			ContextID: "ctx-1",
			Role:      a2a.RoleUser,
			Parts:     []a2a.Part{a2a.TextPart("input")},
		}
		result, err := agent.HandleSendMessage(ctx, a2a.SendMessageRequest{Message: msg})
		require.NoError(t, err)
		_, exists := ids[result.ID]
		assert.False(t, exists, "duplicate task ID: %s", result.ID)
		ids[result.ID] = struct{}{}
	}
	assert.Len(t, ids, 10)
}
