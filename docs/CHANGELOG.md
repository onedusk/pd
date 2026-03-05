# Changelog

All notable changes to the progressive-decomposition project.

## [Unreleased]

### Added
- **Review phase** (`internal/review/`) — two-pass verification system between Stage 4 completion and implementation start. Go binary runs 5 mechanical checks (producer), then delegates an interpretive triage pass to a separate agent session via A2A (consumer).
- **5 mechanical review checks:**
  - **Check 1: File existence** (`check_existence.go`) — `os.Stat` validation. CREATE+exists→MISMATCH, MODIFY+missing→MISMATCH, DELETE+missing→STALE.
  - **Check 2: Symbol verification** (`check_symbols.go`) — validates symbol refs from task outlines against codebase via graph `QuerySymbols` or file-read fallback. Relocated→STALE, missing→MISMATCH.
  - **Check 3: Dependency completeness** (`check_deps.go`) — finds dependents of MODIFY targets via `GetDependencies(DirectionUpstream)` or import-regex filesystem walk. Unlisted dependent→OMISSION. Direction semantics gated by `TestDirectionSemantics`.
  - **Check 4: Cross-milestone consistency** (`check_crossms.go`) — detects conflicting actions (MODIFY-before-CREATE, CREATE+DELETE), overlapping symbol modifications across milestones. Uses `orchestrator.ParseMilestones` for DAG ordering. Semantic conflicts deferred to A2A interpretive pass.
  - **Check 5: Coverage gap scan** (`check_coverage.go`) — scope-boundary filesystem walk + graph clusters/impact analysis. High recall, moderate precision — interpretive session triages false positives.
- **Stage 3 directory tree parser** (`parse_stage3.go`) — handles both box-drawing characters (`├──`, `└──`, `│`) and plain indentation. Extracts file paths + `CREATE/MODIFY/DELETE (milestone)` annotations. Multiple actions per line supported.
- **Stage 4 task spec parser** (`parse_stage4.go`) — parses `tasks_mNN.md` files. Extracts task IDs (`T-MM.SS`), file+action, depends-on, symbol refs from outlines (backtick identifiers + PascalCase names).
- **Review report formatter** (`report.go`) — markdown output with summary table, per-check findings sections (`R-N.NN [CLASS] \`path\`: description`), and `<!-- INTERPRETIVE PASS NEEDED -->` stub for A2A consumer. `ParseMismatchCount` for implement warning.
- **`decompose review <name>`** CLI command — verifies stages 1-4 complete, opens graph store if available, runs all 5 checks, writes `review-findings.md`, prints summary to stderr.
- **`decompose review-interpret <name>`** CLI command — discovers A2A agents with `review-interpret` skill, submits interpretive triage task with findings + context artifacts. Graceful degradation: A2A available→submit task, not available→print instructions for manual delegation.
- **A2A review-interpret client** (`a2a_interpret.go`) — `SubmitInterpretTask` discovers agents, validates skill, sends `SendMessage` with findings + Stage 1/3/4 files as message parts. `PollInterpretTask` polls until terminal state.
- **`run_review` MCP tool** — single tool running all 5 checks. Registered in unified server alongside decompose tools. Uses `CodeIntelService` graph when available via `SetCodeIntel` method.
- **Two-level implement warning** — `runImplement` checks review state: missing `review-findings.md`→warn; existing with mismatches→warn louder. `--skip-review` flag suppresses.
- **12 review tests** — `TestDirectionSemantics` (gate test), parser tests (tree-chars + plain indent), check tests (file existence, cross-milestone), A2A client tests (mock server, skill validation, error paths).

- **Verification agent + pipeline integration** — "fresh eyes" post-stage verification that catches gaps before they propagate. `VerificationReport` with severity levels (critical/warning/info), 50+ rule-based checks across all 5 stages plus cross-stage coherence rules. Critical findings block pipeline progression in `RouteRange`. Runs automatically after each stage; skip with `--skip-verification`.
- **Verification agent** (`internal/agent/verification.go`) — A2A-compatible agent with `verify-stage` and `verify-cross-stage` skills. Receives only stage output content (never producing agent's context) for "fresh eyes" isolation. Registered as `RoleVerification` in agent registry.
- **Milestone scheduler** (`internal/orchestrator/scheduler.go`) — dependency-aware DAG scheduler using Kahn's algorithm for cycle detection. Thread-safe with `sync.Mutex`. `Ready()`, `MarkRunning()`, `MarkCompleted()`, `MarkFailed()`, `AllCompleted()`. `ParseMilestones()` extracts dependency relationships from Stage 3 markdown.
- **Review gates** (`internal/orchestrator/review.go`) — `ReviewStrategy` interface with three implementations: `CLIReviewStrategy` (terminal prompt), `PRReviewStrategy` (`gh pr create` + poll), `FileReviewStrategy` (sentinel file polling). Select at runtime with `--review-mode cli|pr|file`.
- **Implementation pipeline** (`internal/orchestrator/implement.go`) — coordinates milestone fan-out to parallel Claude Code sessions respecting the Stage 3 dependency DAG. `ImplementerFunc` abstraction enables testing without spawning actual processes. Batch concurrency controlled by `--max-concurrent N`.
- **`decompose implement <name>`** CLI command — verifies decomposition completeness, parses milestones, builds scheduler, selects review strategy, runs implementation pipeline. Each milestone spawns a focused `claude --print` session with Stage 0 + Stage 2 + task spec context only.
- **Implementer agent** (`internal/agent/implementer.go`) — wraps Claude Code CLI as implementation adapter. Composes focused prompts, spawns `claude --print --output-format stream-json --max-turns 50`, collects changed files via `git diff`.
- **Eval framework design** (`docs/internal/eval-framework-design.md`) — design document for empirical evaluation of pd's methodology claims. Defines rubric-based scoring (ambiguity, completeness, file accuracy, clarification count), comparative evaluation, Stage 2 ablation study, and LLM-as-judge extension point.

### Changed
- **Go module path** — renamed from `github.com/dusk-indust/decompose` to `github.com/onedusk/pd` across all 46 files (68 import references) to match actual repository URL.

### Added (prior)
- **`--help` flag and usage output** — custom help with synopsis, subcommand table, stage descriptions, examples, and all flags. `--help` exits cleanly (code 0); bare `decompose` shows usage then errors.
- **`decompose status [name]`** CLI command — shows stage completion for one or all decompositions. Shared `internal/status` package used by both CLI and MCP `get_status` tool.
- **`decompose export <name>`** CLI command — exports decomposition as structured JSON to stdout. Parses Stage 4 task markdown files to extract task IDs, titles, file actions, dependencies, and acceptance criteria. 73 tasks parsed from the agent-parallel decomposition.
- **`decompose diagram`** CLI command + `generate_diagram` MCP tool — produces Mermaid `graph TD` diagrams from the persisted code graph. Clusters become subgraphs, IMPORTS edges become arrows.
- **Config file support (`decompose.yml`)** — project-level YAML config with `outputDir`, `languages`, `excludeDirs`, `templatePath`, `verbose`, `singleAgent`, `graphExcludes`. CLI flags override config values. Uses `gopkg.in/yaml.v3` (already an indirect dep).
- **Template customization** — `GetStageContext` checks `.decompose/templates/stage-{N}-{name}.md` before falling back to embedded templates. Per-project template overrides without modifying the binary.
- **Graph build progress indicators** — stderr output during `build_graph`: file scan count, parsing progress (every 100 files), resolved edge count, clustering status.
- **MCP server startup/shutdown messages** — `decompose MCP server v{version} starting on stdio (project: {path})` on stderr at startup; shutdown message on exit.
- **TypeScript wildcard export resolution** — `package.json` exports with `*` patterns (e.g., `"./components/*": "./src/components/*.tsx"`) now resolve correctly. Array export values use first-match semantics.
- **3 new resolver tests** — workspace wildcard, conditional exports, and array export value parsing.
- **`internal/export` package** — `json.go` (structured decomposition export with task parsing), `mermaid.go` (graph-to-Mermaid generation).
- **`internal/config` package** — YAML config loading with `Load(dir)`.
- **`internal/status` package** — shared stage scanning extracted from MCP handlers.

### Fixed
- **Better error messages** — capability detection failure now explains what was probed and suggests `--agents`. Init checks for `jq` on PATH and warns if missing. Verbose mode shows human-readable capability descriptions.

### Changed
- **`scanCompletedStages` and `nextStage` extracted** from `internal/mcptools/decompose_handlers.go` to `internal/status/status.go` — eliminates duplication between MCP and CLI code paths.

### Added (import-resolution)
- **Import path resolution** (`internal/graph/resolve.go`) — new `Resolver` type rewrites raw import specifiers to repo-relative file paths during `build_graph`. Supports TypeScript (relative + workspace packages via `package.json` exports), Go (local module imports via `go.mod`), Python (relative dot imports), and Rust (`crate::`/`self::`/`super::` paths). External packages silently skipped.
- **Two-pass BuildGraph** — `build_graph` now collects all parse results first, then stores files, builds the resolver, and stores symbols + resolved edges. Ensures all File nodes exist before IMPORTS edges reference them.
- **`GetAllEdges` on Store interface** — new method to enumerate all edges; used by clustering (O(E) adjacency build) and `persistGraph` (edge copying to file store).
- **TypeScript monorepo test fixture** (`testdata/fixtures/ts_monorepo/`) — minimal workspace with `@test/logger` and `@test/db` packages for resolver unit tests.
- **19 resolver unit tests** (`internal/graph/resolve_test.go`) — covers relative imports, workspace resolution, parent traversal, index files, external package skipping, and edge passthrough.

### Fixed
- **IMPORTS edges now persist to disk** — previously all IMPORTS edges had raw specifiers (`./service`, `@dusk/db/queries`) that didn't match any File node in KuzuDB, so they silently failed. With import resolution, 4,161 IMPORTS edges now persist for the Dusk project (2,096 files).
- **KuzuStore upstream query direction** — `fileNeighbors` upstream Cypher was equivalent to downstream (both found what a file imports). Fixed to correctly find files that import a given file.
- **augment.go direction semantics** — swapped `DirectionDownstream`/`DirectionUpstream` usage to match store semantics: downstream = dependencies (what this file imports), upstream = dependents (who imports this file).
- **`longestCommonPrefix` infinite loop** — when prefix was `"a/"` and didn't match, `LastIndex("/")` found the trailing slash and `[:idx+1]` produced the same string forever. Fixed with `TrimRight(prefix, "/")` before `LastIndex`.
- **Clustering O(N*E) performance** — `buildAdjacency` called `GetDependencies` per file (2,096 calls, each scanning 58K edges). Replaced with single `GetAllEdges` pass (O(E)).
- **`persistGraph` aborts on duplicate cluster name** — changed from `return error` to `continue` so persistence completes even with duplicate cluster names.
- **Duplicate hook registration** — removed hooks from SKILL.md frontmatter; `.claude/settings.json` is now the single source for hook config.
- **Hook stderr causing "hook error"** — added `exec 2>/dev/null` at top of `decompose-tool-guard.sh` to silence all stderr from jq, timeout, and KuzuDB.
- **MCP server now exposes all tools on a single stdio connection** — previously, `--serve-mcp` only registered 3 decompose tools (`run_stage`, `get_status`, `list_decompositions`). The 5 code intelligence tools (`build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`) were defined but never wired into the stdio server. Now all tools are available through a unified MCP server.
- **`--serve-mcp` capability upgraded from `CapBasic` to `CapMCPOnly`** — the MCP server was hardcoded to `CapBasic`, causing `run_stage` to produce `<!-- TODO -->` stubs even when running as an MCP server. Changed to `CapMCPOnly` so the server operates at its correct capability level.

### Added (prior)
- **`write_stage` MCP tool** — Claude generates section content, the binary validates (coherence checking + merge ordering) and writes the output file. Closes the loop where Claude had no way to write content _through_ the binary.
- **`get_stage_context` MCP tool** — returns the template, section names, and prerequisite stage content for a stage. Claude calls this once before generating sections instead of manually reading template files.
- **`set_input` MCP tool** — stores a high-level input file or content for a decomposition. Content is included in `get_stage_context` output for Stage 1.
- **`--input` CLI flag** — `decompose --input path/to/idea.md my-project` seeds the pipeline with a high-level description file.
- **`InputFile` / `InputContent` fields on `orchestrator.Config`** — allows the pipeline to carry a seed document.
- **Unified MCP server** (`internal/mcptools/unified_server.go`) — single server factory that registers all 11 tools: 3 decompose + 3 hybrid + 5 code intelligence.

### Changed
- **SKILL.md rewritten to be binary-first** — MCP tools are now the primary workflow with imperative language ("you MUST use MCP tools when available"). Manual file operations demoted to a clearly labeled fallback. Stage-specific code intelligence instructions added (e.g., Stage 3 uses `get_clusters` for milestone boundaries, `assess_impact` for dependency ordering).
- **`MergePlanForStage` exported** from `internal/orchestrator` — was `mergePlanForStage` (unexported). Needed by the new `write_stage` handler to validate and merge Claude-generated sections.

## [0.1.0] — 2026-02-26

Initial release with full Go implementation.

### Milestones delivered
- **M1**: Project scaffolding, A2A types, graph schema, orchestrator interfaces
- **M2**: Code intelligence layer — tree-sitter parser (Go, TypeScript, Python, Rust), MemStore, KuzuDB store, file clustering
- **M3**: MCP tool servers — 5 code intelligence tools over MCP protocol
- **M4**: A2A agent framework — HTTP client/server, SSE streaming, task store, base agent
- **M5**: Specialist agents — research, schema, planning, task-writer + registry
- **M6**: Orchestrator pipeline — router, fan-out, merge, coherence checking, progress reporting, CLI wiring
- **M7**: Graceful degradation — capability detection (4 levels), fallback execution, MCP server mode
- **M8**: Testing + release — E2E tests, golden files, goreleaser, CI, Makefile
- **`decompose init`** command — installs skill files + `.mcp.json` into target projects

### Methodology (pre-Go)
- 5-stage pipeline: dev standards → design pack → implementation skeletons → task index → task specifications
- `/decompose` Claude Code skill with argument routing, templates, and process guide
- Named decompositions with kebab-case directories under `docs/decompose/`
