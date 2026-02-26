# Stage 4: Task Specifications — Milestone 3: MCP Tool Servers

> Wraps the code intelligence layer from M2 as MCP tools using the official Go SDK (v1.3.1). Five tools: `build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`.
>
> Fulfills: ADR-002 (MCP for tool integration)

---

- [ ] **T-03.01 — Define MCP tool input/output types**
  - **File:** `internal/mcptools/codeintel.go` (CREATE)
  - **Depends on:** T-01.06
  - **Outline:**
    - Copy from Stage 2 skeleton: `BuildGraphInput`, `BuildGraphOutput`, `QuerySymbolsInput`, `QuerySymbolsOutput`, `GetDependenciesInput`, `GetDependenciesOutput`, `AssessImpactInput`, `AssessImpactOutput`, `GetClustersInput`, `GetClustersOutput`
    - All input structs have `json` and `jsonschema` struct tags for MCP auto-schema generation
    - Import `github.com/dusk-indust/decompose/internal/graph` for result types
  - **Acceptance:** Package compiles. All 5 input types and 5 output types are present. `jsonschema` tags exist on all required fields.

---

- [ ] **T-03.02 — Implement MCP tool handlers**
  - **File:** `internal/mcptools/handlers.go` (CREATE)
  - **Depends on:** T-03.01, T-02.01, T-02.06
  - **Outline:**
    - Define `CodeIntelService` struct holding a `graph.Store` and `graph.Parser`
    - Constructor: `NewCodeIntelService(store graph.Store, parser graph.Parser) *CodeIntelService`
    - Implement 5 handler functions matching `mcp.ToolHandlerFor[In, Out]` signature:
      - `BuildGraph(ctx, req, input BuildGraphInput) (*mcp.CallToolResult, BuildGraphOutput, error)` — walk `input.RepoPath` recursively, filter by `input.Languages` and `input.ExcludeDirs`, parse each file, insert into store, run clustering, return stats
      - `QuerySymbols(ctx, req, input QuerySymbolsInput) (*mcp.CallToolResult, QuerySymbolsOutput, error)` — delegate to `store.QuerySymbols`, apply default limit of 20
      - `GetDependencies(ctx, req, input GetDependenciesInput) (*mcp.CallToolResult, GetDependenciesOutput, error)` — parse direction (default "downstream"), maxDepth (default 5), delegate to `store.GetDependencies`
      - `AssessImpact(ctx, req, input AssessImpactInput) (*mcp.CallToolResult, AssessImpactOutput, error)` — delegate to `store.AssessImpact`
      - `GetClusters(ctx, req, input GetClustersInput) (*mcp.CallToolResult, GetClustersOutput, error)` — delegate to `store.GetClusters`
    - `BuildGraph` walks the filesystem using `filepath.WalkDir`, skips `input.ExcludeDirs`, detects language from file extension (`.go`→Go, `.ts`/`.tsx`→TypeScript, `.py`→Python, `.rs`→Rust)
    - All handlers return `nil` for `*mcp.CallToolResult` (SDK auto-populates from output struct)
  - **Acceptance:** Each handler compiles and accepts the correct input type. `BuildGraph` with a fixture project returns non-zero `GraphStats`. `QuerySymbols` returns matching symbols. All handlers return errors (not panics) for invalid inputs.

---

- [ ] **T-03.03 — Wire MCP server with tool registration**
  - **File:** `internal/mcptools/server.go` (CREATE)
  - **Depends on:** T-03.02
  - **Outline:**
    - Define `NewCodeIntelMCPServer(svc *CodeIntelService) *mcp.Server`
    - Create server: `mcp.NewServer(&mcp.Implementation{Name: "decompose-codeintel", Version: version}, opts)`
    - Register 5 tools using `mcp.AddTool(server, &mcp.Tool{Name: "build_graph", Description: "..."}, svc.BuildGraph)` pattern
    - Tool names: `build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`
    - Tool descriptions match Stage 2 API reference
    - Define `RunMCPServer(ctx context.Context, svc *CodeIntelService, addr string) error` — creates server, wraps in `mcp.NewStreamableHTTPHandler`, serves on `addr`
  - **Acceptance:** `NewCodeIntelMCPServer` returns a server with 5 tools registered. `RunMCPServer` starts an HTTP server that accepts MCP connections. Server responds to `tools/list` with 5 tools.

---

- [ ] **T-03.04 — Write MCP tool handler unit tests**
  - **File:** `internal/mcptools/handlers_test.go` (CREATE)
  - **Depends on:** T-03.02, T-02.07, T-02.09
  - **Outline:**
    - Use `MemStore` (no CGO) and a real `TreeSitterParser` (requires CGO — use build tag)
    - Test `BuildGraph`:
      - Input: `testdata/fixtures/go_project/`, languages: `["go"]`
      - Assert: stats have non-zero file and symbol counts
      - Input with non-existent path: returns error
      - Input with empty languages: defaults to tier-1
    - Test `QuerySymbols`:
      - Pre-populate store with known symbols
      - Query substring match: returns matching symbols
      - Query with kind filter: returns only that kind
      - Query with limit: respects limit
    - Test `GetDependencies`:
      - Pre-populate store with A→B→C chain
      - Downstream from A: returns chain [A,B,C]
      - Upstream from C: returns chain [C,B,A]
    - Test `AssessImpact`:
      - Pre-populate with diamond: A→B, A→C, B→D, C→D
      - Change A: all files affected
      - Change D: no downstream (leaf)
    - Test `GetClusters`:
      - Pre-populate and run clustering
      - Returns non-empty clusters
  - **Acceptance:** `go test ./internal/mcptools/ -run TestHandlers` passes. Each tool handler has at least 2 test cases (happy path + edge case).

---

- [ ] **T-03.05 — Write MCP server integration tests**
  - **File:** `internal/mcptools/server_test.go` (CREATE)
  - **Depends on:** T-03.03, T-03.04
  - **Outline:**
    - Use `mcp.NewInMemoryTransports()` to test without HTTP
    - Create a `CodeIntelService` with `MemStore`
    - Create server and client, connect via in-memory transport
    - Test: `session.ListTools()` returns 5 tools with correct names
    - Test: `session.CallTool("build_graph", ...)` with fixture path returns stats
    - Test: `session.CallTool("query_symbols", ...)` returns results
    - Test: calling unknown tool returns error
  - **Acceptance:** `go test ./internal/mcptools/ -run TestServer` passes. Client can list tools and call each tool through the MCP protocol layer. Error responses are properly formatted.

---

- [ ] **T-03.06 — Add MCP SDK dependency to go.mod**
  - **File:** `go.mod` (MODIFY)
  - **Depends on:** T-01.01
  - **Outline:**
    - Run `go get github.com/modelcontextprotocol/go-sdk@v1.3.1`
    - Run `go mod tidy`
  - **Acceptance:** `go.mod` lists `github.com/modelcontextprotocol/go-sdk v1.3.1`. `go build ./...` succeeds.
