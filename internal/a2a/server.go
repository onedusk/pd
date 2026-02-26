package a2a

import (
	"context"
	"net/http"
)

// Handler processes incoming A2A requests for a specialist agent.
type Handler interface {
	// HandleSendMessage processes an incoming message and returns a task.
	HandleSendMessage(ctx context.Context, req SendMessageRequest) (*Task, error)

	// HandleGetTask returns the current state of a task.
	HandleGetTask(ctx context.Context, req GetTaskRequest) (*Task, error)

	// HandleListTasks returns tasks matching the filter.
	HandleListTasks(ctx context.Context, req ListTasksRequest) (*ListTasksResponse, error)

	// HandleCancelTask cancels a running task.
	HandleCancelTask(ctx context.Context, req CancelTaskRequest) (*Task, error)
}

// Server is the HTTP server that exposes an A2A agent.
type Server struct {
	card    AgentCard
	handler Handler
	http    *http.Server
}

// NewServer creates an A2A server for the given agent.
func NewServer(card AgentCard, handler Handler) *Server {
	return &Server{
		card:    card,
		handler: handler,
	}
}
