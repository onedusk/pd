package a2a

import "context"

// Client is the interface for an A2A client that sends tasks to agents.
type Client interface {
	// SendMessage sends a message to an agent and returns the task.
	// For blocking mode, waits until the task reaches a terminal or interrupted state.
	SendMessage(ctx context.Context, endpoint string, req SendMessageRequest) (*Task, error)

	// GetTask retrieves a task by ID from a specific agent.
	GetTask(ctx context.Context, endpoint string, req GetTaskRequest) (*Task, error)

	// ListTasks queries tasks from a specific agent.
	ListTasks(ctx context.Context, endpoint string, req ListTasksRequest) (*ListTasksResponse, error)

	// CancelTask cancels a running task.
	CancelTask(ctx context.Context, endpoint string, req CancelTaskRequest) (*Task, error)

	// SubscribeToTask opens an SSE stream for task updates.
	SubscribeToTask(ctx context.Context, endpoint string, taskID string) (<-chan StreamEvent, error)

	// DiscoverAgent fetches the Agent Card from a well-known URI.
	DiscoverAgent(ctx context.Context, baseURL string) (*AgentCard, error)
}

// StreamEvent is a typed event received from an SSE subscription.
type StreamEvent struct {
	// Exactly one of these is set.
	Task           *Task                    `json:"task,omitempty"`
	Message        *Message                 `json:"message,omitempty"`
	StatusUpdate   *TaskStatusUpdateEvent   `json:"statusUpdate,omitempty"`
	ArtifactUpdate *TaskArtifactUpdateEvent `json:"artifactUpdate,omitempty"`

	// Err is set if the stream encountered an error.
	Err error `json:"-"`
}
