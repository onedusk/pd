package orchestrator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient implements a2a.Client for testing FanOut. Only SendMessage is
// wired to a configurable function; other methods are stubs.
type mockClient struct {
	sendMessage func(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error)
}

func (m *mockClient) SendMessage(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error) {
	return m.sendMessage(ctx, endpoint, req)
}

func (m *mockClient) GetTask(ctx context.Context, endpoint string, req a2a.GetTaskRequest) (*a2a.Task, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClient) ListTasks(ctx context.Context, endpoint string, req a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClient) CancelTask(ctx context.Context, endpoint string, req a2a.CancelTaskRequest) (*a2a.Task, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClient) SubscribeToTask(ctx context.Context, endpoint string, taskID string) (<-chan a2a.StreamEvent, error) {
	return nil, errors.New("not implemented")
}

func (m *mockClient) DiscoverAgent(ctx context.Context, baseURL string) (*a2a.AgentCard, error) {
	return nil, errors.New("not implemented")
}

// completedTask returns an a2a.Task in COMPLETED state with one text artifact.
func completedTask(id, section string) *a2a.Task {
	return &a2a.Task{
		ID: id,
		Status: a2a.TaskStatus{
			State:     a2a.TaskStateCompleted,
			Timestamp: time.Now(),
		},
		Artifacts: []a2a.Artifact{
			{
				ArtifactID: "art-" + id,
				Name:       section + "-output",
				Parts:      []a2a.Part{a2a.TextPart("result for " + section)},
			},
		},
	}
}

// makeTasks creates n AgentTasks with distinct section names.
func makeTasks(n int) []AgentTask {
	sections := []string{"platform-baseline", "api-contracts", "data-model", "security-controls", "ux-flows"}
	tasks := make([]AgentTask, n)
	for i := 0; i < n; i++ {
		name := sections[i%len(sections)]
		tasks[i] = AgentTask{
			AgentEndpoint: "http://localhost:9000/a2a",
			Message: a2a.Message{
				MessageID: "msg-" + name,
				Role:      a2a.RoleUser,
				Parts:     []a2a.Part{a2a.TextPart("produce " + name)},
			},
			Section: name,
		}
	}
	return tasks
}

func TestFanOut_AllTasksSucceed(t *testing.T) {
	client := &mockClient{
		sendMessage: func(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error) {
			// Derive a section name from the message for the task ID.
			section := req.Message.Parts[0].Text
			return completedTask("t-"+section, section), nil
		},
	}

	fanout := NewFanOut(client, nil)
	tasks := makeTasks(3)

	results, err := fanout.Run(context.Background(), StageDesignPack, tasks)
	require.NoError(t, err)
	require.Len(t, results, 3)

	for i, res := range results {
		assert.Equal(t, tasks[i].Section, res.Section)
		assert.NoError(t, res.Err)
		assert.NotNil(t, res.Task)
		require.Len(t, res.Artifacts, 1)
	}
}

func TestFanOut_SecondTaskFails_ReturnsError(t *testing.T) {
	var callCount atomic.Int32

	client := &mockClient{
		sendMessage: func(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error) {
			n := callCount.Add(1)
			// Make the second call fail. Because goroutines are concurrent
			// we key on the section name to be deterministic.
			if req.Message.MessageID == "msg-api-contracts" {
				return nil, errors.New("agent timeout")
			}
			// Other tasks may succeed or be canceled due to the errgroup
			// context cancellation — both are acceptable.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			_ = n
			section := req.Message.Parts[0].Text
			return completedTask("t-"+section, section), nil
		},
	}

	fanout := NewFanOut(client, nil)
	tasks := makeTasks(3)

	results, err := fanout.Run(context.Background(), StageDesignPack, tasks)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent timeout")

	// All result slots should be present (length equals input length).
	require.Len(t, results, 3)

	// The failing task should have its error recorded.
	failIdx := 1 // "api-contracts" is the second task.
	assert.Error(t, results[failIdx].Err)
	assert.Equal(t, "api-contracts", results[failIdx].Section)
}

func TestFanOut_TaskReturnsInputRequired(t *testing.T) {
	inputRequiredTask := &a2a.Task{
		ID: "t-input-required",
		Status: a2a.TaskStatus{
			State:     a2a.TaskStateInputRequired,
			Timestamp: time.Now(),
			Message: &a2a.Message{
				MessageID: "agent-msg-1",
				Role:      a2a.RoleAgent,
				Parts:     []a2a.Part{a2a.TextPart("Need clarification on data model")},
			},
		},
		Artifacts: []a2a.Artifact{
			{
				ArtifactID: "partial-art",
				Name:       "partial-output",
				Parts:      []a2a.Part{a2a.TextPart("partial result")},
			},
		},
	}

	client := &mockClient{
		sendMessage: func(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error) {
			return inputRequiredTask, nil
		},
	}

	fanout := NewFanOut(client, nil)
	tasks := makeTasks(1)

	results, err := fanout.Run(context.Background(), StageDesignPack, tasks)
	require.NoError(t, err, "INPUT_REQUIRED is not an error from SendMessage")
	require.Len(t, results, 1)

	res := results[0]
	assert.Equal(t, tasks[0].Section, res.Section)
	require.NotNil(t, res.Task)
	assert.Equal(t, a2a.TaskStateInputRequired, res.Task.Status.State)
	require.Len(t, res.Artifacts, 1)
	assert.Equal(t, "partial-art", res.Artifacts[0].ArtifactID)
}

func TestFanOut_ContextCancellation_TerminatesGoroutines(t *testing.T) {
	// Use a channel to signal that at least one goroutine has started.
	started := make(chan struct{}, 3)

	client := &mockClient{
		sendMessage: func(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error) {
			started <- struct{}{}
			// Block until context is canceled.
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	fanout := NewFanOut(client, nil)
	tasks := makeTasks(3)

	ctx, cancel := context.WithCancel(context.Background())

	// Run fan-out in a separate goroutine.
	type runResult struct {
		results []AgentResult
		err     error
	}
	ch := make(chan runResult, 1)
	go func() {
		results, err := fanout.Run(ctx, StageDesignPack, tasks)
		ch <- runResult{results: results, err: err}
	}()

	// Wait for at least one goroutine to start.
	<-started

	// Cancel the context.
	cancel()

	// The Run call should return promptly.
	select {
	case res := <-ch:
		require.Error(t, res.err)
		assert.ErrorIs(t, res.err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("FanOut.Run did not return after context cancellation within 5s")
	}
}

func TestFanOut_ProgressEventsEmitted(t *testing.T) {
	client := &mockClient{
		sendMessage: func(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error) {
			section := req.Message.Parts[0].Text
			return completedTask("t-"+section, section), nil
		},
	}

	var mu sync.Mutex
	var events []ProgressEvent

	onProgress := func(ev ProgressEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, ev)
	}

	fanout := NewFanOut(client, onProgress)
	tasks := makeTasks(3)

	results, err := fanout.Run(context.Background(), StageDesignPack, tasks)
	require.NoError(t, err)
	require.Len(t, results, 3)

	mu.Lock()
	defer mu.Unlock()

	// Each task should emit at least: Pending, Working, Complete = 3 events.
	// With 3 tasks that is at minimum 9 events.
	assert.GreaterOrEqual(t, len(events), 9,
		"expected at least 9 progress events (3 per task), got %d", len(events))

	// Verify we see every expected status at least once per section.
	sectionStatuses := make(map[string]map[ProgressStatus]bool)
	for _, ev := range events {
		assert.Equal(t, StageDesignPack, ev.Stage)
		if sectionStatuses[ev.Section] == nil {
			sectionStatuses[ev.Section] = make(map[ProgressStatus]bool)
		}
		sectionStatuses[ev.Section][ev.Status] = true
	}

	for _, task := range tasks {
		statuses, ok := sectionStatuses[task.Section]
		require.True(t, ok, "no progress events for section %q", task.Section)
		assert.True(t, statuses[ProgressPending], "missing Pending event for section %q", task.Section)
		assert.True(t, statuses[ProgressWorking], "missing Working event for section %q", task.Section)
		assert.True(t, statuses[ProgressComplete], "missing Complete event for section %q", task.Section)
	}
}
