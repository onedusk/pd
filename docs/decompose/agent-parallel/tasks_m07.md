# Stage 4: Task Specifications — Milestone 7: Graceful Degradation + Skill Integration

> Implements four-level capability detection (ADR-004), fallback code paths for each level, and upgrades the `/decompose` skill to detect and delegate to the Go binary.
>
> Fulfills: ADR-004 (graceful degradation), PDR-003 (/decompose skill remains primary UX), PDR-004 (three-layer entry point)

---

- [ ] **T-07.01 — Implement capability detector**
  - **File:** `internal/orchestrator/detector_impl.go` (CREATE)
  - **Depends on:** T-01.08, T-04.01
  - **Outline:**
    - Define `DefaultDetector` struct implementing `Detector` interface
    - `Detect(ctx context.Context) (CapabilityLevel, []string, error)`:
      1. **Probe for A2A agents:** Iterate over well-known local ports (9100–9110, configurable). For each port, call `client.DiscoverAgent(ctx, fmt.Sprintf("http://localhost:%d", port))`. Collect discovered agent endpoints. Timeout: 500ms per probe.
      2. **Probe for MCP tools:** Check if code intelligence MCP server is reachable (attempt `tools/list`). Check if external MCP tools (web search, etc.) are available.
      3. **Probe for code intelligence:** Check if Tree-sitter and KuzuDB are available (attempt to create a `TreeSitterParser` and `KuzuStore` — they require CGO, which may not be available).
      4. **Determine level:**
         - A2A agents found + MCP tools + code intelligence → `CapFull`
         - A2A agents found + MCP tools, no code intelligence → `CapA2AMCP`
         - MCP tools only (no A2A agents) → `CapMCPOnly`
         - Nothing available → `CapBasic`
      5. Return level + list of discovered agent endpoint URLs
    - Handle gracefully: each probe catches panics (CGO can panic on missing libraries). Log probe results at debug level.
    - Support `--single-agent` flag: if set, skip all probes and return `CapBasic`
  - **Acceptance:** Detector returns `CapBasic` when no agents/tools are available. Returns `CapFull` when mock agents are running. `--single-agent` override works. CGO unavailability (build tag `!cgo`) doesn't panic — returns `CapMCPOnly` or `CapBasic`.

---

- [ ] **T-07.02 — Write capability detector tests**
  - **File:** `internal/orchestrator/detector_test.go` (CREATE)
  - **Depends on:** T-07.01
  - **Outline:**
    - Test: no services running → `CapBasic`, empty agent list
    - Test: start 2 mock A2A agents on known ports → `CapA2AMCP` or `CapFull` (depending on MCP/code intel availability), agent endpoints returned
    - Test: `SingleAgent: true` in config → always `CapBasic` regardless of available services
    - Test: agent discovery timeout (mock agent that delays 5s, detector timeout 500ms) → agent not discovered, no hang
    - Test: one agent reachable, one not → only reachable one in list
    - Use `httptest` for mock agents
  - **Acceptance:** `go test ./internal/orchestrator/ -run TestDetector` passes. Detection is correct for all 4 capability levels. Timeouts prevent hanging.

---

- [ ] **T-07.03 — Implement fallback execution paths**
  - **File:** `internal/orchestrator/fallback.go` (CREATE)
  - **Depends on:** T-06.01, T-01.08
  - **Outline:**
    - Define `FallbackExecutor` struct implementing `StageExecutor` for each capability level
    - `CapBasic` fallback:
      - No agents, no MCP tools. The pipeline writes template files with TODO markers for sections that would normally be produced by agents. User fills them in manually or via Claude Code's native capabilities.
      - For each stage, copy the stage template to the output directory with section headers intact and placeholder content.
    - `CapMCPOnly` fallback:
      - Single agent with MCP tools. Executes each stage section sequentially using MCP tools directly (no A2A fan-out).
      - `BuildGraph` is called directly, `QuerySymbols`/`GetDependencies` are available for planning.
      - No parallelism — sections produced one at a time.
    - `CapA2AMCP` fallback:
      - Full parallel pipeline but code intelligence graph is built with file-reading heuristics instead of Tree-sitter + KuzuDB.
      - Planning Agent falls back to listing files and inferring dependencies from file content.
    - `CapFull`:
      - Not a fallback — this is the primary path (implemented in Pipeline, M6).
    - Each fallback path produces valid stage output files in the same format as the full pipeline.
  - **Acceptance:** Each capability level produces output files in `docs/decompose/<name>/`. `CapBasic` output contains section headers with TODO placeholders. `CapMCPOnly` output has real content for sections backed by MCP tools. All output files are valid markdown.

---

- [ ] **T-07.04 — Write fallback execution tests**
  - **File:** `internal/orchestrator/fallback_test.go` (CREATE)
  - **Depends on:** T-07.03
  - **Outline:**
    - Test `CapBasic` fallback: run Stage 1 → output file exists with all section headers, TODO markers present
    - Test `CapMCPOnly` fallback: run Stage 1 with mock MCP tools → output file has real content in tool-backed sections
    - Test `CapA2AMCP` fallback: run Stage 3 without code intelligence → output has milestone list derived from heuristics
    - Test: fallback output is valid markdown (no unclosed code blocks, headers are properly formatted)
    - Test: all 4 levels produce files that pass the stage verification checklists (section headers present)
    - Use temp directories for output
  - **Acceptance:** `go test ./internal/orchestrator/ -run TestFallback` passes. Each capability level produces valid, parseable output files.

---

- [ ] **T-07.05 — Wire detector into pipeline**
  - **File:** `internal/orchestrator/pipeline.go` (MODIFY), `cmd/decompose/main.go` (MODIFY)
  - **Depends on:** T-07.01, T-07.03, T-06.10
  - **Outline:**
    - `pipeline.go`: At the start of `RunStage`, if `cfg.Capability` is not set, call `Detector.Detect()` to determine it. Cache the result for subsequent stages in the same run.
    - `pipeline.go`: In `RunStage`, switch on `cfg.Capability`:
      - `CapFull`: existing parallel pipeline from T-06.10
      - `CapA2AMCP`: parallel pipeline with fallback Planning Agent
      - `CapMCPOnly`: `FallbackExecutor` with MCP tools
      - `CapBasic`: `FallbackExecutor` template mode
    - `main.go`: After parsing flags, create `DefaultDetector`, run detection, log detected level, pass to `NewPipeline`
    - Print detected capability level to stderr: `"Detected capability: {level}"`
  - **Acceptance:** `decompose --single-agent myproject 1` uses `CapBasic` path. `decompose myproject 1` with no agents available auto-detects `CapBasic`. Detection result is logged. Pipeline routes to correct executor for each level.

---

- [ ] **T-07.06 — Update /decompose skill for binary delegation**
  - **File:** `skill/decompose/SKILL.md` (MODIFY)
  - **Depends on:** T-07.05
  - **Outline:**
    - Add a section to the skill's General Workflow (before step 1):
      - Check if `decompose` binary is in `PATH`: run `which decompose` or `command -v decompose`
      - If found: delegate to the binary by running `decompose --project-root $(pwd) --output-dir docs/decompose/<name> <name> <stage>` and stream its output
      - If not found: continue with current single-agent behavior (no change to existing workflow)
    - Add a note in the skill header explaining the binary delegation
    - Update the `!` shell preprocessing commands to include binary detection
    - Do NOT change any existing skill behavior — only add the delegation check at the top
  - **Acceptance:** When the binary is in PATH, the skill delegates to it. When the binary is not in PATH, the skill works exactly as before (current single-agent behavior). No existing functionality is broken.

---

- [ ] **T-07.07 — Write integration tests for all degradation levels**
  - **File:** `internal/orchestrator/degradation_test.go` (CREATE)
  - **Depends on:** T-07.05
  - **Outline:**
    - Integration test that runs Stage 1 at each capability level and validates output:
    - Test `CapBasic`: no agents, no MCP → template with TODOs, all section headers present
    - Test `CapMCPOnly`: mock MCP server available → real content in tool-backed sections
    - Test `CapA2AMCP`: mock A2A agents + mock MCP, no code intelligence → parallel execution, heuristic-based planning
    - Test `CapFull`: mock A2A agents + mock MCP + mock code intelligence → full parallel pipeline
    - Each test verifies:
      - Output file exists at expected path
      - Output contains all required section headers for the stage
      - Output is valid markdown
      - No panics or goroutine leaks
    - Use temp directories, `httptest` for mock agents/MCP
  - **Acceptance:** `go test ./internal/orchestrator/ -run TestDegradation` passes. All 4 levels produce valid output. Tests clean up temp directories.

---

- [ ] **T-07.08 — Implement MCP server mode for the decompose binary**
  - **File:** `internal/mcptools/decompose_server.go` (CREATE), `internal/mcptools/decompose_handlers.go` (CREATE)
  - **Depends on:** T-03.03, T-06.10
  - **Outline:**
    - `decompose_server.go`: Define `NewDecomposeMCPServer(pipeline orchestrator.Orchestrator, cfg orchestrator.Config) *mcp.Server`
      - Create MCP server: `mcp.NewServer(&mcp.Implementation{Name: "decompose", Version: version}, opts)`
      - Register 3 tools: `run_stage`, `get_status`, `list_decompositions`
    - `decompose_handlers.go`: Implement handlers matching `mcp.ToolHandlerFor[In, Out]`:
      - `RunStage(ctx, req, input RunStageInput) (*mcp.CallToolResult, RunStageOutput, error)` — call `pipeline.RunStage(ctx, Stage(input.Stage))`, return files written
      - `GetStatus(ctx, req, input GetStatusInput) (*mcp.CallToolResult, GetStatusOutput, error)` — scan `docs/decompose/<name>/` for existing stage files, determine completed stages and next stage
      - `ListDecompositions(ctx, req, input ListDecompositionsInput) (*mcp.CallToolResult, ListDecompositionsOutput, error)` — scan `docs/decompose/` for subdirectories, check each for stage completion
    - These tools are for the `/decompose` skill to call — they are NOT the code intelligence tools (those are in M3)
  - **Acceptance:** MCP server registers 3 tools. `run_stage` triggers the pipeline and returns file paths. `get_status` correctly reports which stages are complete. `list_decompositions` finds existing decompositions.

---

- [ ] **T-07.09 — Wire --serve-mcp flag in CLI**
  - **File:** `cmd/decompose/main.go` (MODIFY)
  - **Depends on:** T-07.08, T-01.02
  - **Outline:**
    - Add `--serve-mcp` flag to `cliFlags` (already in Stage 2 skeleton)
    - In `run()`: if `flags.ServeMCP` is true, create `DecomposeMCPServer`, run it on stdio transport (`mcp.StdioTransport{}`), block until stdin closes
    - The MCP server mode is mutually exclusive with normal CLI mode — if `--serve-mcp` is set, ignore name/stage arguments
    - Document in `--help` output: "Run as MCP server for Claude Code integration"
  - **Acceptance:** `decompose --serve-mcp` starts and waits for MCP protocol messages on stdin/stdout. Sending a `tools/list` JSON-RPC request returns 3 tools. Ctrl-C or stdin close exits cleanly.

---

- [ ] **T-07.10 — Write MCP server mode tests**
  - **File:** `internal/mcptools/decompose_server_test.go` (CREATE)
  - **Depends on:** T-07.08
  - **Outline:**
    - Use `mcp.NewInMemoryTransports()` for testing
    - Test `list_decompositions` with empty `docs/decompose/` → empty list, `hasStage0: false`
    - Test `list_decompositions` with one decomposition directory containing stage-1 and stage-2 files → returns summary with `completedStages: [1, 2]`, `nextStage: 3`
    - Test `get_status` for a known decomposition → correct completed stages
    - Test `run_stage` with a mock pipeline → returns files written
    - Test `run_stage` with invalid stage number → error
    - Use temp directories for test decomposition files
  - **Acceptance:** `go test ./internal/mcptools/ -run TestDecomposeServer` passes. All 3 tools tested with happy path and edge cases.
