# Stage 1: Design Pack — Agent-Parallel Decomposition

> Evolve the progressive decomposition pipeline from a single-agent sequential process into a
> multi-agent parallel system. Specialist agents coordinate via A2A, access tools via MCP, and
> use code intelligence (Tree-sitter + graph DB) for structural analysis.
>
> This design pack formalizes and extends `docs/internal/agent-parallel-design.md`.

---

## Assumptions & Constraints *(required)*

1. **Evolution, not replacement.** The agent-parallel system is an upgrade path for the existing `/decompose` skill. Users without A2A infrastructure get current single-agent behavior unchanged.
2. **Local-only for v1.** Everything runs on the user's machine. No cloud infrastructure, no remote services, no networking configuration. Agents are local goroutines or child processes.
3. **Go is the implementation language.** Single-binary distribution, cgo required for Tree-sitter and graph DB. See ADR-005.
4. **A2A is the coordination protocol.** Google's Agent-to-Agent Protocol (v0.4.0, Linux Foundation) — HTTP + JSON-RPC 2.0 with SSE streaming. No proprietary coordination layer.
5. **MCP is the tool protocol.** Model Context Protocol (Go SDK v1.3.1, stable post-1.0). Each specialist agent gets capabilities through MCP tools.
6. **Graceful degradation is mandatory.** Four levels: full parallel → parallel without code graph → single agent with MCP tools → current skill behavior. No hard dependencies on any external system.
7. **Code intelligence is self-contained and ephemeral.** The system builds a temporary knowledge graph per decomposition using Tree-sitter + embedded graph DB. Built fresh, discarded after use.

---

## Target Platform & Tooling Baseline *(required)*

| Component | Version | Reference |
|-----------|---------|-----------|
| Language | Go 1.26.0 | [go.dev](https://go.dev/doc/) |
| MCP SDK | `github.com/modelcontextprotocol/go-sdk` v1.3.1 | [GitHub](https://github.com/modelcontextprotocol/go-sdk) |
| A2A Protocol | v0.4.0 (spec only — no official Go SDK) | [GitHub](https://github.com/google/A2A), [Spec](https://google.github.io/A2A/) |
| Tree-sitter (Go bindings) | `github.com/tree-sitter/go-tree-sitter` v0.25.0 | [GitHub](https://github.com/tree-sitter/go-tree-sitter) |
| Graph DB | `github.com/kuzudb/go-kuzu` v0.11.3 (archived Oct 2025) | [GitHub](https://github.com/kuzudb/go-kuzu) |
| Build / Release | goreleaser | [goreleaser.com](https://goreleaser.com/intro/) |
| Testing | `github.com/stretchr/testify` | [GitHub](https://github.com/stretchr/testify) |
| Linting | golangci-lint | [GitHub](https://github.com/golangci/golangci-lint) |

### Version Notes

- **MCP Go SDK** is post-1.0 with stability guarantees. Supports MCP spec versions 2024-11-05 through 2025-11-25. Key packages: `mcp` (primary API), `jsonrpc`, `auth`. Transports: stdio, StreamableHTTP, SSE, in-memory.
- **A2A** has no official Go SDK. Implementation is against the spec directly — HTTP + JSON-RPC 2.0 is well-supported by Go's stdlib. The protobuf spec is at `specification/a2a.proto` in the A2A repo.
- **KuzuDB** was archived Oct 2025. v0.11.3 is functional and bundles all needed extensions. Since the graph is ephemeral (built and discarded per decomposition), the archived status is acceptable for v1. If KuzuDB becomes untenable, the Bighorn fork or an in-memory adjacency list with a query layer are fallback options. See ADR-006.
- **Tree-sitter Go bindings** are the official bindings from the tree-sitter org (not the community `smacker/go-tree-sitter`). Requires CGO. Manual `Close()` calls required on Parser, Tree, TreeCursor, Query, QueryCursor to avoid C memory leaks.

---

## Data Model / Schema *(required)*

### Entity: AgentCard

**Purpose:** Self-describing manifest for each specialist agent, following A2A spec.
**Key:** `name` (string, unique)

**Fields:**

| Field | Type | Nullable | Purpose |
|-------|------|:--------:|---------|
| name | string | No | Human-readable agent name |
| description | string | No | Agent purpose and capabilities |
| version | string | No | Agent version (semver) |
| url | string | No | Local HTTP endpoint |
| skills | []AgentSkill | No | Declared capabilities |
| capabilities | AgentCapabilities | No | Feature flags (streaming, push) |
| default_input_modes | []string | No | Accepted MIME types |
| default_output_modes | []string | No | Produced MIME types |

### Entity: DecompositionTask

**Purpose:** Tracks the state of an A2A task within a decomposition.
**Key:** `id` (UUID, unique)

**Fields:**

| Field | Type | Nullable | Purpose |
|-------|------|:--------:|---------|
| id | UUID | No | Unique task identifier |
| context_id | string | No | Decomposition name (groups related tasks) |
| stage | int | No | Pipeline stage (0–4) |
| section | string | Yes | Stage section being worked (e.g., "platform-baseline") |
| state | TaskState | No | SUBMITTED / WORKING / COMPLETED / FAILED / INPUT_REQUIRED |
| assigned_agent | string | Yes | Agent name handling this task |
| artifacts | []Artifact | No | Output artifacts produced |
| created_at | time.Time | No | Task creation timestamp |
| updated_at | time.Time | No | Last state change |

**Relationships:**

| Relationship | Target | Cardinality | Delete Rule |
|-------------|--------|-------------|-------------|
| parent_task | DecompositionTask | many-to-one | Cascade |
| reference_tasks | DecompositionTask | many-to-many | None |

### Entity: Artifact

**Purpose:** Output content produced by an agent for a task.
**Key:** `artifact_id` (UUID, unique within task)

**Fields:**

| Field | Type | Nullable | Purpose |
|-------|------|:--------:|---------|
| artifact_id | UUID | No | Unique identifier within task |
| name | string | No | Human-readable name (e.g., "platform-baseline") |
| description | string | Yes | What this artifact contains |
| parts | []Part | No | Content parts (text, data, raw) |
| metadata | map[string]any | Yes | Custom metadata (e.g., token_count) |

### Entity: GraphNode

**Purpose:** A node in the code intelligence knowledge graph.
**Key:** Composite (`kind` + `id`)

**Fields:**

| Field | Type | Nullable | Purpose |
|-------|------|:--------:|---------|
| id | string | No | Unique identifier (path for File, qualified name for Symbol) |
| kind | NodeKind | No | FILE / SYMBOL / CLUSTER |
| path | string | Yes | File path (File nodes only) |
| language | string | Yes | Programming language (File nodes only) |
| loc | int | Yes | Lines of code (File nodes only) |
| name | string | Yes | Symbol/cluster name |
| symbol_kind | SymbolKind | Yes | FUNCTION / CLASS / TYPE / ENUM / INTERFACE (Symbol nodes only) |
| exported | bool | Yes | Whether symbol is exported (Symbol nodes only) |
| cohesion_score | float64 | Yes | Cluster cohesion metric (Cluster nodes only) |

### Entity: GraphEdge

**Purpose:** A relationship in the code intelligence knowledge graph.

**Fields:**

| Field | Type | Nullable | Purpose |
|-------|------|:--------:|---------|
| source_id | string | No | Source node ID |
| target_id | string | No | Target node ID |
| kind | EdgeKind | No | DEFINES / IMPORTS / CALLS / INHERITS / BELONGS |

**Relationships:**

| Relationship | Target | Cardinality | Delete Rule |
|-------------|--------|-------------|-------------|
| source | GraphNode | many-to-one | Cascade |
| target | GraphNode | many-to-one | Cascade |

### Entity: DecompositionConfig

**Purpose:** Runtime configuration for a decomposition run.
**Key:** `name` (string, unique per run)

**Fields:**

| Field | Type | Nullable | Purpose |
|-------|------|:--------:|---------|
| name | string | No | Decomposition name (kebab-case) |
| project_root | string | No | Absolute path to target project |
| output_dir | string | No | Path to `docs/decompose/<name>/` |
| capability_level | CapabilityLevel | No | Detected: FULL / A2A_MCP / MCP_ONLY / BASIC |
| agents_available | []string | No | Names of discovered specialist agents |
| stage_0_path | string | Yes | Path to shared development standards |

---

## Architecture *(required)*

### Component Diagram

```
┌─────────────────────────────────────────────────────────┐
│                    CLI Entry Point                        │
│                  (cmd/decompose)                          │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                  Orchestrator                             │
│                                                          │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ Capability   │  │ Stage        │  │ Merge         │  │
│  │ Detector     │  │ Router       │  │ Engine        │  │
│  └─────────────┘  └──────────────┘  └───────────────┘  │
│                                                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │         A2A Client (task management)             │    │
│  └─────────────────────────────────────────────────┘    │
└────────┬──────────┬──────────┬──────────┬───────────────┘
         │          │          │          │
    ┌────▼───┐ ┌────▼───┐ ┌───▼────┐ ┌───▼─────┐
    │Research│ │Schema  │ │Planning│ │Task     │
    │Agent   │ │Agent   │ │Agent   │ │Writer   │
    │        │ │        │ │        │ │Agent(s) │
    └───┬────┘ └───┬────┘ └───┬────┘ └───┬─────┘
        │          │          │          │
    ┌───▼────┐ ┌───▼────┐ ┌───▼────┐ ┌───▼─────┐
    │MCP     │ │MCP     │ │MCP     │ │MCP      │
    │Tools   │ │Tools   │ │Tools   │ │Tools    │
    │(web,   │ │(lang   │ │(graph, │ │(search, │
    │ docs)  │ │ server)│ │ deps)  │ │ fs)     │
    └────────┘ └────────┘ └────────┘ └─────────┘
```

### Architectural Pattern

**Orchestrator pattern with protocol-based agent coordination.**

The orchestrator is the single decision-maker. It does not perform domain work — it delegates to specialist agents via A2A tasks, merges their artifacts, and writes final output. Each specialist agent is an independent A2A server with its own MCP tools.

This pattern was chosen over:
- **Peer-to-peer agents** — requires complex consensus; orchestrator is simpler and matches the sequential pipeline
- **Event-driven choreography** — harder to reason about; the pipeline has clear sequential dependencies
- **Monolithic agent with tools** — the current system; doesn't parallelize

### Concurrency / Threading Model

```
main goroutine
  └── Orchestrator
        ├── Capability detection (sync)
        ├── A2A agent discovery (sync — local HTTP probes)
        ├── Stage execution:
        │     ├── Fan-out: N goroutines, one per specialist task
        │     │     ├── goroutine 1: A2A SendMessage → agent A
        │     │     ├── goroutine 2: A2A SendMessage → agent B
        │     │     └── goroutine N: A2A SendMessage → agent N
        │     ├── Wait: sync.WaitGroup or errgroup
        │     └── Merge: single goroutine collects artifacts
        └── Output: write files (sync)

Each specialist agent (local process):
  └── HTTP server (net/http)
        ├── JSON-RPC handler (A2A)
        ├── MCP client connections (to tool servers)
        └── Work goroutines (per-task)
```

Key concurrency decisions:
- **errgroup** for fan-out with cancellation on first error
- **Channels** for artifact streaming from specialists to orchestrator
- **Context propagation** for timeout and cancellation across A2A boundaries
- Parsers (Tree-sitter) are **not thread-safe** — one parser per goroutine

---

## UI/UX Layout *(skip — CLI tool)*

This is a CLI tool invoked via `/decompose` or a standalone binary. No UI/UX layout applies.

**CLI interface:**

```
decompose [flags] [name] [stage|command]

Flags:
  --project-root    Path to target project (default: cwd)
  --output-dir      Override output directory
  --agents          Comma-separated agent endpoints (default: auto-discover)
  --single-agent    Force single-agent mode
  --verbose         Show agent-level progress
  --serve-mcp       Run as MCP server (stdio transport, for Claude Code integration)
  --version         Print version and exit
```

**MCP server mode** (`--serve-mcp`): The binary runs as an MCP server on stdio, exposing decomposition operations as MCP tools. Users configure it in their project's `.mcp.json`:

```json
{
  "mcpServers": {
    "decompose": {
      "command": "decompose",
      "args": ["--serve-mcp"]
    }
  }
}
```

MCP tools exposed:
- `run_stage` — execute a pipeline stage (input: name, stage, project_root)
- `get_status` — report progress for a decomposition (input: name)
- `list_decompositions` — list all decompositions in the project

The `/decompose` skill detects these tools and calls them directly instead of shelling out.

**Progress output:** Milestone-level callbacks. User sees checkpoint updates:
```
[agent-parallel] Stage 1: Design Pack
  ✓ Platform research complete
  ✓ Codebase exploration complete
  ● Writing data model...
  ○ Architecture (pending)
  ○ ADRs (pending)
```

---

## Features *(required)*

### Core

- [ ] **Orchestrator engine** — parse `/decompose` arguments, detect capabilities, route to appropriate execution mode (parallel or single-agent)
- [ ] **A2A task management** — create, track, and merge tasks across specialist agents using A2A protocol
- [ ] **Agent discovery** — find available specialist agents via Agent Card well-known URIs on localhost
- [ ] **Specialist: Research Agent** — platform investigation, version verification, codebase exploration via MCP tools
- [ ] **Specialist: Schema Agent** — translate data models to compilable code, validate types, write interface contracts
- [ ] **Specialist: Planning Agent** — build code intelligence graph, analyze dependencies, plan milestones
- [ ] **Specialist: Task Writer Agent** — produce per-milestone task specification files with acceptance criteria
- [ ] **Code intelligence graph** — Tree-sitter parsing → symbol extraction → KuzuDB graph → MCP-queryable

### Retrieval / Discovery

- [ ] **Graph queries via MCP** — `build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`
- [ ] **Agent Card discovery** — automatic local agent enumeration at startup
- [ ] **Decomposition status** — report progress across all active decompositions via A2A `ListTasks`

### Quality of Life

- [ ] **Graceful degradation** — four-level capability detection with automatic fallback
- [ ] **Section-based merge** — deterministic merge of parallel agent outputs with coherence checking
- [ ] **Progress reporting** — milestone-level callbacks to the user during pipeline execution
- [ ] **Token tracking** — report token usage per agent via A2A artifact metadata (observability, no enforcement in v1)

### Integrations

- [ ] **`/decompose` skill integration** — the existing Claude Code skill delegates to the Go binary when available
- [ ] **MCP tool protocol** — all specialist capabilities exposed as MCP tools for reuse
- [ ] **A2A protocol compliance** — full v0.4.0 support (tasks, streaming, push notifications)

---

## Integration Points *(required)*

### A2A Protocol (Agent-to-Agent)

- **API surface:** JSON-RPC 2.0 over HTTP. Methods: `SendMessage`, `GetTask`, `ListTasks`, `CancelTask`, `SubscribeToTask`. SSE for streaming responses.
- **Auth / permissions:** None for v1 (local-only). Bearer token support available in the spec for v2.
- **Constraints:** No official Go SDK — implement against the proto spec directly. Task states: SUBMITTED → WORKING → COMPLETED/FAILED/INPUT_REQUIRED. Artifacts carry content as Parts (text, data, raw bytes). v0.4.0 adds `ListTasks` with pagination and filtering.

### MCP Protocol (Model Context Protocol)

- **API surface:** Go SDK v1.3.1 (`github.com/modelcontextprotocol/go-sdk/mcp`). `mcp.NewServer()` + `mcp.AddTool()` for tool registration. `mcp.NewClient()` + `session.CallTool()` for invocation. Transports: stdio for subprocess-based agents, StreamableHTTP for HTTP-based agents, in-memory for testing.
- **Auth / permissions:** None for v1 (local tools). OAuth 2.0 support available in the SDK for v2.
- **Constraints:** Typed tool handlers auto-generate JSON schemas from Go structs via `jsonschema` tags. Content types: TextContent, ImageContent, EmbeddedResource. Auto-pagination via iterators.

### Tree-sitter

- **API surface:** `github.com/tree-sitter/go-tree-sitter` v0.25.0. `parser.SetLanguage()` + `parser.Parse()` → `tree.RootNode()`. Query via S-expression patterns. TreeCursor for efficient AST traversal.
- **Auth / permissions:** None (local library).
- **Constraints:** Requires CGO. Manual `Close()` on Parser, Tree, TreeCursor, Query, QueryCursor — no finalizers. Parsers are not thread-safe (one per goroutine). Language grammars are separate Go modules: `tree-sitter-go`, `tree-sitter-typescript`, `tree-sitter-python`, `tree-sitter-rust`.

### KuzuDB

- **API surface:** `github.com/kuzudb/go-kuzu` v0.11.3. `kuzu.OpenInMemoryDatabase()` → `kuzu.OpenConnection()` → `conn.Query()` (Cypher). Prepared statements with `conn.Prepare()` + `conn.Execute()`.
- **Auth / permissions:** None (embedded database).
- **Constraints:** Requires CGO (`libkuzu.so` / `libkuzu.dylib` must be available). Project archived Oct 2025 — no future updates. Node tables require explicit `PRIMARY KEY`. Relationship tables defined with `FROM`/`TO` syntax. See ADR-006 for risk assessment.

---

## Security & Privacy Plan *(required)*

- **Data at rest:** The code intelligence graph is ephemeral (in-memory or temp directory, discarded after decomposition). Decomposition output files are plaintext markdown written to the user's project directory. No encryption at rest — the files are meant to be committed to version control.
- **Data in transit:** All agent communication is localhost HTTP in v1. No TLS required for local loopback. If remote agents are added in v2, TLS (or mTLS per A2A spec) becomes mandatory.
- **Permissions required:** File system read access to the target project (for code intelligence). File system write access to `docs/decompose/`. Network access to localhost ports (for local A2A agents). Optionally, network access for web search MCP tools.
- **System exposure:** The code graph indexes source code structure (symbols, dependencies) but does not persist or transmit source code content. MCP web search tools may send project-related queries to external search APIs — this is opt-in and configurable.
- **Optional hardening (v2):** Bearer token auth for A2A agents. MCP tool allowlisting (restrict which tools each agent can call). Sandboxed agent processes.

---

## Architecture Decision Records *(required)*

### ADR-001 — A2A over custom coordination protocol

- **Status:** Accepted
- **Context:** Need a way for specialist agents to communicate. Could build a custom protocol, use a message queue, or adopt an existing standard.
- **Decision:** Use A2A (Agent-to-Agent Protocol, v0.4.0, Linux Foundation).
- **Consequences:**
  - (+) Standards-based — any A2A-compatible agent can participate
  - (+) Framework-agnostic — agents can be built with different tools (ADK, LangGraph, CrewAI, plain code)
  - (+) Built-in support for streaming (SSE), async (push notifications), and task lifecycle
  - (+) Agent Cards provide discoverable capability declarations
  - (-) A2A is still pre-1.0 — breaking changes possible
  - (-) Requires HTTP server per agent (local processes, but still overhead)
  - (-) No official Go SDK — must implement against the spec directly

### ADR-002 — MCP for tool integration

- **Status:** Accepted
- **Context:** Specialist agents need access to external tools (web search, type checkers, graph databases). Could embed tools directly, use function calling, or use a tool protocol.
- **Decision:** Use MCP (Model Context Protocol) for all tool integration.
- **Consequences:**
  - (+) Official Go SDK (v1.3.1, post-1.0 stability guarantee) eliminates protocol implementation risk
  - (+) Already adopted by Claude Code, Cursor, Windsurf, OpenCode — ecosystem is mature
  - (+) Tools are reusable across agents (same MCP server can serve multiple agents)
  - (+) Clean separation between agent logic and tool capability
  - (+) Typed tool handlers auto-generate JSON schemas from Go structs
  - (-) Adds a layer of indirection vs. direct function calls
  - (-) MCP servers must be running for tools to work

### ADR-003 — Temporary graph over persistent index

- **Status:** Accepted
- **Context:** The code intelligence layer needs a graph database for dependency analysis. Could use a persistent index or build a temporary one per decomposition.
- **Decision:** Build a temporary, in-memory graph per decomposition. Discard after use.
- **Consequences:**
  - (+) No stale data — graph reflects codebase at moment of decomposition
  - (+) No infrastructure to maintain — no persistent DB process
  - (+) Scoped to relevant files — doesn't index the entire monorepo
  - (+) Fast for typical projects (< 10k files with Tree-sitter)
  - (-) Re-indexes if the same codebase is decomposed again (acceptable tradeoff)
  - (-) No historical comparison between decompositions

### ADR-004 — Graceful degradation to single-agent

- **Status:** Accepted
- **Context:** Not all users will have A2A infrastructure or MCP tools configured. The system must work without them.
- **Decision:** Implement a four-level capability detection layer. The CLI checks what's available and adjusts behavior accordingly.
- **Consequences:**
  - (+) Zero-config default — current skill works unchanged
  - (+) Progressive enhancement — each layer (MCP, A2A, code intelligence) adds capability
  - (+) No hard dependencies on any external system
  - (-) Must maintain multiple code paths (one per capability level)
  - (-) Testing matrix increases (4 degradation levels)

### ADR-005 — Go as implementation language

- **Status:** Accepted
- **Context:** Need a language that produces single-binary distributions, cross-compiles trivially, and has strong support for both A2A (HTTP + JSON-RPC) and MCP.
- **Decision:** Go.
- **Consequences:**
  - (+) Single binary distribution — `go install` or download from releases
  - (+) Cross-platform in one command — macOS, Linux, Windows
  - (+) Goroutines + channels are the natural concurrency model for agent orchestration
  - (+) Official MCP SDK (v1.3.1) eliminates protocol implementation risk
  - (+) Fast builds (seconds, not minutes)
  - (-) CGO required for KuzuDB and Tree-sitter — complicates cross-compilation (need C toolchain per target)
  - (-) No official A2A Go SDK — must implement against the spec directly
  - (-) Tree-sitter Go bindings require manual `Close()` calls — no finalizer safety

  **Alternatives rejected:**
  - **Rust:** First-class Tree-sitter bindings, no GC. But harder to write, slower builds, steeper contributor barrier. Workload is I/O-bound — Rust's performance advantage doesn't apply.
  - **TypeScript:** Tier 1 MCP SDK. But requires Node.js runtime — no single binary.
  - **Python:** Tier 1 MCP SDK. But requires Python runtime, slow startup. Unacceptable for CLI distribution.

### ADR-006 — KuzuDB despite archived status

- **Status:** Accepted
- **Context:** KuzuDB was archived Oct 2025. v0.11.3 is the final release. The design specifies KuzuDB for the code intelligence graph. Alternatives: Bighorn fork, FalkorDB, in-memory adjacency lists.
- **Decision:** Use KuzuDB v0.11.3 for v1. Isolate all graph DB access behind an interface so the backend can be swapped.
- **Consequences:**
  - (+) KuzuDB v0.11.3 is functional and bundles all needed extensions
  - (+) Embedded (no server process), Cypher query language, fast for local use
  - (+) Graph is ephemeral — we don't need long-term maintenance or updates
  - (+) Interface isolation means we can swap to Bighorn fork or another graph DB without touching consumer code
  - (-) No security patches or bug fixes — any KuzuDB bugs we hit are ours to work around
  - (-) CGO dependency on a C library from an archived project
  - (-) Bighorn fork viability is uncertain (community-maintained)

### ADR-007 — Official Tree-sitter Go bindings over community package

- **Status:** Accepted
- **Context:** Two Go packages for Tree-sitter exist: official (`github.com/tree-sitter/go-tree-sitter`) and community (`github.com/smacker/go-tree-sitter`).
- **Decision:** Use the official bindings.
- **Consequences:**
  - (+) Maintained by the Tree-sitter org — tracks upstream closely
  - (+) Modular grammar loading — each language is a separate Go module
  - (+) Closer API to the C/Rust reference (easier to reference Tree-sitter docs)
  - (-) Pre-v1 — API may change
  - (-) Grammars are not bundled — must be imported individually
  - (-) Query predicates must be evaluated manually (no built-in `eq?`, `match?`)

---

## Product Decision Records *(required)*

### PDR-001 — Milestone-level progress, not agent-level streaming

- **Status:** Accepted
- **Problem:** Users running a multi-agent decomposition need feedback on progress. How granular should it be?
- **Decision:** Report milestone-level checkpoints ("Platform research complete. Starting data model..."). Do not stream individual agent thoughts or tool calls.
- **Rationale:** Agent-level streaming is noisy and confusing — users see interleaved output from multiple agents with no clear narrative. Milestone checkpoints give users confidence that progress is happening while preserving readability. The `--verbose` flag can be added later for debugging.

### PDR-002 — Single binary entry point, not daemon

- **Status:** Accepted
- **Problem:** The agent-parallel system requires multiple agents. Should they be a persistent daemon or spawned on demand?
- **Decision:** Single binary that spawns agents as needed and exits when done. No persistent daemon, no background service.
- **Rationale:** Users expect CLI tools to start, do work, and exit. A daemon model adds lifecycle management (start, stop, restart, health checks) that is inappropriate for a project planning tool. Spawning agents on demand adds a few seconds of startup but eliminates a category of operational complexity (stale processes, port conflicts, zombie agents).

### PDR-003 — `/decompose` skill remains the primary UX

- **Status:** Accepted
- **Problem:** With a Go binary handling orchestration, should users switch to a new CLI command or keep using `/decompose`?
- **Decision:** The `/decompose` skill remains the entry point. It detects whether the Go binary is available and delegates to it transparently. Users who don't have the binary get current single-agent behavior unchanged.
- **Rationale:** The skill is already documented, integrated into Claude Code, and familiar to users. Forcing a UX change for a backend improvement is unnecessary friction. The Go binary can also be invoked directly for CI/CD or non-Claude workflows — both entry points work.

### PDR-004 — Three-layer entry point: MCP server + CLI + skill fallback

- **Status:** Accepted
- **Problem:** Users need multiple ways to interact with the system — Claude Code users via `/decompose`, CI/CD pipelines via CLI, and power users who want deep Claude Code integration via MCP tools. How do these coexist?
- **Decision:** The Go binary serves three roles:
  1. **MCP server** (`decompose --serve-mcp`): registers as a Claude Code MCP server, exposes tools (`run_stage`, `get_status`, `list_decompositions`). Configured in the project's MCP settings.
  2. **CLI** (`decompose <name> <stage>`): standalone command for scripts, CI/CD, and non-Claude workflows.
  3. **Subprocess**: the `/decompose` skill shells out to the binary when it's in PATH.
  The skill's detection order is: MCP tools available → subprocess → single-agent fallback.
- **Rationale:** MCP integration is the tightest Claude Code experience — the skill calls structured tools rather than parsing CLI stdout. CLI mode gives maximum portability. Both use the same orchestrator core. Distribution via `go install` (developers) and GitHub binary releases (everyone else).

---

## Condensed PRD *(required)*

**Goal:** Evolve the progressive decomposition pipeline into a multi-agent parallel system that produces higher-quality decomposition artifacts faster, while maintaining full backward compatibility with the existing single-agent workflow.

**Primary User Stories:**

1. As a developer, I can run `/decompose` and have specialist agents work in parallel on independent sections of the design pack, reducing Stage 1 completion time.
2. As a developer, I can get structurally accurate task plans because the Planning Agent has a real dependency graph of my codebase (via Tree-sitter + graph DB), not just file-reading heuristics.
3. As a developer who doesn't have the Go binary installed, I can still use `/decompose` exactly as before — the system gracefully falls back to single-agent mode.
4. As a developer, I can see which stage sections are complete and which are in progress during a multi-agent decomposition.

**Non-Goals (this version):**

- Remote agent deployment (v2)
- Token budget enforcement (v2 — tracked for observability only)
- Custom agent plugins (users can't add their own specialist agents in v1)
- Real-time streaming of agent thought processes (milestone callbacks only)
- CI/CD integration (the binary works standalone, but no official pipeline templates)

**Success Criteria:**

- Stage 1 wall-clock time is reduced by 40%+ when running with all specialist agents vs. single-agent mode
- Stage 4 wall-clock time scales linearly with milestone count (one Task Writer per milestone)
- Code intelligence graph builds in < 10 seconds for projects with < 10k files
- Zero behavioral change when running without the Go binary (graceful degradation)
- All 4 degradation levels produce valid decomposition output

---

## Data Lifecycle & Retention *(required)*

- **Deletion behavior:** Decomposition output files (`docs/decompose/<name>/`) are user-owned plaintext. Deleting the directory removes all artifacts. The code intelligence graph is in-memory and discarded when the decomposition run completes (or the binary exits). A2A task records exist only in memory during the run — no persistent task store.
- **Export format:** All decomposition output is Markdown files in a well-defined directory structure. No proprietary format. Git-friendly (commit and version the output alongside your code).
- **Retention policy:** No automatic retention or cleanup. Output files persist until the user deletes them. The ephemeral graph and A2A task state are not retained — if you need to re-analyze, re-run the decomposition.

---

## Testing Strategy *(required)*

### Unit Tests

- Capability detection logic: given a set of available services, assert correct capability level
- Stage routing: given arguments and capability level, assert correct execution path
- Section-based merge: given N agent artifacts with pre-assigned sections, assert correct merge order and content
- A2A message serialization: round-trip encode/decode for all task states, artifacts, and parts
- Graph query results: given a known graph, assert correct dependency chains, blast radius, cluster detection
- Tree-sitter symbol extraction: given Go/TS/Python/Rust source, assert correct symbols, imports, and call sites

### Integration Tests

- A2A round-trip: orchestrator creates task → specialist accepts → specialist produces artifact → orchestrator receives
- MCP tool invocation: client connects to code intelligence MCP server → calls `build_graph` → receives graph stats
- Tree-sitter → graph pipeline: parse a known codebase → extract symbols → build KuzuDB graph → query dependencies → assert correctness
- Graceful degradation: start with no agents available → verify single-agent fallback produces valid output
- Merge coherence: run two Research Agents with contradicting platform info → verify orchestrator flags the contradiction

### E2E Tests

- Full pipeline: given a small reference project, run `/decompose <name> 1` through `/decompose <name> 4` and verify all output files are well-formed and cross-reference correctly
- Golden path: compare output against a known-good baseline for a reference project

---

## Implementation Plan *(required)*

1. **M1: Project scaffolding + A2A types** — Go module setup, CLI entry point, A2A message types from proto spec, basic orchestrator shell
2. **M2: Code intelligence layer** — Tree-sitter parsing, symbol extraction, KuzuDB graph construction, Cypher queries for dependency analysis and clustering
3. **M3: MCP tool servers** — Code intelligence exposed as MCP tools (`build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`) using the Go SDK
4. **M4: A2A agent framework** — Agent Card serving, A2A server/client implementation, task lifecycle management, SSE streaming
5. **M5: Specialist agents** — Research Agent, Schema Agent, Planning Agent, Task Writer Agent implementations with their MCP tool connections
6. **M6: Orchestrator pipeline** — Stage routing, fan-out/fan-in, section-based merge, coherence checking, progress reporting
7. **M7: Graceful degradation + `/decompose` integration** — Capability detection, four-level fallback, skill integration (skill detects binary and delegates)
8. **M8: Testing + release** — Integration tests for all degradation levels, E2E golden path tests, goreleaser config, cross-platform builds

---

## Before Moving On

Verify before proceeding to Stage 2:

- [x] Every assumption is written down
- [x] Platform/tooling versions are specific and researched (not guessed)
- [x] Data model covers every entity with all fields, types, and relationships
- [x] Architecture pattern is named and justified
- [x] At least 3 ADRs are written (7 total)
- [x] At least 2 PDRs are written (3 total)
- [x] Implementation plan has an ordered milestone list (8 milestones)
- [x] You can describe the project in one sentence (the PRD goal)
