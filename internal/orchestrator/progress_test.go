package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressReporter_EmitAndSubscribe(t *testing.T) {
	pr := NewProgressReporter()
	defer pr.Close()

	ch := pr.Subscribe()
	want := ProgressEvent{
		Stage:   StageDesignPack,
		Section: "architecture",
		Status:  ProgressWorking,
		Message: "generating",
	}

	pr.Emit(want)

	select {
	case got := <-ch:
		assert.Equal(t, want, got)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for progress event")
	}
}

func TestProgressReporter_EmitWhenFull_DoesNotBlock(t *testing.T) {
	pr := NewProgressReporter()
	defer pr.Close()

	// The internal channel buffer is 64. Emitting 100 events must never block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			pr.Emit(ProgressEvent{
				Stage:   StageDesignPack,
				Section: "section",
				Status:  ProgressWorking,
				Message: "msg",
			})
		}
		close(done)
	}()

	select {
	case <-done:
		// Success: all 100 emits returned without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("Emit blocked when the channel was full")
	}
}

func TestProgressReporter_Close_ChannelClosed(t *testing.T) {
	pr := NewProgressReporter()
	ch := pr.Subscribe()

	pr.Emit(ProgressEvent{
		Stage:   StageTaskIndex,
		Section: "progress",
		Status:  ProgressComplete,
	})
	pr.Close()

	// Range over the channel; it must terminate because Close was called.
	var received []ProgressEvent
	for ev := range ch {
		received = append(received, ev)
	}
	require.Len(t, received, 1)
	assert.Equal(t, ProgressComplete, received[0].Status)
}

func TestFormatProgress_AllStatuses(t *testing.T) {
	tests := []struct {
		name   string
		event  ProgressEvent
		expect string
	}{
		{
			name:   "pending",
			event:  ProgressEvent{Section: "data-model", Status: ProgressPending},
			expect: "  \u25cb data-model (pending)",
		},
		{
			name:   "working",
			event:  ProgressEvent{Section: "data-model", Status: ProgressWorking},
			expect: "  \u25cf data-model...",
		},
		{
			name:   "complete",
			event:  ProgressEvent{Section: "data-model", Status: ProgressComplete},
			expect: "  \u2713 data-model complete",
		},
		{
			name:   "failed",
			event:  ProgressEvent{Section: "data-model", Status: ProgressFailed, Message: "timeout"},
			expect: "  \u2717 data-model failed: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatProgress(tt.event)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestFormatStageHeader(t *testing.T) {
	got := FormatStageHeader("my-project", StageDesignPack)
	assert.Equal(t, "[my-project] Stage 1: design-pack", got)
}
