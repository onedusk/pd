# Changelog

All notable changes to the progressive-decomposition project.

## [Unreleased]

### Fixed
- **MCP server now exposes all tools on a single stdio connection** — previously, `--serve-mcp` only registered 3 decompose tools (`run_stage`, `get_status`, `list_decompositions`). The 5 code intelligence tools (`build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`) were defined but never wired into the stdio server. Now all tools are available through a unified MCP server.
- **`--serve-mcp` capability upgraded from `CapBasic` to `CapMCPOnly`** — the MCP server was hardcoded to `CapBasic`, causing `run_stage` to produce `<!-- TODO -->` stubs even when running as an MCP server. Changed to `CapMCPOnly` so the server operates at its correct capability level.

### Added
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
