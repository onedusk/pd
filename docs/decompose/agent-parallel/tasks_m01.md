# Stage 4: Task Specifications — Milestone 1: Project Scaffolding + A2A Types

> Foundation layer: Go module, all shared type definitions from Stage 2 skeletons, CLI entry point shell, and serialization tests. Everything else depends on this milestone.
>
> Fulfills: ADR-005 (Go), ADR-001 (A2A types), PDR-002 (single binary entry point)

---

- [ ] **T-01.01 — Initialize Go module and dependencies**
  - **File:** `go.mod` (CREATE), `go.sum` (CREATE), `LICENSE` (CREATE)
  - **Depends on:** None
  - **Outline:**
    - Run `go mod init github.com/dusk-indust/decompose`
    - Add dependencies: `github.com/stretchr/testify` (testing)
    - Copy PolyForm Shield 1.0.0 license text to `LICENSE`
    - No other dependencies yet — A2A types are hand-written, graph/MCP dependencies come in M2/M3
  - **Acceptance:** `go build ./...` succeeds. `go mod tidy` makes no changes. LICENSE file exists.

---

- [ ] **T-01.02 — Create CLI entry point shell**
  - **File:** `cmd/decompose/main.go` (CREATE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Define `cliFlags` struct: `ProjectRoot`, `OutputDir`, `Agents`, `SingleAgent`, `Verbose`, `Version` (all from Stage 2 skeleton)
    - Parse flags using `flag` stdlib package
    - Implement `main()` that calls `run(os.Args[1:])` and exits on error
    - `run()` parses flags, prints version if `--version`, otherwise returns `nil` (orchestrator wiring comes in M6)
    - Set `var version = "dev"` for goreleaser injection
  - **Acceptance:** `go run ./cmd/decompose --version` prints `dev`. `go run ./cmd/decompose` exits with code 0. Unknown flags produce an error message.

---

- [ ] **T-01.03 — Implement A2A core types**
  - **File:** `internal/a2a/types.go` (CREATE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Copy the `a2a` package types from Stage 2 skeleton: `TaskState` (string enum with 8 values + `IsTerminal()` method), `Role` (string enum), `Task`, `TaskStatus`, `Message`, `Part` (with `TextPart`/`DataPart` helpers), `Artifact`
    - Copy agent card types: `AgentCard`, `AgentInterface`, `AgentProvider`, `AgentCapabilities`, `AgentSkill`
    - Copy streaming types: `TaskStatusUpdateEvent`, `TaskArtifactUpdateEvent`
    - Copy request/response types: `SendMessageRequest`, `SendMessageConfig`, `GetTaskRequest`, `ListTasksRequest`, `ListTasksResponse`, `CancelTaskRequest`
    - All JSON tags must match A2A spec field names (camelCase)
  - **Acceptance:** Package compiles. All types from Stage 2 `internal/a2a/types.go` skeleton are present. `json.Marshal`/`Unmarshal` round-trips preserve all fields.

---

- [ ] **T-01.04 — Implement A2A JSON-RPC types**
  - **File:** `internal/a2a/jsonrpc.go` (CREATE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Copy from Stage 2 skeleton: `JSONRPCRequest`, `JSONRPCResponse`, `JSONRPCError` structs
    - Define constants: `JSONRPCVersion = "2.0"`, error codes (`ErrCodeParse`, `ErrCodeInvalidRequest`, `ErrCodeMethodNotFound`, `ErrCodeInvalidParams`, `ErrCodeInternal`, `ErrCodeTaskNotFound`, `ErrCodeTaskNotCancelable`)
    - Define method name constants: `MethodSendMessage`, `MethodStreamMessage`, `MethodGetTask`, `MethodListTasks`, `MethodCancelTask`
  - **Acceptance:** Package compiles. All JSON-RPC constants are defined. `JSONRPCResponse` can hold either `Result` or `Error` (not both).

---

- [ ] **T-01.05 — Define A2A client and server interfaces**
  - **File:** `internal/a2a/client.go` (CREATE), `internal/a2a/server.go` (CREATE)
  - **Depends on:** T-01.03
  - **Outline:**
    - `client.go`: Copy `Client` interface and `StreamEvent` struct from Stage 2 skeleton. Methods: `SendMessage`, `GetTask`, `ListTasks`, `CancelTask`, `SubscribeToTask`, `DiscoverAgent`. No implementation — interface only.
    - `server.go`: Copy `Handler` interface from Stage 2 skeleton. Methods: `HandleSendMessage`, `HandleGetTask`, `HandleListTasks`, `HandleCancelTask`. Define `Server` struct shell with `NewServer(card, handler)` constructor. No HTTP wiring yet — that's M4.
  - **Acceptance:** Package compiles. Both interfaces are exported. `NewServer` returns a non-nil `*Server`.

---

- [ ] **T-01.06 — Implement graph schema types**
  - **File:** `internal/graph/schema.go` (CREATE)
  - **Depends on:** T-01.01
  - **Outline:**
    - Copy from Stage 2 skeleton: `NodeKind` (3 values), `SymbolKind` (7 values), `EdgeKind` (6 values), `Language` (4 values + `Tier1Languages` slice)
    - Copy model structs: `FileNode`, `SymbolNode`, `ClusterNode`, `Edge`
    - Copy result types: `GraphStats`, `DependencyChain`, `ImpactResult`
    - All structs have JSON tags for serialization
  - **Acceptance:** Package compiles. All enums and model types from Stage 2 `internal/graph/schema.go` are present. `Tier1Languages` contains exactly Go, TypeScript, Python, Rust.

---

- [ ] **T-01.07 — Define graph Store and Parser interfaces**
  - **File:** `internal/graph/store.go` (CREATE), `internal/graph/parser.go` (CREATE)
  - **Depends on:** T-01.06
  - **Outline:**
    - `store.go`: Copy `Store` interface from Stage 2 skeleton. Methods: `InitSchema`, `AddFile`, `AddSymbol`, `AddCluster`, `AddEdge`, `GetFile`, `GetSymbol`, `QuerySymbols`, `GetDependencies`, `AssessImpact`, `GetClusters`, `Stats`, `Close`. Copy `Direction` type.
    - `parser.go`: Copy `Parser` interface and `ParseResult` struct from Stage 2 skeleton. Methods: `Parse`, `SupportedLanguages`, `Close`.
    - Interfaces only — implementations come in M2
  - **Acceptance:** Package compiles. Both interfaces are exported. `Store` embeds `io.Closer`. `ParseResult` contains `FileNode`, `[]SymbolNode`, `[]Edge`.

---

- [ ] **T-01.08 — Implement orchestrator config, types, and agent interface**
  - **File:** `internal/orchestrator/config.go` (CREATE), `internal/orchestrator/orchestrator.go` (CREATE), `internal/orchestrator/merge.go` (CREATE), `internal/orchestrator/detector.go` (CREATE), `internal/agent/agent.go` (CREATE)
  - **Depends on:** T-01.03
  - **Outline:**
    - `config.go`: Copy `CapabilityLevel` (4 levels with `String()`) and `Config` struct from Stage 2 skeleton
    - `orchestrator.go`: Copy `Stage` (5 values with `String()`), `StageResult`, `Section`, `ProgressEvent`, `ProgressStatus`, and `Orchestrator` interface from Stage 2 skeleton
    - `merge.go`: Copy `MergeStrategy`, `MergePlan`, `CoherenceIssue` types from Stage 2 skeleton
    - `detector.go`: Copy `Detector` interface from Stage 2 skeleton
    - `agent.go`: Copy `Agent` interface and `Role` type from Stage 2 skeleton
  - **Acceptance:** All five files compile. `CapabilityLevel` constants order correctly (`CapBasic < CapMCPOnly < CapA2AMCP < CapFull`). `Stage` has values 0–4. `Orchestrator` interface has `RunStage`, `RunPipeline`, `Progress` methods.

---

- [ ] **T-01.09 — Write A2A serialization round-trip tests**
  - **File:** `internal/a2a/types_test.go` (CREATE)
  - **Depends on:** T-01.03, T-01.04
  - **Outline:**
    - Table-driven tests using testify `assert`/`require`
    - Test `Task` marshal/unmarshal with all `TaskState` values
    - Test `Message` with multi-part content (text + data)
    - Test `AgentCard` with skills and capabilities
    - Test `SendMessageRequest` with and without configuration
    - Test `Part` with each content type: text, raw (base64), URL, data
    - Test `TaskState.IsTerminal()` for all 8 states
    - Test JSON-RPC request/response envelope round-trips
  - **Acceptance:** `go test ./internal/a2a/...` passes. All round-trip tests verify `original == unmarshal(marshal(original))`. `IsTerminal` returns `true` for completed/failed/canceled/rejected, `false` for all others.
