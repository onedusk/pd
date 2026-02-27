package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// Compile-time interface checks.
var (
	_ Agent       = (*BaseAgent)(nil)
	_ a2a.Handler = (*BaseAgent)(nil)
)

// ProcessFunc is the function that specialist agents implement to handle
// incoming messages. It receives the task (in WORKING state) and the message,
// and returns artifacts to attach to the completed task.
type ProcessFunc func(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error)

// BaseAgent provides shared boilerplate for specialist agents. It composes an
// A2A server and task store, implementing both the Agent and a2a.Handler
// interfaces. Specialist agents embed BaseAgent and provide a ProcessFunc.
type BaseAgent struct {
	server  *a2a.Server
	store   *a2a.TaskStore
	card    a2a.AgentCard
	process ProcessFunc
}

// NewBaseAgent creates a BaseAgent with the given card and process function.
func NewBaseAgent(card a2a.AgentCard, process ProcessFunc) *BaseAgent {
	b := &BaseAgent{
		store:   a2a.NewTaskStore(),
		card:    card,
		process: process,
	}
	b.server = a2a.NewServer(card, b)
	return b
}

// Card returns the agent's A2A Agent Card.
func (b *BaseAgent) Card() a2a.AgentCard {
	return b.card
}

// HandleTask processes an A2A task with a message and returns the completed task.
func (b *BaseAgent) HandleTask(ctx context.Context, task a2a.Task, msg a2a.Message) (*a2a.Task, error) {
	// Store the task in SUBMITTED state.
	task.Status = a2a.TaskStatus{
		State:     a2a.TaskStateSubmitted,
		Timestamp: time.Now(),
	}
	if err := b.store.Create(task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	// Transition to WORKING.
	if err := b.store.Update(task.ID, func(t *a2a.Task) {
		t.Status = a2a.TaskStatus{
			State:     a2a.TaskStateWorking,
			Timestamp: time.Now(),
		}
	}); err != nil {
		return nil, fmt.Errorf("update task to working: %w", err)
	}

	// Run the specialist's process function.
	artifacts, err := b.process(ctx, &task, msg)
	if err != nil {
		// Transition to FAILED.
		_ = b.store.Update(task.ID, func(t *a2a.Task) {
			t.Status = a2a.TaskStatus{
				State:     a2a.TaskStateFailed,
				Timestamp: time.Now(),
				Message:   &a2a.Message{Role: a2a.RoleAgent, Parts: []a2a.Part{a2a.TextPart(err.Error())}},
			}
		})
		result, _ := b.store.Get(task.ID)
		return result, err
	}

	// Transition to COMPLETED with artifacts.
	if err := b.store.Update(task.ID, func(t *a2a.Task) {
		t.Status = a2a.TaskStatus{
			State:     a2a.TaskStateCompleted,
			Timestamp: time.Now(),
		}
		t.Artifacts = artifacts
	}); err != nil {
		return nil, fmt.Errorf("update task to completed: %w", err)
	}

	return b.store.Get(task.ID)
}

// Start launches the agent's HTTP server on the given address.
func (b *BaseAgent) Start(ctx context.Context, addr string) error {
	return b.server.Start(ctx, addr)
}

// Stop gracefully shuts down the agent.
func (b *BaseAgent) Stop(ctx context.Context) error {
	return b.server.Stop(ctx)
}

// --- a2a.Handler implementation ---

// HandleSendMessage creates a task from the incoming message and processes it.
func (b *BaseAgent) HandleSendMessage(ctx context.Context, req a2a.SendMessageRequest) (*a2a.Task, error) {
	task := a2a.Task{
		ID:        a2a.NewTaskID(),
		ContextID: req.Message.ContextID,
	}
	return b.HandleTask(ctx, task, req.Message)
}

// HandleGetTask retrieves a task by ID from the store.
func (b *BaseAgent) HandleGetTask(_ context.Context, req a2a.GetTaskRequest) (*a2a.Task, error) {
	return b.store.Get(req.ID)
}

// HandleListTasks returns tasks matching the filter.
func (b *BaseAgent) HandleListTasks(_ context.Context, req a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	return b.store.List(req)
}

// HandleCancelTask cancels a running task if it is not in a terminal state.
func (b *BaseAgent) HandleCancelTask(_ context.Context, req a2a.CancelTaskRequest) (*a2a.Task, error) {
	err := b.store.Update(req.ID, func(t *a2a.Task) {
		if !t.Status.State.IsTerminal() {
			t.Status = a2a.TaskStatus{
				State:     a2a.TaskStateCanceled,
				Timestamp: time.Now(),
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return b.store.Get(req.ID)
}
