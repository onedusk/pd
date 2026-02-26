# Agent-Parallel Decomposition — Design Pack

> Internal design document for the next evolution of Progressive Decomposition.
> This serves as a Stage 1 Design Pack: architecture, decisions, and open questions
> to be decomposed into Stages 2–4 in future sessions.
> when decomposing, please reference the actual [A2A spec](../../../A2A/)
  - within the aforementioned filepath, we can generate a protobuf if/when needed
> when decomposing, please use firecrawl skill to grab whats needed for MCP GO-sdk - https://github.com/modelcontextprotocol/go-sdk

---

## 1. Problem Statement

The current progressive decomposition pipeline is **sequential and single-agent**. Each stage blocks the next. A single Claude instance must research, design, write code skeletons, plan milestones, and produce task specs — all within one context window, one at a time.

This creates two bottlenecks:

1. **Throughput** — stages that contain parallelizable work (Stage 1's 14 sections, Stage 4's per-milestone task files) are processed serially.
2. **Depth** — a single agent can't simultaneously run web research, type-check skeleton code, analyze dependency graphs, and write task specs. It can only approximate each of these by reading files and reasoning.

The goal is to evolve the pipeline so that:
- Independent work fans out to specialist agents that run in parallel
- Each agent is equipped with domain-specific tools (via MCP) that make it genuinely capable, not just a general LLM guessing
- Agents coordinate via a standard protocol (A2A) so the system is framework-agnostic and extensible
- The existing single-agent `/decompose` skill remains the entry point and fallback

---

## 2. Assumptions & Constraints

1. **Evolution, not replacement.** The agent-parallel system must be an upgrade path for the existing `/decompose` skill. Users who don't have A2A infrastructure get the current single-agent behavior unchanged.
2. **A2A is the coordination protocol.** Google's Agent-to-Agent protocol (v0.3.0, launched under Linux Foundation governance (June 2025)) — HTTP + JSON-RPC 2.0, with SSE streaming and push notifications. No proprietary coordination layer.
3. **MCP is the tool protocol.** Model Context Protocol is already adopted by Claude Code, Cursor, Windsurf, and OpenCode. Each specialist agent gets its capabilities through MCP tools.
4. **Code intelligence is self-contained.** The system builds its own temporary knowledge graph per decomposition — it does not depend on GitNexus or any external service. It uses the same *methodology* (Tree-sitter parsing, graph-based dependency analysis) but owns its implementation.
5. **Graceful degradation is mandatory.** If A2A agents aren't available → single-agent mode. If MCP tools aren't available → Claude's native capabilities. If code intelligence isn't available → file-reading heuristics (current behavior). No hard dependencies.
6. **No cloud infrastructure required.** Everything runs locally. Agents can be local processes, not remote services.
7. **Go is the implementation language.** Single-binary distribution, trivial cross-compilation, official MCP Go SDK (Google-maintained), goroutines for agent parallelism. See ADR-005.

---

## 3. Architecture

### 3.1 System Overview

```
User
  │
  ▼
/decompose skill (entry point)
  │
  ▼
Orchestrator Agent (A2A server + client)
  │
  ├──► Research Agent (A2A server)
  │      └── MCP tools: web search, docs fetcher, API explorer, version checker
  │
  ├──► Schema Agent (A2A server)
  │      └── MCP tools: language server, type checker, linter, AST parser
  │
  ├──► Planning Agent (A2A server)
  │      └── MCP tools: code graph builder, dependency analyzer, blast radius
  │
  └──► Task Writer Agent (A2A server)
         └── MCP tools: codebase search, test runner, file system
```

### 3.2 Agent Roles

#### Orchestrator Agent

The conductor. Manages the overall pipeline, decides what to parallelize, fans out work to specialists, merges their outputs, and handles user interaction.

**Responsibilities:**
- Parse `/decompose` arguments and determine which stage(s) to run
- Discover available specialist agents via Agent Cards
- Create A2A tasks and assign them to specialists
- Merge artifacts from parallel agents into coherent stage outputs
- Handle INPUT_REQUIRED states by routing questions to the user
- Write final output files to `docs/decompose/<name>/`
- Fall back to single-agent mode when specialists aren't available

**Does NOT:**
- Research platforms, write code, build graphs, or produce task specs directly (delegates all domain work)

#### Research Agent

Platform investigation and verification specialist.

**Skills (declared in Agent Card):**
- `research-platform`: Investigate a platform/framework/SDK — current version, API surface, known limitations
- `verify-versions`: Cross-check version numbers against official sources
- `explore-codebase`: Understand existing project structure, patterns, and conventions

**MCP tools it uses:**
- Web search (Firecrawl or similar)
- Documentation fetcher
- File system reader (for existing codebase)

**Artifacts it produces:**
- Platform & tooling baseline (Stage 1 section)
- Integration points analysis (Stage 1 section)
- Codebase exploration summary (existing patterns, conventions)

#### Schema Agent

Type system and code skeleton specialist.

**Skills:**
- `translate-schema`: Convert a data model specification into compilable code
- `validate-types`: Type-check skeleton code against the target language
- `write-contracts`: Produce interface contracts (request/response types, API DTOs)

**MCP tools it uses:**
- Language server (for type checking and validation)
- AST parser (Tree-sitter)
- Linter (for style validation)

**Artifacts it produces:**
- Data model code (Stage 2)
- Interface contracts (Stage 2)
- Validation report (compilation status, ambiguities found)

#### Planning Agent

Dependency analysis and milestone planning specialist. This is where **code intelligence** lives.

**Skills:**
- `build-code-graph`: Parse a codebase into a knowledge graph (dependencies, call chains, clusters)
- `analyze-dependencies`: Compute dependency graph between proposed changes
- `assess-impact`: Blast radius analysis for a set of file changes
- `plan-milestones`: Organize work into dependency-ordered milestones

**MCP tools it uses:**
- Code graph builder (Tree-sitter + graph DB)
- Dependency analyzer
- File system reader
- Blast radius calculator

**Artifacts it produces:**
- Milestone dependency graph (Stage 3)
- Target directory tree (Stage 3)
- Impact analysis per milestone (used by Task Writer)

#### Task Writer Agent

Per-milestone task specification specialist. Multiple instances can run in parallel (one per milestone).

**Skills:**
- `write-task-specs`: Produce a complete task file for one milestone
- `validate-dependencies`: Check cross-milestone task dependencies

**MCP tools it uses:**
- Codebase search (to reference existing code in task outlines)
- Impact data from Planning Agent (blast radius per file)

**Artifacts it produces:**
- `tasks_m{NN}.md` files (Stage 4)

### 3.3 Communication Flow

```
                    ┌─────────────────────────────────────────────┐
                    │              Orchestrator                    │
                    │                                             │
Stage 0 ─────────► │ (writes directly — simple, no delegation)   │
                    │                                             │
Stage 1 ─────────► │ ──► Research Agent (platform, integrations) │
                    │ ──► Research Agent (codebase exploration)   │  parallel
                    │ ──► user questions (assumptions, features)  │
                    │ ◄── merge artifacts into design pack        │
                    │                                             │
Stage 2 ─────────► │ ──► Schema Agent (model code)               │
                    │ ──► Schema Agent (interface contracts)      │  parallel
                    │ ◄── merge + validate (type check)           │
                    │                                             │
Stage 3 ─────────► │ ──► Planning Agent (build graph, plan)      │
                    │ ◄── dependency graph + directory tree       │  sequential
                    │                                             │
Stage 4 ─────────► │ ──► Task Writer (M1)                        │
                    │ ──► Task Writer (M2)                        │
                    │ ──► Task Writer (M3)                        │  parallel
                    │ ──► ...                                     │
                    │ ◄── merge + validate cross-dependencies     │
                    └─────────────────────────────────────────────┘
```

### 3.4 A2A Mapping

| Progressive Decomposition Concept | A2A Concept |
|----------------------------------|-------------|
| Named decomposition (`auth-system`) | `contextId` — groups all related tasks |
| Stage | A2A `Task` — SUBMITTED → WORKING → COMPLETED |
| Stage output file | A2A `Artifact` — carried between agents |
| User clarification point | `INPUT_REQUIRED` task state |
| Stage section (within Stage 1) | Sub-task, linked via `referenceTaskIds` |
| `/decompose status` | `ListTasks` filtered by contextId |

### 3.5 Graceful Degradation

| Available | Behavior |
|-----------|----------|
| A2A + MCP + Code Intelligence | Full parallel pipeline with graph-informed planning |
| A2A + MCP (no code graph) | Parallel pipeline, file-reading heuristics instead of graph analysis |
| MCP only (no A2A) | Single agent with enhanced tools (current skill + MCP tools) |
| Neither | Current `/decompose` skill behavior (single agent, native capabilities) |

---

## 4. Code Intelligence Layer

### 4.1 Purpose

Give the Planning Agent a structural understanding of the codebase — not just file names and contents, but actual dependency chains, call graphs, and functional clusters. This makes Stage 3 (task index) and Stage 4 (task specs) dramatically more accurate.

### 4.2 How It Works

1. **Parse** — Tree-sitter extracts AST from all source files in the target codebase
2. **Extract** — Walk ASTs to identify: symbols (functions, classes, types), imports/dependencies, call sites, exports
3. **Build graph** — Store relationships in a graph database:
   - `IMPORTS` edges (file A imports from file B)
   - `CALLS` edges (function X calls function Y)
   - `DEFINES` edges (file A defines symbol Z)
   - `INHERITS` / `IMPLEMENTS` edges
4. **Cluster** — Identify functional groups (files that are tightly connected form a cluster)
5. **Expose** — Make the graph queryable via MCP tools

### 4.3 Graph Schema

```
Nodes:
  File      { path, language, loc }
  Symbol    { name, kind (function|class|type|enum|interface), exported }
  Cluster   { name, cohesion_score }

Edges:
  File    ─[DEFINES]──►  Symbol
  File    ─[IMPORTS]──►  File
  Symbol  ─[CALLS]────►  Symbol
  Symbol  ─[INHERITS]─►  Symbol
  File    ─[BELONGS]──►  Cluster
```

### 4.4 MCP Tools Exposed

| Tool | Input | Output | Used By |
|------|-------|--------|---------|
| `build_graph` | repo path | graph stats (nodes, edges, clusters) | Planning Agent |
| `query_symbols` | search query | matching symbols with context | Planning Agent, Task Writer |
| `get_dependencies` | file or symbol | upstream/downstream dependency chain | Planning Agent |
| `assess_impact` | list of files to change | affected files, blast radius, risk | Planning Agent, Task Writer |
| `get_clusters` | — | functional clusters with members | Planning Agent |

### 4.5 Temporary by Design

The knowledge graph is built fresh for each decomposition and discarded after. Reasons:
- No stale data — the graph reflects the codebase *at the moment of decomposition*
- No infrastructure — no persistent graph database to maintain
- Scoped — only indexes files relevant to the decomposition, not the entire monorepo
- Fast — a typical project (< 10k files) indexes in seconds with Tree-sitter

---

## 5. Parallelism Model

### 5.1 Within-Stage Parallelism

**Stage 1 (Design Pack)** — highest parallelism potential:

```
Orchestrator fans out:
  ├── Research Agent: platform versions     ─┐
  ├── Research Agent: codebase exploration   ├── parallel
  ├── Research Agent: integration points     ─┘
  ├── User Q&A: assumptions, features        ── sequential (needs user)
  └── Orchestrator: merge all into design pack
```

Independent sections: platform/tooling, integration points, codebase patterns.
Dependent sections: data model (needs features), architecture (needs data model), ADRs (need architecture).

**Stage 2 (Skeletons)** — moderate parallelism:

```
Schema Agent: model group A  ─┐
Schema Agent: model group B   ├── parallel (if independent)
Schema Agent: API contracts   ─┘
Type-check all together       ── sequential (validation)
```

**Stage 3 (Task Index)** — mostly sequential:

The Planning Agent builds one dependency graph. This is inherently a single-agent task, though it can use parallel MCP tool calls internally (query multiple clusters simultaneously).

**Stage 4 (Task Specs)** — highest parallelism:

```
Task Writer: M1  ─┐
Task Writer: M2   │
Task Writer: M3   ├── fully parallel (once Stage 3 exists)
Task Writer: M4   │
Task Writer: M5  ─┘
Cross-dependency validation  ── sequential (merge)
```

### 5.2 Cross-Stage Parallelism

Stages are fundamentally sequential (each builds on the previous). However:
- Stage 2 can start *partially* before Stage 1 is fully complete (data model section is usually done first)
- Stage 4 milestones can start as soon as their section of Stage 3 is written

These are optimizations, not requirements. The default is strict stage ordering.

### 5.3 Cross-Decomposition Parallelism

Named decompositions (`auth-system`, `payment-flow`) are completely independent after Stage 0. Multiple orchestrators can run simultaneously for different decompositions. Each gets its own `contextId`.

---

## 6. Architecture Decision Records

### ADR-001 — A2A over custom coordination protocol

- **Status:** Proposed
- **Context:** Need a way for specialist agents to communicate. Could build a custom protocol, use a message queue, or adopt an existing standard.
- **Decision:** Use A2A (Agent-to-Agent Protocol, v0.3.0, Linux Foundation).
- **Consequences:**
  - (+) Standards-based — any A2A-compatible agent can participate
  - (+) Framework-agnostic — agents can be built with different tools (ADK, LangGraph, CrewAI, plain code)
  - (+) Built-in support for streaming (SSE), async (push notifications), and task lifecycle
  - (+) Agent Cards provide discoverable capability declarations
  - (-) A2A is still pre-1.0 — breaking changes possible
  - (-) Requires HTTP server per agent (local processes, but still overhead)

### ADR-002 — MCP for tool integration

- **Status:** Proposed
- **Context:** Specialist agents need access to external tools (web search, type checkers, graph databases). Could embed tools directly, use function calling, or use a tool protocol.
- **Decision:** Use MCP (Model Context Protocol) for all tool integration.
- **Consequences:**
  - (+) Already adopted by Claude Code, Cursor, Windsurf, OpenCode — ecosystem is mature
  - (+) Tools are reusable across agents (same MCP server can serve multiple agents)
  - (+) Clean separation between agent logic and tool capability
  - (+) Extensible — users can add their own MCP tools
  - (-) Adds a layer of indirection vs. direct function calls
  - (-) MCP servers must be running for tools to work

### ADR-003 — Temporary graph over persistent index

- **Status:** Proposed
- **Context:** The code intelligence layer needs a graph database for dependency analysis. Could use a persistent index (like GitNexus) or build a temporary one per decomposition.
- **Decision:** Build a temporary, purpose-built graph per decomposition. Discard after use.
- **Consequences:**
  - (+) No stale data — graph reflects codebase at moment of decomposition
  - (+) No infrastructure to maintain — no persistent DB process
  - (+) Scoped to relevant files — doesn't index the entire monorepo
  - (+) Fast for typical projects (< 10k files with Tree-sitter)
  - (-) Re-indexes if the same codebase is decomposed again (acceptable tradeoff)
  - (-) No historical comparison between decompositions

### ADR-004 — Graceful degradation to single-agent

- **Status:** Proposed
- **Context:** Not all users will have A2A infrastructure or MCP tools configured. The system must work without them.
- **Decision:** Implement a capability detection layer. The `/decompose` skill checks what's available and adjusts behavior accordingly.
- **Consequences:**
  - (+) Zero-config default — current skill works unchanged
  - (+) Progressive enhancement — each layer (MCP, A2A, code intelligence) adds capability
  - (+) No hard dependencies on any external system
  - (-) Must maintain two code paths (single-agent and multi-agent)
  - (-) Testing matrix increases (4 degradation levels)

### ADR-005 — Go as implementation language

- **Status:** Proposed
- **Context:** The agent-parallel system needs a language that produces single-binary distributions, cross-compiles trivially, and has strong support for both A2A (HTTP + JSON-RPC + concurrent agents) and MCP. Candidates: Go, Rust, TypeScript, Python.
- **Decision:** Go.
- **Rationale:**
  - **Single binary, trivial cross-compilation.** `GOOS=linux GOARCH=amd64 go build` — no runtime dependencies, no installer. Users download one binary and run it.
  - **Official MCP Go SDK.** `github.com/modelcontextprotocol/go-sdk` — maintained in collaboration with Google, well-starred and actively maintained (commits from Feb 2026). Provides server + client APIs, stdio + StreamableHTTP transports, OAuth 2.0, JSON-RPC, tools, resources, prompts, and sampling. This is not a community wrapper — it's under the `modelcontextprotocol` GitHub org.
  - **Goroutines map directly to parallel agents.** Fan out N specialist agents, each in a goroutine, coordinate via channels. No async/await ceremony, no runtime overhead.
  - **A2A is HTTP + JSON-RPC 2.0.** Go's `net/http` stdlib handles this natively. No framework needed.
  - **Tree-sitter has solid Go bindings.** `go-tree-sitter` — not the official binding (that's Rust), but well-maintained and sufficient for AST extraction (we're parsing to extract symbols, not building an LSP).
  - **KuzuDB has Go bindings.** Embedded graph database with C FFI — works via cgo.
  - **Contributor accessibility.** Simpler language than Rust, lower barrier to contribute and maintain.
- **Consequences:**
  - (+) Single binary distribution — `go install` or download from releases
  - (+) Cross-platform in one command — macOS, Linux, Windows from any build host
  - (+) Goroutines + channels are the natural concurrency model for agent orchestration
  - (+) Official MCP SDK eliminates protocol implementation risk
  - (+) Fast builds (seconds, not minutes)
  - (-) Tree-sitter Go bindings are not first-class (Rust's are) — may lag on new grammars
  - (-) cgo required for KuzuDB and Tree-sitter — complicates cross-compilation slightly (need C toolchain per target)
  - (-) No official A2A Go SDK yet — must implement against the spec directly (HTTP + JSON-RPC, straightforward in Go)

  **Alternatives rejected:**
  - **Rust:** First-class Tree-sitter bindings, smaller binaries, no GC. But harder to write, slower builds, steeper contributor barrier. The workload is I/O-bound (HTTP, file parsing) — Rust's performance advantage doesn't apply.
  - **TypeScript:** Tier 1 MCP SDK, large ecosystem. But requires Node.js runtime — no single binary. Distribution friction for a CLI tool.
  - **Python:** Tier 1 MCP SDK, strongest AI ecosystem. But requires Python runtime, venv management, slow startup. Unacceptable for a CLI tool users install globally.

---

## 7. Resolved Questions

> These were open questions during initial design. All have been decided.

### Architecture

1. **Graph database: KuzuDB.**
   Embedded, no server process, fast for local use, Cypher query language, C bindings via cgo. Used by GitNexus in production. Rejected SQLite (graph queries require recursive CTEs — awkward and slow for deep traversals) and in-memory adjacency lists (no query language, must build traversal from scratch).

2. **Agent deployment: local-only for v1.**
   Orchestrator spawns agents as local goroutines or child processes. No networking config, no auth, no service discovery. Remote deployment can be added in v2 via A2A's built-in HTTP transport — the protocol already supports it, we just don't need it yet.

3. **Orchestrator identity: `/decompose` skill delegates to a Go binary.**
   The skill stays lean (markdown + shell preprocessing). When invoked, it calls the Go orchestrator binary which owns all agent coordination, A2A communication, and MCP tool management. Clean separation — the skill is the UX layer, the binary is the engine. This also means the orchestrator works independently of Claude Code (could be invoked from any CI/CD pipeline, CLI, or other agent framework).

### Implementation

4. **Token budget: deferred to v2.**
   No budget enforcement in v1. Agents run freely. Usage is tracked for observability (agents report token counts in A2A artifact metadata) but not capped. This avoids premature optimization — we need real-world data on per-stage costs before designing a budget system.

5. **Merge strategy: section-based concatenation.**
   Each parallel agent writes to a pre-assigned named section. The orchestrator concatenates sections in template order. No conflict is possible because sections are assigned before fan-out. After section-based concatenation, the orchestrator runs a lightweight coherence check: it reads the merged output and flags potential contradictions between sections (e.g., a platform limitation discovered by one Research Agent instance that conflicts with an integration approach proposed by another). This is a focused cross-section consistency scan, not a full LLM re-generation pass. If contradictions are found, the orchestrator routes them back to the relevant agents as INPUT_REQUIRED tasks.

6. **Tree-sitter tier-1 languages: Go, TypeScript, Python, Rust.**
   These four get full graph support (symbol extraction, call chains, dependency edges, cluster detection) and are tested in CI. All other Tree-sitter-supported languages get best-effort parsing — symbols and imports are extracted, but call chain accuracy is not guaranteed. Users can report quality issues for specific languages to promote them to tier-1.

### Product

7. **Progress UX: milestone callbacks.**
   The orchestrator reports when each agent completes its task. User sees checkpoint-level updates: "Platform research complete. Starting data model..." Not real-time streaming of every agent thought — that's too noisy. Keeps the UX informative without overwhelming.

8. **Code graph: always build it.**
   The graph is cheap to build (seconds with Tree-sitter for typical projects). No threshold logic — just always build it. Even small projects benefit from dependency detection. Eliminates a code path (threshold checking) and a class of bugs (threshold too high/low for specific projects).

---

## 8. Next Steps

This document is a Stage 1 artifact. To continue the decomposition:

1. **Stage 2 (Skeletons)** — Define Go types for: Agent Card schemas, A2A message types for decomposition artifacts, MCP tool interfaces for code intelligence, graph schema (nodes + edges).

2. **Stage 3 (Task Index)** — Break the implementation into milestones:
   - M1: Code intelligence layer (Tree-sitter + graph)
   - M2: MCP tools wrapping the code intelligence
   - M3: A2A agent scaffolding (orchestrator + agent cards)
   - M4: Specialist agent implementations
   - M5: `/decompose` skill upgrade (orchestrator integration)
   - M6: Graceful degradation + testing

3. **Stage 4 (Task Specs)** — Per-milestone task files with file-level granularity.

These will be produced in `docs/internal/` as the decomposition continues.
