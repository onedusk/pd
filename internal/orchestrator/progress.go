package orchestrator

import "fmt"

// ProgressReporter emits progress events through a buffered channel.
type ProgressReporter struct {
	ch chan ProgressEvent
}

// NewProgressReporter creates a ProgressReporter with a buffered channel of size 64.
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{
		ch: make(chan ProgressEvent, 64),
	}
}

// Emit sends a progress event in a non-blocking fashion.
// If the channel is full, the event is silently dropped.
func (pr *ProgressReporter) Emit(event ProgressEvent) {
	select {
	case pr.ch <- event:
	default:
		// Drop the event if the channel is full.
	}
}

// Subscribe returns a read-only channel for consuming progress events.
func (pr *ProgressReporter) Subscribe() <-chan ProgressEvent {
	return pr.ch
}

// Close closes the progress event channel.
func (pr *ProgressReporter) Close() {
	close(pr.ch)
}

// FormatProgress formats a ProgressEvent as a human-readable status line.
func FormatProgress(event ProgressEvent) string {
	switch event.Status {
	case ProgressPending:
		return fmt.Sprintf("  \u25cb %s (pending)", event.Section)
	case ProgressWorking:
		return fmt.Sprintf("  \u25cf %s...", event.Section)
	case ProgressComplete:
		return fmt.Sprintf("  \u2713 %s complete", event.Section)
	case ProgressFailed:
		return fmt.Sprintf("  \u2717 %s failed: %s", event.Section, event.Message)
	default:
		return fmt.Sprintf("  ? %s (unknown status)", event.Section)
	}
}

// FormatStageHeader formats a stage header for display.
// Returns: "[{name}] Stage {N}: {stage.String()}"
func FormatStageHeader(name string, stage Stage) string {
	return fmt.Sprintf("[%s] Stage %d: %s", name, int(stage), stage.String())
}
