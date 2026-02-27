package a2a

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSEWriter writes Server-Sent Events to an http.ResponseWriter.
// Call Init once before writing any events to set the required headers.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSEWriter wrapping the given ResponseWriter.
// The ResponseWriter must implement http.Flusher for streaming to work;
// if it does not, writes will still succeed but may be buffered.
func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	f, _ := w.(http.Flusher)
	return &SSEWriter{
		w:       w,
		flusher: f,
	}
}

// Init sets the SSE response headers and flushes them to the client.
// Call this exactly once before the first WriteEvent call.
func (sw *SSEWriter) Init() {
	h := sw.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	if sw.flusher != nil {
		sw.flusher.Flush()
	}
}

// WriteEvent serializes the StreamEvent as JSON and writes it in SSE format:
//
//	data: {json}\n\n
//
// After writing, the underlying connection is flushed so the client receives
// the event immediately.
func (sw *SSEWriter) WriteEvent(event StreamEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("sse: marshal event: %w", err)
	}
	// Write the SSE data frame: "data: <json>\n\n"
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("sse: write event: %w", err)
	}
	if sw.flusher != nil {
		sw.flusher.Flush()
	}
	return nil
}

// SSEReader parses Server-Sent Events from an HTTP response body.
type SSEReader struct{}

// ReadEvents reads SSE events from body and delivers them on the returned
// channel. The channel is closed when the body is exhausted, an unrecoverable
// read error occurs, or ctx is cancelled. The body is closed when reading
// finishes.
//
// SSE format rules applied:
//   - Lines prefixed with "data: " (or "data:") carry the JSON payload.
//   - Lines starting with ":" are comments and are ignored.
//   - An empty line signals the end of an event.
//   - Multiple "data:" lines within a single event are concatenated (joined
//     with newlines) before JSON unmarshaling.
//   - Malformed JSON produces a StreamEvent with Err set; the reader continues.
func ReadEvents(ctx context.Context, body io.ReadCloser) <-chan StreamEvent {
	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		defer body.Close()

		scanner := bufio.NewScanner(body)
		var dataBuf strings.Builder

		for {
			// Check for context cancellation before each scan.
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !scanner.Scan() {
				// If there is accumulated data when the stream ends, try to
				// emit it as a final event.
				if dataBuf.Len() > 0 {
					emit(ctx, ch, dataBuf.String())
					dataBuf.Reset()
				}
				return
			}

			line := scanner.Text()

			switch {
			case line == "":
				// Empty line: end of event.
				if dataBuf.Len() > 0 {
					emit(ctx, ch, dataBuf.String())
					dataBuf.Reset()
				}

			case strings.HasPrefix(line, ":"):
				// Comment line — ignore.

			case strings.HasPrefix(line, "data: "):
				payload := strings.TrimPrefix(line, "data: ")
				if dataBuf.Len() > 0 {
					dataBuf.WriteByte('\n')
				}
				dataBuf.WriteString(payload)

			case strings.HasPrefix(line, "data:"):
				// "data:" with no space after the colon is also valid.
				payload := strings.TrimPrefix(line, "data:")
				if dataBuf.Len() > 0 {
					dataBuf.WriteByte('\n')
				}
				dataBuf.WriteString(payload)

			default:
				// Unknown field — ignore per SSE spec.
			}
		}
	}()
	return ch
}

// emit unmarshals raw into a StreamEvent and sends it on ch.
// If unmarshaling fails, a StreamEvent with Err set is sent instead.
// Respects context cancellation.
func emit(ctx context.Context, ch chan<- StreamEvent, raw string) {
	var ev StreamEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		ev = StreamEvent{Err: fmt.Errorf("sse: unmarshal event: %w", err)}
	}
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}
