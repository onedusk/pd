# Stage 4: Task Specifications — Milestone 4: A2A Agent Framework

> Implements the A2A protocol in Go. HTTP client for the orchestrator, HTTP server for agents, SSE streaming, in-memory task store, and base agent implementation.
>
> Fulfills: ADR-001 (A2A over custom protocol)

---

- [ ] **T-04.01 — Implement A2A HTTP client**
  - **File:** `internal/a2a/httpclient.go` (CREATE)
  - **Depends on:** T-01.03, T-01.04, T-01.05
  - **Outline:**
    - Define `HTTPClient` struct implementing `Client` interface, wrapping `*http.Client`
    - Constructor: `NewHTTPClient(opts ...ClientOption) *HTTPClient` — accepts options for timeout, custom HTTP client
    - `SendMessage(ctx, endpoint, req)` — POST to `{endpoint}/message:send`, marshal `req` as JSON-RPC request with method `message/send`, unmarshal JSON-RPC response, extract `Task` from result
    - `GetTask(ctx, endpoint, req)` — GET to `{endpoint}/tasks/{req.ID}`, with optional `historyLength` query param
    - `ListTasks(ctx, endpoint, req)` — GET to `{endpoint}/tasks` with query params: `contextId`, `status`, `pageSize`, `pageToken`
    - `CancelTask(ctx, endpoint, req)` — POST to `{endpoint}/tasks/{req.ID}:cancel`
    - `DiscoverAgent(ctx, baseURL)` — GET to `{baseURL}/.well-known/agent-card.json`, unmarshal `AgentCard`
    - All methods: check HTTP status, parse JSON-RPC error if present, wrap in Go error
    - Respect `ctx` for cancellation and timeouts
  - **Acceptance:** Compiles. Given a mock HTTP server, `SendMessage` sends correctly formatted JSON-RPC and parses the response. `DiscoverAgent` fetches and parses an Agent Card. Context cancellation stops in-flight requests.

---

- [ ] **T-04.02 — Write A2A HTTP client tests**
  - **File:** `internal/a2a/httpclient_test.go` (CREATE)
  - **Depends on:** T-04.01
  - **Outline:**
    - Use `httptest.NewServer` to create mock A2A servers
    - Test `SendMessage` — mock returns a completed task, verify client parses it
    - Test `SendMessage` — mock returns JSON-RPC error, verify client returns Go error with code and message
    - Test `GetTask` — mock returns task with artifacts, verify parsing
    - Test `ListTasks` — mock returns paginated response, verify `NextPageToken` is captured
    - Test `CancelTask` — verify correct HTTP method and path
    - Test `DiscoverAgent` — mock serves Agent Card JSON, verify all fields parsed
    - Test timeout: context with short deadline, mock delays, verify context error
    - Test non-200 status: mock returns 500, verify error
  - **Acceptance:** `go test ./internal/a2a/ -run TestHTTPClient` passes. All Client methods tested with happy path and error cases.

---

- [ ] **T-04.03 — Implement A2A HTTP server**
  - **File:** `internal/a2a/httpserver.go` (CREATE)
  - **Depends on:** T-01.04, T-01.05
  - **Outline:**
    - Expand `Server` struct from T-01.05 with HTTP routing
    - `Start(ctx, addr)` — create `http.ServeMux`, register routes:
      - `GET /.well-known/agent-card.json` → serve `s.card` as JSON
      - `POST /message:send` → parse JSON-RPC, dispatch to `s.handler.HandleSendMessage`
      - `GET /tasks/{id}` → dispatch to `s.handler.HandleGetTask`
      - `GET /tasks` → dispatch to `s.handler.HandleListTasks`
      - `POST /tasks/{id}:cancel` → dispatch to `s.handler.HandleCancelTask`
    - JSON-RPC envelope: parse `JSONRPCRequest`, dispatch by method, wrap result in `JSONRPCResponse`
    - Error handling: return proper JSON-RPC errors for parse failures, unknown methods, handler errors
    - `Stop(ctx)` — graceful shutdown via `s.http.Shutdown(ctx)`
    - Use Go 1.22+ `http.ServeMux` patterns with `{id}` path parameters
  - **Acceptance:** Server starts on a given address. Agent Card is served at well-known URI. JSON-RPC requests are routed to the correct handler method. Graceful shutdown completes within timeout.

---

- [ ] **T-04.04 — Write A2A HTTP server tests**
  - **File:** `internal/a2a/httpserver_test.go` (CREATE)
  - **Depends on:** T-04.03
  - **Outline:**
    - Create a mock `Handler` using testify/mock
    - Start server on random port via `httptest`
    - Test: GET `/.well-known/agent-card.json` returns valid Agent Card
    - Test: POST `/message:send` with valid JSON-RPC → handler called, response is valid JSON-RPC
    - Test: POST `/message:send` with malformed JSON → JSON-RPC parse error (-32700)
    - Test: POST `/message:send` with unknown method → method not found error (-32601)
    - Test: GET `/tasks/{id}` routes to HandleGetTask with correct ID
    - Test: GET `/tasks?contextId=X` routes to HandleListTasks with correct filter
    - Test: POST `/tasks/{id}:cancel` routes to HandleCancelTask
    - Test: graceful shutdown while request is in-flight
  - **Acceptance:** `go test ./internal/a2a/ -run TestHTTPServer` passes. All A2A endpoints respond with correct JSON-RPC formatting. Error responses use standard error codes.

---

- [ ] **T-04.05 — Implement SSE streaming**
  - **File:** `internal/a2a/sse.go` (CREATE)
  - **Depends on:** T-01.03
  - **Outline:**
    - Define `SSEWriter` struct for server-side SSE output:
      - `WriteEvent(w http.ResponseWriter, event StreamEvent) error` — format as `data: {json}\n\n`, flush
      - Set headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
    - Define `SSEReader` struct for client-side SSE parsing:
      - `ReadEvents(ctx context.Context, resp *http.Response) <-chan StreamEvent` — parse `data:` lines, unmarshal JSON into `StreamEvent`, send on channel
      - Handle reconnection: not needed for v1 (local only, connections are stable)
    - Handle context cancellation: close the event channel when ctx is done
  - **Acceptance:** `SSEWriter` produces valid SSE format (lines prefixed with `data: `, separated by `\n\n`). `SSEReader` parses SSE events into `StreamEvent` structs. Channel closes when context is canceled.

---

- [ ] **T-04.06 — Write SSE streaming tests**
  - **File:** `internal/a2a/sse_test.go` (CREATE)
  - **Depends on:** T-04.05
  - **Outline:**
    - Test `SSEWriter`: write 3 events to a `httptest.ResponseRecorder`, verify SSE format
    - Test `SSEReader`: create an `io.Pipe`, write SSE-formatted data, verify events are parsed
    - Test: status update event → `StreamEvent.StatusUpdate` is set
    - Test: artifact update event → `StreamEvent.ArtifactUpdate` is set
    - Test: context cancellation → channel closes, no goroutine leak
    - Test: malformed SSE data → event has `Err` set, channel continues
  - **Acceptance:** `go test ./internal/a2a/ -run TestSSE` passes. Events round-trip through write/read. No goroutine leaks (verified by `goleak` or manual check).

---

- [ ] **T-04.07 — Implement in-memory task store**
  - **File:** `internal/a2a/taskstore.go` (CREATE)
  - **Depends on:** T-01.03
  - **Outline:**
    - Define `TaskStore` struct — in-memory store for agent-side task tracking
    - Fields: `tasks map[string]*Task` protected by `sync.RWMutex`
    - Methods:
      - `Create(task Task) error` — stores task, fails if ID exists
      - `Get(id string) (*Task, error)` — returns copy, fails if not found
      - `Update(id string, fn func(*Task)) error` — applies mutation function under write lock
      - `List(filter ListTasksRequest) (*ListTasksResponse, error)` — filter by contextID and status, paginate
    - UUID generation for task IDs: use `crypto/rand` based UUID v4
    - Used by each specialist agent to track its own tasks
  - **Acceptance:** `Create` + `Get` round-trip works. `Update` mutates in-place. `List` filters by contextID and status. Concurrent access is safe (`go test -race`). Duplicate `Create` returns error.

---

- [ ] **T-04.08 — Implement base agent with shared boilerplate**
  - **File:** `internal/agent/base.go` (CREATE)
  - **Depends on:** T-04.03, T-04.07, T-01.08
  - **Outline:**
    - Define `BaseAgent` struct composing:
      - `a2a.Server` (HTTP serving)
      - `a2a.TaskStore` (task tracking)
      - `card a2a.AgentCard`
    - Implement `a2a.Handler` interface by delegating to an abstract `ProcessMessage` function:
      - `HandleSendMessage` — create task in SUBMITTED state, call `ProcessMessage`, update task to COMPLETED or FAILED
      - `HandleGetTask` — delegate to TaskStore
      - `HandleListTasks` — delegate to TaskStore
      - `HandleCancelTask` — update task to CANCELED if not terminal
    - Implement `Agent` interface methods:
      - `Card()` returns the stored card
      - `Start(ctx, addr)` delegates to embedded `a2a.Server.Start`
      - `Stop(ctx)` delegates to embedded `a2a.Server.Stop`
    - Specialist agents (M5) embed `BaseAgent` and implement `ProcessMessage`
    - Define `ProcessFunc` type: `func(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error)`
  - **Acceptance:** `BaseAgent` compiles. It implements both `Agent` and `a2a.Handler` interfaces. A minimal agent using `BaseAgent` can start, receive a `SendMessage`, and return a completed task with artifacts.

---

- [ ] **T-04.09 — Expand A2A client and server interfaces with implementations**
  - **File:** `internal/a2a/client.go` (MODIFY), `internal/a2a/server.go` (MODIFY), `internal/agent/agent.go` (MODIFY)
  - **Depends on:** T-04.01, T-04.03, T-04.08
  - **Outline:**
    - `client.go`: Add `var _ Client = (*HTTPClient)(nil)` compile-time interface check
    - `server.go`: Wire `httpserver.go` methods into `Server` struct (Start, Stop delegate to internal `*http.Server`)
    - `agent.go`: Add `var _ Agent = (*BaseAgent)(nil)` compile-time interface check in `base.go`
    - Ensure all concrete types satisfy their interfaces at compile time
  - **Acceptance:** `go build ./...` succeeds. Interface satisfaction is verified at compile time. No runtime type assertion panics possible.
