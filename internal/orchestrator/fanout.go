package orchestrator

import (
	"context"
	"sync"

	"github.com/dusk-indust/decompose/internal/a2a"
	"golang.org/x/sync/errgroup"
)

// AgentTask describes a single unit of work to send to a remote A2A agent.
type AgentTask struct {
	// AgentEndpoint is the URL of the target A2A agent.
	AgentEndpoint string

	// Message is the A2A message to send.
	Message a2a.Message

	// Section identifies which section of the stage this task produces.
	Section string
}

// AgentResult holds the outcome of a single AgentTask after fan-out.
type AgentResult struct {
	// Section identifies which section of the stage this result belongs to.
	Section string

	// Artifacts are the outputs produced by the agent on success.
	Artifacts []a2a.Artifact

	// Err is non-nil if the agent call failed.
	Err error

	// Task is the full A2A task returned by the agent.
	Task *a2a.Task
}

// FanOut dispatches AgentTasks to remote A2A agents in parallel and collects
// their results. If any agent fails, the derived context is canceled so that
// remaining in-flight calls are abandoned promptly.
type FanOut struct {
	client     a2a.Client
	onProgress func(ProgressEvent)
	mu         sync.Mutex // guards nothing at struct level; kept for future use
}

// NewFanOut creates a FanOut that dispatches tasks via client.
// onProgress is called synchronously from each goroutine; it may be nil.
func NewFanOut(client a2a.Client, onProgress func(ProgressEvent)) *FanOut {
	return &FanOut{
		client:     client,
		onProgress: onProgress,
	}
}

// Run dispatches every task in parallel, emitting progress events for each.
// It uses errgroup.WithContext so that the first agent failure cancels the
// derived context, causing remaining SendMessage calls to return early.
//
// All collected AgentResults are returned regardless of whether an error
// occurred. The returned error is the first non-nil error from the errgroup.
func (f *FanOut) Run(ctx context.Context, stage Stage, tasks []AgentTask) ([]AgentResult, error) {
	results := make([]AgentResult, len(tasks))
	g, gctx := errgroup.WithContext(ctx)

	for i, task := range tasks {
		f.emit(ProgressEvent{
			Stage:   stage,
			Section: task.Section,
			Status:  ProgressPending,
		})

		g.Go(func() error {
			f.emit(ProgressEvent{
				Stage:   stage,
				Section: task.Section,
				Status:  ProgressWorking,
			})

			req := a2a.SendMessageRequest{
				Message:       task.Message,
				Configuration: &a2a.SendMessageConfig{Blocking: true},
			}

			t, err := f.client.SendMessage(gctx, task.AgentEndpoint, req)
			if err != nil {
				results[i] = AgentResult{
					Section: task.Section,
					Err:     err,
				}
				f.emit(ProgressEvent{
					Stage:   stage,
					Section: task.Section,
					Status:  ProgressFailed,
					Message: err.Error(),
				})
				return err // triggers context cancellation for other goroutines
			}

			results[i] = AgentResult{
				Section:   task.Section,
				Artifacts: t.Artifacts,
				Task:      t,
			}
			f.emit(ProgressEvent{
				Stage:   stage,
				Section: task.Section,
				Status:  ProgressComplete,
			})
			return nil
		})
	}

	err := g.Wait()
	return results, err
}

// emit sends a progress event if a callback is registered.
func (f *FanOut) emit(ev ProgressEvent) {
	if f.onProgress != nil {
		f.onProgress(ev)
	}
}
