package a2a

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// T-04.06  SSE streaming tests
// ---------------------------------------------------------------------------

func TestSSEWriter_WritesValidSSEFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewSSEWriter(rec)
	w.Init()

	events := []StreamEvent{
		{StatusUpdate: &TaskStatusUpdateEvent{TaskID: "t1", ContextID: "c1", Status: TaskStatus{State: TaskStateWorking}}},
		{StatusUpdate: &TaskStatusUpdateEvent{TaskID: "t2", ContextID: "c2", Status: TaskStatus{State: TaskStateCompleted}}},
		{ArtifactUpdate: &TaskArtifactUpdateEvent{TaskID: "t3", ContextID: "c3", Artifact: Artifact{ArtifactID: "a1", Name: "out"}}},
	}

	for _, ev := range events {
		require.NoError(t, w.WriteEvent(ev))
	}

	// Verify SSE headers.
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))

	// Verify the body contains 3 SSE frames: "data: {json}\n\n".
	body := rec.Body.String()
	frames := strings.Split(body, "\n\n")
	// Split produces an extra empty element after the final "\n\n".
	nonEmpty := make([]string, 0, len(frames))
	for _, f := range frames {
		if strings.TrimSpace(f) != "" {
			nonEmpty = append(nonEmpty, f)
		}
	}
	assert.Len(t, nonEmpty, 3, "expected 3 SSE frames")

	for _, frame := range nonEmpty {
		assert.True(t, strings.HasPrefix(frame, "data: "), "each frame must start with 'data: ', got: %s", frame)
		// The payload after "data: " must be valid JSON (it was marshaled).
		payload := strings.TrimPrefix(frame, "data: ")
		assert.True(t, len(payload) > 0, "payload must not be empty")
		assert.True(t, payload[0] == '{', "payload must be JSON object, got: %s", payload)
	}
}

func TestSSEReader_ParsesEvents(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		// Write two valid SSE frames.
		fmt.Fprint(pw, "data: {\"statusUpdate\":{\"taskId\":\"t1\",\"contextId\":\"c1\",\"status\":{\"state\":\"working\"}}}\n\n")
		fmt.Fprint(pw, "data: {\"statusUpdate\":{\"taskId\":\"t2\",\"contextId\":\"c2\",\"status\":{\"state\":\"completed\"}}}\n\n")
	}()

	ch := ReadEvents(context.Background(), pr)

	ev1 := <-ch
	require.NoError(t, ev1.Err)
	require.NotNil(t, ev1.StatusUpdate)
	assert.Equal(t, "t1", ev1.StatusUpdate.TaskID)
	assert.Equal(t, TaskStateWorking, ev1.StatusUpdate.Status.State)

	ev2 := <-ch
	require.NoError(t, ev2.Err)
	require.NotNil(t, ev2.StatusUpdate)
	assert.Equal(t, "t2", ev2.StatusUpdate.TaskID)
	assert.Equal(t, TaskStateCompleted, ev2.StatusUpdate.Status.State)

	// Channel should close after pipe is exhausted.
	_, open := <-ch
	assert.False(t, open, "channel should be closed after body is exhausted")
}

func TestSSEReader_StatusUpdateEvent(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		fmt.Fprint(pw, "data: {\"statusUpdate\":{\"taskId\":\"task-42\",\"contextId\":\"ctx-7\",\"status\":{\"state\":\"input-required\"}}}\n\n")
	}()

	ch := ReadEvents(context.Background(), pr)
	ev := <-ch

	require.NoError(t, ev.Err)
	require.NotNil(t, ev.StatusUpdate, "StatusUpdate must be set")
	assert.Nil(t, ev.Task, "Task must be nil")
	assert.Nil(t, ev.Message, "Message must be nil")
	assert.Nil(t, ev.ArtifactUpdate, "ArtifactUpdate must be nil")

	assert.Equal(t, "task-42", ev.StatusUpdate.TaskID)
	assert.Equal(t, "ctx-7", ev.StatusUpdate.ContextID)
	assert.Equal(t, TaskStateInputRequired, ev.StatusUpdate.Status.State)
}

func TestSSEReader_ArtifactUpdateEvent(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		fmt.Fprint(pw, `data: {"artifactUpdate":{"taskId":"task-99","contextId":"ctx-5","artifact":{"artifactId":"art-1","name":"report","parts":[{"text":"hello"}]},"append":true,"lastChunk":false}}`+"\n\n")
	}()

	ch := ReadEvents(context.Background(), pr)
	ev := <-ch

	require.NoError(t, ev.Err)
	require.NotNil(t, ev.ArtifactUpdate, "ArtifactUpdate must be set")
	assert.Nil(t, ev.StatusUpdate, "StatusUpdate must be nil")

	assert.Equal(t, "task-99", ev.ArtifactUpdate.TaskID)
	assert.Equal(t, "ctx-5", ev.ArtifactUpdate.ContextID)
	assert.Equal(t, "art-1", ev.ArtifactUpdate.Artifact.ArtifactID)
	assert.Equal(t, "report", ev.ArtifactUpdate.Artifact.Name)
	assert.True(t, ev.ArtifactUpdate.Append)
	assert.False(t, ev.ArtifactUpdate.LastChunk)
	require.Len(t, ev.ArtifactUpdate.Artifact.Parts, 1)
	assert.Equal(t, "hello", ev.ArtifactUpdate.Artifact.Parts[0].Text)
}

func TestSSEReader_ContextCancellation(t *testing.T) {
	pr, pw := io.Pipe()
	// Keep the pipe open; do not close pw so the reader blocks.
	defer pw.Close()

	ctx, cancel := context.WithCancel(context.Background())

	ch := ReadEvents(ctx, pr)

	// Cancel the context.
	cancel()

	// The channel must close within a reasonable timeout.
	select {
	case _, open := <-ch:
		assert.False(t, open, "channel should be closed after context cancellation")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel to close after context cancellation")
	}
}

func TestSSEReader_MalformedSSEData(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		// First event: malformed JSON.
		fmt.Fprint(pw, "data: {not valid json!!!}\n\n")
		// Second event: valid JSON.
		fmt.Fprint(pw, "data: {\"statusUpdate\":{\"taskId\":\"t-ok\",\"contextId\":\"c-ok\",\"status\":{\"state\":\"working\"}}}\n\n")
	}()

	ch := ReadEvents(context.Background(), pr)

	// First event should have Err set.
	ev1 := <-ch
	assert.Error(t, ev1.Err, "malformed JSON should produce an error event")
	assert.Contains(t, ev1.Err.Error(), "unmarshal")

	// Second event should parse correctly -- reader continues after error.
	ev2 := <-ch
	require.NoError(t, ev2.Err)
	require.NotNil(t, ev2.StatusUpdate)
	assert.Equal(t, "t-ok", ev2.StatusUpdate.TaskID)

	// Channel should close.
	_, open := <-ch
	assert.False(t, open, "channel should be closed after body is exhausted")
}

func TestSSEWriter_RoundTrip(t *testing.T) {
	// Write events through SSEWriter, feed the output to ReadEvents,
	// and verify the round-trip fidelity.
	rec := httptest.NewRecorder()
	w := NewSSEWriter(rec)
	w.Init()

	sent := []StreamEvent{
		{StatusUpdate: &TaskStatusUpdateEvent{
			TaskID:    "rt-1",
			ContextID: "ctx-rt",
			Status:    TaskStatus{State: TaskStateWorking},
		}},
		{ArtifactUpdate: &TaskArtifactUpdateEvent{
			TaskID:    "rt-1",
			ContextID: "ctx-rt",
			Artifact: Artifact{
				ArtifactID: "a-rt",
				Name:       "output",
				Parts:      []Part{TextPart("result text")},
			},
			LastChunk: true,
		}},
		{StatusUpdate: &TaskStatusUpdateEvent{
			TaskID:    "rt-1",
			ContextID: "ctx-rt",
			Status:    TaskStatus{State: TaskStateCompleted},
		}},
	}

	for _, ev := range sent {
		require.NoError(t, w.WriteEvent(ev))
	}

	// Feed the recorded body into ReadEvents.
	body := io.NopCloser(strings.NewReader(rec.Body.String()))
	ch := ReadEvents(context.Background(), body)

	var received []StreamEvent
	for ev := range ch {
		require.NoError(t, ev.Err)
		received = append(received, ev)
	}

	require.Len(t, received, 3)

	// Verify first event.
	require.NotNil(t, received[0].StatusUpdate)
	assert.Equal(t, "rt-1", received[0].StatusUpdate.TaskID)
	assert.Equal(t, TaskStateWorking, received[0].StatusUpdate.Status.State)

	// Verify second event.
	require.NotNil(t, received[1].ArtifactUpdate)
	assert.Equal(t, "a-rt", received[1].ArtifactUpdate.Artifact.ArtifactID)
	assert.True(t, received[1].ArtifactUpdate.LastChunk)
	require.Len(t, received[1].ArtifactUpdate.Artifact.Parts, 1)
	assert.Equal(t, "result text", received[1].ArtifactUpdate.Artifact.Parts[0].Text)

	// Verify third event.
	require.NotNil(t, received[2].StatusUpdate)
	assert.Equal(t, TaskStateCompleted, received[2].StatusUpdate.Status.State)
}

func TestSSEReader_CommentsIgnored(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		// SSE comment lines (starting with ':') should be ignored.
		fmt.Fprint(pw, ": this is a comment\n")
		fmt.Fprint(pw, "data: {\"statusUpdate\":{\"taskId\":\"t-comment\",\"contextId\":\"c-1\",\"status\":{\"state\":\"submitted\"}}}\n\n")
	}()

	ch := ReadEvents(context.Background(), pr)
	ev := <-ch
	require.NoError(t, ev.Err)
	require.NotNil(t, ev.StatusUpdate)
	assert.Equal(t, "t-comment", ev.StatusUpdate.TaskID)
}

func TestSSEReader_DataNoSpace(t *testing.T) {
	// "data:" with no space after colon is valid SSE.
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		fmt.Fprint(pw, "data:{\"statusUpdate\":{\"taskId\":\"t-ns\",\"contextId\":\"c-ns\",\"status\":{\"state\":\"working\"}}}\n\n")
	}()

	ch := ReadEvents(context.Background(), pr)
	ev := <-ch
	require.NoError(t, ev.Err)
	require.NotNil(t, ev.StatusUpdate)
	assert.Equal(t, "t-ns", ev.StatusUpdate.TaskID)
}
