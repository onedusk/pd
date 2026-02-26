package agent

import (
	"context"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// Agent is the interface that all specialist agents implement.
type Agent interface {
	// Card returns the agent's A2A Agent Card.
	Card() a2a.AgentCard

	// HandleTask processes an A2A task and returns the completed task.
	HandleTask(ctx context.Context, task a2a.Task, msg a2a.Message) (*a2a.Task, error)

	// Start launches the agent's HTTP server on the given address.
	Start(ctx context.Context, addr string) error

	// Stop gracefully shuts down the agent.
	Stop(ctx context.Context) error
}

// Role identifies a specialist agent type.
type Role string

const (
	RoleResearch   Role = "research"
	RoleSchema     Role = "schema"
	RolePlanning   Role = "planning"
	RoleTaskWriter Role = "task-writer"
)
