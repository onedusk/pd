# Stage 3: Task Index — Agent-Parallel Decomposition

> Master build plan derived from the Design Pack (Stage 1) and Implementation Skeletons (Stage 2).
>
> Module: `github.com/dusk-indust/decompose`

---

## Legend

- `[ ]` — Not started
- `[x]` — Complete
- **CREATE** — new file
- **MODIFY** — edit existing file
- **DELETE** — remove file
- Task IDs: `T-{milestone}.{sequence}` (e.g., T-01.03 = Milestone 1, task 3)

---

## Progress

| # | Milestone | File | Tasks | Done |
|---|-----------|------|:-----:|:----:|
| M1 | Project Scaffolding + A2A Types | [tasks_m01.md](tasks_m01.md) | 9 | 0 |
| M2 | Code Intelligence Layer | [tasks_m02.md](tasks_m02.md) | 12 | 0 |
| M3 | MCP Tool Servers | [tasks_m03.md](tasks_m03.md) | 6 | 0 |
| M4 | A2A Agent Framework | [tasks_m04.md](tasks_m04.md) | 9 | 0 |
| M5 | Specialist Agents | [tasks_m05.md](tasks_m05.md) | 9 | 0 |
| M6 | Orchestrator Pipeline | [tasks_m06.md](tasks_m06.md) | 10 | 0 |
| M7 | Graceful Degradation + Skill Integration | [tasks_m07.md](tasks_m07.md) | 10 | 0 |
| M8 | Testing + Release | [tasks_m08.md](tasks_m08.md) | 8 | 0 |
| | **Total** | | **73** | **0** |

---

## Milestone Dependencies

```
M1 ──┬──► M2 ──► M3 ──┐
     │                  ├──► M5 ──► M6 ──► M7 ──► M8
     └──► M4 ──────────┘
```

**Critical path:** M1 → M2 → M3 → M5 → M6 → M7 → M8

**Parallelizable:**
- **M2 ‖ M4** — Code intelligence layer and A2A agent framework are independent after M1. Both build on the types defined in M1 but do not depend on each other.
- **M5 specialists** — Individual agent implementations (Research, Schema, Planning, Task Writer) are internally parallelizable once M3 + M4 are complete.

**Convergence point:** M5 requires both M3 (MCP tools for agents to call) and M4 (A2A framework for agents to serve).

### Expanded Dependency Detail

```
M1: Project Scaffolding + A2A Types
│   Produces: go.mod, A2A types, graph schema types, orchestrator config,
│             CLI shell. Foundation for everything.
│
├──► M2: Code Intelligence Layer
│    │   Depends on: M1 (graph schema types)
│    │   Produces: Tree-sitter parser, KuzuDB store, clustering
│    │
│    └──► M3: MCP Tool Servers
│         │   Depends on: M2 (graph Store + Parser to wrap)
│         │   Produces: MCP server exposing code intelligence as tools
│         │
│         └──────────┐
│                     ▼
├──► M4: A2A Agent Framework
│    │   Depends on: M1 (A2A types)
│    │   Produces: HTTP client/server, SSE streaming, agent base, task store
│    │
│    └──────────┐
│               ▼
│         M5: Specialist Agents
│         │   Depends on: M3 (MCP tools to call) + M4 (A2A framework to serve)
│         │   Produces: Research, Schema, Planning, Task Writer agents
│         │
│         └──► M6: Orchestrator Pipeline
│              │   Depends on: M5 (agents to orchestrate)
│              │   Produces: Stage routing, fan-out, merge, coherence, progress
│              │
│              └──► M7: Graceful Degradation + Skill Integration
│                   │   Depends on: M6 (full pipeline to degrade from)
│                   │   Produces: Capability detection, fallback paths, skill shim
│                   │
│                   └──► M8: Testing + Release
│                        Depends on: M7 (all code paths complete)
│                        Produces: E2E tests, goreleaser, CI, docs
```

---

## Feature → Milestone Mapping

| Feature (from Stage 1) | Milestone(s) |
|------------------------|-------------|
| Orchestrator engine | M1 (shell), M6 (implementation) |
| A2A task management | M1 (types), M4 (client/server) |
| Agent discovery | M4 (Agent Card serving), M6 (discovery loop) |
| Specialist: Research Agent | M5 |
| Specialist: Schema Agent | M5 |
| Specialist: Planning Agent | M5 |
| Specialist: Task Writer Agent | M5 |
| Code intelligence graph | M2 (core), M3 (MCP exposure) |
| Graph queries via MCP | M3 |
| Agent Card discovery | M4, M6 |
| Decomposition status | M6 |
| Graceful degradation | M7 |
| Section-based merge | M6 |
| Progress reporting | M6 |
| Token tracking | M5 (artifact metadata), M6 (aggregation) |
| `/decompose` skill integration | M7 |
| MCP server mode (`--serve-mcp`) | M7 |
| MCP tool protocol | M3 |
| A2A protocol compliance | M1 (types), M4 (implementation) |

| ADR/PDR | Fulfilled by |
|---------|-------------|
| ADR-001 (A2A) | M1, M4 |
| ADR-002 (MCP) | M3 |
| ADR-003 (Temporary graph) | M2 |
| ADR-004 (Graceful degradation) | M7 |
| ADR-005 (Go) | M1 |
| ADR-006 (KuzuDB behind interface) | M2 |
| ADR-007 (Official Tree-sitter) | M2 |
| PDR-001 (Milestone-level progress) | M6 |
| PDR-002 (Single binary, not daemon) | M1, M8 |
| PDR-003 (/decompose skill remains UX) | M7 |
| PDR-004 (Three-layer entry: MCP + CLI + skill) | M7 |

---

## Target Directory Tree

```
github.com/dusk-indust/decompose/
│
├── go.mod                                        CREATE (M1)
├── go.sum                                        CREATE (M1)
├── Makefile                                      CREATE (M8)
├── .goreleaser.yml                               CREATE (M8)
├── README.md                                     CREATE (M8)
├── LICENSE                                       CREATE (M1)
│
├── cmd/
│   └── decompose/
│       └── main.go                               CREATE (M1), MODIFY (M6, M7)
│
├── internal/
│   ├── a2a/
│   │   ├── types.go                              CREATE (M1)
│   │   ├── types_test.go                         CREATE (M1)
│   │   ├── jsonrpc.go                            CREATE (M1)
│   │   ├── client.go                             CREATE (M1), MODIFY (M4)
│   │   ├── server.go                             CREATE (M1), MODIFY (M4)
│   │   ├── httpclient.go                         CREATE (M4)
│   │   ├── httpclient_test.go                    CREATE (M4)
│   │   ├── httpserver.go                         CREATE (M4)
│   │   ├── httpserver_test.go                    CREATE (M4)
│   │   ├── sse.go                                CREATE (M4)
│   │   ├── sse_test.go                           CREATE (M4)
│   │   └── taskstore.go                          CREATE (M4)
│   │
│   ├── graph/
│   │   ├── schema.go                             CREATE (M1)
│   │   ├── store.go                              CREATE (M1), MODIFY (M2)
│   │   ├── parser.go                             CREATE (M1), MODIFY (M2)
│   │   ├── treesitter.go                         CREATE (M2)
│   │   ├── treesitter_go.go                      CREATE (M2)
│   │   ├── treesitter_ts.go                      CREATE (M2)
│   │   ├── treesitter_py.go                      CREATE (M2)
│   │   ├── treesitter_rs.go                      CREATE (M2)
│   │   ├── treesitter_test.go                    CREATE (M2)
│   │   ├── kuzustore.go                          CREATE (M2)
│   │   ├── kuzustore_test.go                     CREATE (M2)
│   │   ├── memstore.go                           CREATE (M2)
│   │   ├── cluster.go                            CREATE (M2)
│   │   └── cluster_test.go                       CREATE (M2)
│   │
│   ├── mcptools/
│   │   ├── codeintel.go                          CREATE (M3)
│   │   ├── server.go                             CREATE (M3)
│   │   ├── handlers.go                           CREATE (M3)
│   │   ├── handlers_test.go                      CREATE (M3)
│   │   ├── server_test.go                        CREATE (M3)
│   │   ├── decompose_server.go                   CREATE (M7)
│   │   ├── decompose_handlers.go                 CREATE (M7)
│   │   └── decompose_server_test.go              CREATE (M7)
│   │
│   ├── agent/
│   │   ├── agent.go                              CREATE (M1), MODIFY (M4)
│   │   ├── base.go                               CREATE (M4)
│   │   ├── research.go                           CREATE (M5)
│   │   ├── research_test.go                      CREATE (M5)
│   │   ├── schema.go                             CREATE (M5)
│   │   ├── schema_test.go                        CREATE (M5)
│   │   ├── planning.go                           CREATE (M5)
│   │   ├── planning_test.go                      CREATE (M5)
│   │   ├── taskwriter.go                         CREATE (M5)
│   │   ├── taskwriter_test.go                    CREATE (M5)
│   │   └── registry.go                           CREATE (M5)
│   │
│   ├── orchestrator/
│   │   ├── config.go                             CREATE (M1)
│   │   ├── orchestrator.go                       CREATE (M1), MODIFY (M6)
│   │   ├── merge.go                              CREATE (M1), MODIFY (M6)
│   │   ├── detector.go                           CREATE (M1), MODIFY (M7)
│   │   ├── router.go                             CREATE (M6)
│   │   ├── router_test.go                        CREATE (M6)
│   │   ├── fanout.go                             CREATE (M6)
│   │   ├── fanout_test.go                        CREATE (M6)
│   │   ├── coherence.go                          CREATE (M6)
│   │   ├── coherence_test.go                     CREATE (M6)
│   │   ├── progress.go                           CREATE (M6)
│   │   ├── merge_impl.go                         CREATE (M6)
│   │   ├── merge_test.go                         CREATE (M6)
│   │   ├── pipeline.go                           CREATE (M6)
│   │   ├── pipeline_test.go                      CREATE (M6)
│   │   ├── detector_impl.go                      CREATE (M7)
│   │   ├── detector_test.go                      CREATE (M7)
│   │   ├── fallback.go                           CREATE (M7)
│   │   ├── fallback_test.go                      CREATE (M7)
│   │   └── degradation_test.go                   CREATE (M7)
│   │
│   └── e2e/
│       ├── pipeline_test.go                      CREATE (M8)
│       └── golden_test.go                        CREATE (M8)
│
├── testdata/
│   ├── fixtures/
│   │   ├── go_project/                           CREATE (M2)
│   │   ├── ts_project/                           CREATE (M2)
│   │   ├── py_project/                           CREATE (M2)
│   │   └── rs_project/                           CREATE (M2)
│   └── golden/
│       ├── stage1_output.md                      CREATE (M8)
│       ├── stage2_output.md                      CREATE (M8)
│       ├── stage3_output.md                      CREATE (M8)
│       └── stage4_output.md                      CREATE (M8)
│
├── .github/
│   └── workflows/
│       └── ci.yml                                CREATE (M8)
│
└── skill/
    └── decompose/
        └── SKILL.md                              MODIFY (M7)
```

**Totals:** 73 files created, 10 modifications, 0 deletions

---

## Milestone Summaries

### M1: Project Scaffolding + A2A Types (9 tasks)

Foundation layer. Everything else depends on this. Produces the Go module, all shared type definitions from Stage 2 skeletons (A2A, graph schema, orchestrator config), the CLI entry point shell, and serialization tests.

**Files:** `go.mod`, `go.sum`, `LICENSE`, `cmd/decompose/main.go`, `internal/a2a/types.go`, `internal/a2a/types_test.go`, `internal/a2a/jsonrpc.go`, `internal/a2a/client.go` (interface only), `internal/a2a/server.go` (interface only), `internal/graph/schema.go`, `internal/graph/store.go` (interface only), `internal/graph/parser.go` (interface only), `internal/orchestrator/config.go`, `internal/orchestrator/orchestrator.go` (interfaces + enums), `internal/orchestrator/merge.go` (types only), `internal/orchestrator/detector.go` (interface only), `internal/agent/agent.go` (interface only)

### M2: Code Intelligence Layer (12 tasks)

Core graph engine. Implements the `Parser` and `Store` interfaces from M1. Tree-sitter parsing with per-language extractors for all four tier-1 languages. KuzuDB-backed graph store behind the `Store` interface. In-memory store for testing. Clustering algorithm. Test fixtures for all tier-1 languages.

**Files:** `internal/graph/treesitter.go`, `internal/graph/treesitter_go.go`, `internal/graph/treesitter_ts.go`, `internal/graph/treesitter_py.go`, `internal/graph/treesitter_rs.go`, `internal/graph/treesitter_test.go`, `internal/graph/kuzustore.go`, `internal/graph/kuzustore_test.go`, `internal/graph/memstore.go`, `internal/graph/cluster.go`, `internal/graph/cluster_test.go`, `testdata/fixtures/` (4 fixture projects). Also MODIFY: `internal/graph/store.go`, `internal/graph/parser.go` (add any implementation details discovered while coding).

### M3: MCP Tool Servers (6 tasks)

Wraps the code intelligence layer from M2 as MCP tools using the Go SDK. Five tools: `build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`. MCP server wiring with StreamableHTTP transport.

**Files:** `internal/mcptools/codeintel.go`, `internal/mcptools/server.go`, `internal/mcptools/handlers.go`, `internal/mcptools/handlers_test.go`, `internal/mcptools/server_test.go`

### M4: A2A Agent Framework (9 tasks)

Implements the A2A protocol in Go. HTTP client for the orchestrator to call agents. HTTP server for agents to receive tasks. SSE streaming for real-time updates. In-memory task store for agent-side task tracking. Base agent implementation with shared boilerplate (HTTP server setup, Agent Card serving, task lifecycle).

**Files:** `internal/a2a/httpclient.go`, `internal/a2a/httpclient_test.go`, `internal/a2a/httpserver.go`, `internal/a2a/httpserver_test.go`, `internal/a2a/sse.go`, `internal/a2a/sse_test.go`, `internal/a2a/taskstore.go`, `internal/agent/base.go`. Also MODIFY: `internal/a2a/client.go`, `internal/a2a/server.go`, `internal/agent/agent.go`.

### M5: Specialist Agents (9 tasks)

Implements the four specialist agents on top of M4's framework. Each agent declares its Agent Card skills, connects to relevant MCP tools from M3, and handles A2A tasks. Research Agent uses web search + file system MCP tools. Schema Agent uses language server + AST parser tools. Planning Agent uses code intelligence graph tools. Task Writer Agent uses codebase search + file system tools.

**Files:** `internal/agent/research.go`, `internal/agent/research_test.go`, `internal/agent/schema.go`, `internal/agent/schema_test.go`, `internal/agent/planning.go`, `internal/agent/planning_test.go`, `internal/agent/taskwriter.go`, `internal/agent/taskwriter_test.go`, `internal/agent/registry.go`

### M6: Orchestrator Pipeline (10 tasks)

Implements the `Orchestrator` interface from M1. Stage routing logic (which stages to run, in what order). Fan-out engine using errgroup (dispatch parallel tasks to agents from M5). Section-based merge with template-order concatenation. Post-merge coherence checking. Progress event emission. Wires everything together into the pipeline.

**Files:** `internal/orchestrator/router.go`, `internal/orchestrator/router_test.go`, `internal/orchestrator/fanout.go`, `internal/orchestrator/fanout_test.go`, `internal/orchestrator/coherence.go`, `internal/orchestrator/coherence_test.go`, `internal/orchestrator/progress.go`, `internal/orchestrator/merge_impl.go`, `internal/orchestrator/merge_test.go`, `internal/orchestrator/pipeline.go`, `internal/orchestrator/pipeline_test.go`. Also MODIFY: `internal/orchestrator/orchestrator.go`, `internal/orchestrator/merge.go`, `cmd/decompose/main.go`.

### M7: Graceful Degradation + Skill Integration (10 tasks)

Implements the four-level capability detection from ADR-004. Probes for A2A agents, MCP tools, and code intelligence at startup. Fallback code paths that produce valid output at each level. Updates the `/decompose` skill to detect and delegate to the Go binary. Integration tests for all four degradation levels. MCP server mode (`--serve-mcp`) exposing `run_stage`, `get_status`, and `list_decompositions` tools for Claude Code integration (PDR-004).

**Files:** `internal/orchestrator/detector_impl.go`, `internal/orchestrator/detector_test.go`, `internal/orchestrator/fallback.go`, `internal/orchestrator/fallback_test.go`, `internal/orchestrator/degradation_test.go`, `internal/mcptools/decompose_server.go`, `internal/mcptools/decompose_handlers.go`, `internal/mcptools/decompose_server_test.go`. Also MODIFY: `internal/orchestrator/pipeline.go`, `internal/orchestrator/detector.go`, `cmd/decompose/main.go`, `skill/decompose/SKILL.md`.

### M8: Testing + Release (8 tasks)

End-to-end test suite with golden path tests against a reference project. Goreleaser configuration for cross-platform binary builds. Makefile for development workflow. CI configuration. README with installation and usage docs.

**Files:** `internal/e2e/pipeline_test.go`, `internal/e2e/golden_test.go`, `testdata/golden/` (4 golden files), `.goreleaser.yml`, `Makefile`, `README.md`, `.github/workflows/ci.yml`

---

## Before Moving On

Verify before proceeding to Stage 4:

- [x] Every file from Stage 2 skeletons appears in the directory tree (all 8 skeleton files mapped)
- [x] Every feature from Stage 1 maps to at least one milestone (18 features mapped)
- [x] Every ADR/PDR is fulfilled by at least one milestone (7 ADRs + 4 PDRs mapped)
- [x] The dependency graph has no cycles (linear chain with one parallel branch: M2‖M4)
- [x] The critical path is identified (M1→M2→M3→M5→M6→M7→M8)
- [x] Parallel work is maximized where possible (M2‖M4)
- [x] Each milestone is independently testable (each has its own test files)
- [x] First milestone creates the foundation layer (M1: types + module + CLI shell)
- [x] Last milestone is the most experimental/optional feature (M8: release tooling)
