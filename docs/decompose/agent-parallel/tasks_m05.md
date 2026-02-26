# Stage 4: Task Specifications — Milestone 5: Specialist Agents

> Implements the four specialist agents on top of M4's base agent framework. Each agent declares its Agent Card, connects to MCP tools, and handles A2A tasks.
>
> Fulfills: Stage 1 features — Research Agent, Schema Agent, Planning Agent, Task Writer Agent

---

- [ ] **T-05.01 — Implement Research Agent**
  - **File:** `internal/agent/research.go` (CREATE)
  - **Depends on:** T-04.08
  - **Outline:**
    - Define `ResearchAgent` struct embedding `BaseAgent`
    - Agent Card skills: `research-platform`, `verify-versions`, `explore-codebase` (from Stage 2 example payload)
    - Default input/output modes: `text/plain` input, `text/markdown` output
    - `ProcessMessage` implementation:
      - Parse the incoming message to determine which skill is requested (match against skill IDs in the message text or metadata)
      - `research-platform`: Read project files to identify frameworks, use MCP web search tool (if available) to verify versions, produce a "Platform & Tooling Baseline" markdown section
      - `verify-versions`: Cross-check version numbers from the design pack against official sources via web search
      - `explore-codebase`: Walk the project directory, identify patterns (directory structure, config files, existing tests), produce a codebase exploration summary
    - MCP client connection: connect to web search MCP server (Firecrawl or similar) and file system MCP server at startup. If MCP tools aren't available, fall back to direct file reading and no web search.
    - Output: `[]a2a.Artifact` with one artifact per section, markdown content in text parts
    - Report token usage in artifact metadata: `{"tokenCount": N}`
  - **Acceptance:** `ResearchAgent` starts and serves Agent Card with 3 skills. Given a message requesting `explore-codebase`, it produces an artifact with markdown describing the project structure. Works without MCP tools (direct file reading fallback).

---

- [ ] **T-05.02 — Write Research Agent tests**
  - **File:** `internal/agent/research_test.go` (CREATE)
  - **Depends on:** T-05.01
  - **Outline:**
    - Test `explore-codebase` skill against `testdata/fixtures/go_project/`:
      - Assert artifact contains file listing
      - Assert artifact mentions detected language (Go)
    - Test `research-platform` without MCP tools (fallback mode):
      - Assert produces a section even without web search
    - Test Agent Card: verify skills, input/output modes, version
    - Test unknown skill request: returns FAILED task with descriptive error
    - Use `httptest` to run agent on random port, `HTTPClient` to send tasks
  - **Acceptance:** `go test ./internal/agent/ -run TestResearch` passes. Agent handles known skills and rejects unknown ones. Fallback mode produces output without MCP.

---

- [ ] **T-05.03 — Implement Schema Agent**
  - **File:** `internal/agent/schema.go` (CREATE)
  - **Depends on:** T-04.08
  - **Outline:**
    - Define `SchemaAgent` struct embedding `BaseAgent`
    - Agent Card skills: `translate-schema`, `validate-types`, `write-contracts`
    - Default input/output modes: `text/plain` + `application/json` input, `text/markdown` output
    - `ProcessMessage` implementation:
      - `translate-schema`: Parse data model description from message, generate compilable type definitions in the target language, return as markdown code blocks
      - `validate-types`: If MCP language server tool is available, run type checking on provided code. Otherwise, verify syntax by attempting to parse with Tree-sitter.
      - `write-contracts`: Generate request/response type definitions for API surfaces described in the message
    - MCP client connection: language server MCP tool, AST parser (Tree-sitter MCP tool from M3), linter. Falls back to pattern-based generation without tools.
    - Output: artifacts with compilable code in markdown code blocks
  - **Acceptance:** `SchemaAgent` starts and serves Agent Card with 3 skills. Given a `translate-schema` message with entity descriptions, produces artifact containing Go struct definitions. Code in output is syntactically valid Go.

---

- [ ] **T-05.04 — Write Schema Agent tests**
  - **File:** `internal/agent/schema_test.go` (CREATE)
  - **Depends on:** T-05.03
  - **Outline:**
    - Test `translate-schema`: input is "Entity User with fields name (string), age (int)", output contains `type User struct` with correct fields
    - Test `write-contracts`: input describes an API endpoint, output contains request/response structs
    - Test `validate-types` fallback (no MCP tools): returns validation report noting tools unavailable
    - Test Agent Card: skills, modes
    - Use `httptest` + `HTTPClient`
  - **Acceptance:** `go test ./internal/agent/ -run TestSchema` passes. Generated code is syntactically valid (parseable by Tree-sitter).

---

- [ ] **T-05.05 — Implement Planning Agent**
  - **File:** `internal/agent/planning.go` (CREATE)
  - **Depends on:** T-04.08, T-03.03
  - **Outline:**
    - Define `PlanningAgent` struct embedding `BaseAgent`
    - Agent Card skills: `build-code-graph`, `analyze-dependencies`, `assess-impact`, `plan-milestones`
    - Default input/output modes: `text/plain` + `application/json` input, `text/markdown` + `application/json` output
    - `ProcessMessage` implementation:
      - `build-code-graph`: Call MCP tool `build_graph` with the project path from the message. Return graph stats as artifact.
      - `analyze-dependencies`: Call MCP tool `get_dependencies` for specified files/symbols. Return dependency chains as artifact.
      - `assess-impact`: Call MCP tool `assess_impact` for specified files. Return blast radius as artifact.
      - `plan-milestones`: Given a design pack (text artifact from Stage 1) and graph data, organize work into dependency-ordered milestones. Produce Stage 3 task index format: milestone list, dependency graph, directory tree.
    - MCP client connection: connect to code intelligence MCP server (from M3). `plan-milestones` uses graph data from previous tool calls (passed via task artifacts or reference task IDs).
    - Fallback without MCP: `plan-milestones` still works using file-reading heuristics (list files, infer dependencies from import statements in file content)
  - **Acceptance:** `PlanningAgent` starts with 4 skills. `build-code-graph` calls MCP tool and returns stats. `plan-milestones` produces markdown with milestone list and dependency graph. Works in fallback mode (no MCP) with reduced accuracy.

---

- [ ] **T-05.06 — Write Planning Agent tests**
  - **File:** `internal/agent/planning_test.go` (CREATE)
  - **Depends on:** T-05.05
  - **Outline:**
    - Test `build-code-graph` with MCP: use in-memory MCP transport, code intelligence MCP server with `MemStore`. Assert artifact contains graph stats.
    - Test `plan-milestones`: provide a simple design pack text, assert output contains milestone list and ASCII dependency graph
    - Test fallback mode (no MCP tools): `build-code-graph` returns error explaining tools unavailable, `plan-milestones` falls back to heuristics
    - Test Agent Card: skills, modes
  - **Acceptance:** `go test ./internal/agent/ -run TestPlanning` passes. Agent correctly delegates to MCP tools when available and falls back gracefully.

---

- [ ] **T-05.07 — Implement Task Writer Agent**
  - **File:** `internal/agent/taskwriter.go` (CREATE)
  - **Depends on:** T-04.08
  - **Outline:**
    - Define `TaskWriterAgent` struct embedding `BaseAgent`
    - Agent Card skills: `write-task-specs`, `validate-dependencies`
    - Default input/output modes: `text/plain` + `text/markdown` input, `text/markdown` output
    - `ProcessMessage` implementation:
      - `write-task-specs`: Given a milestone section from Stage 3 (passed as message text), produce a complete `tasks_mNN.md` file. For each file in the milestone:
        - Assign task ID `T-{MM}.{SS}`
        - Determine file action (CREATE/MODIFY/DELETE)
        - Write implementation outline naming types, methods, and parameters
        - Write binary acceptance criteria
        - Order tasks so dependencies come first
      - `validate-dependencies`: Given multiple task files (as artifacts or text), check cross-milestone dependencies. Verify all referenced task IDs exist. Report any missing or circular dependencies.
    - MCP tools: codebase search (if available) to reference existing code in task outlines
    - Output: one artifact containing the complete task file in markdown
    - Multiple instances of Task Writer can run in parallel (one per milestone) — no shared state beyond BaseAgent
  - **Acceptance:** `TaskWriterAgent` starts with 2 skills. Given a milestone description, `write-task-specs` produces a markdown file with tasks in `T-MM.SS` format. Each task has file path, action, dependencies, outline, and acceptance criteria.

---

- [ ] **T-05.08 — Write Task Writer Agent tests**
  - **File:** `internal/agent/taskwriter_test.go` (CREATE)
  - **Depends on:** T-05.07
  - **Outline:**
    - Test `write-task-specs`: input is a milestone with 3 files, assert output contains 3+ tasks with correct ID format
    - Test task ordering: if file B depends on file A, T for A comes before T for B
    - Test `validate-dependencies`: input has cross-milestone reference T-01.03, no task file for M1 → reports missing reference
    - Test `validate-dependencies`: input has circular dependency → reports cycle
    - Test Agent Card: skills, modes
  - **Acceptance:** `go test ./internal/agent/ -run TestTaskWriter` passes. Task specs follow the `T-MM.SS` format. Dependency validation catches missing references and cycles.

---

- [ ] **T-05.09 — Add agent spawn registry**
  - **File:** `internal/agent/registry.go` (CREATE)
  - **Depends on:** T-05.01, T-05.03, T-05.05, T-05.07
  - **Outline:**
    - Define `Registry` struct: maps `Role` → agent constructor function
    - `NewRegistry() *Registry` — pre-registers all 4 specialist agents:
      - `RoleResearch` → `NewResearchAgent`
      - `RoleSchema` → `NewSchemaAgent`
      - `RolePlanning` → `NewPlanningAgent`
      - `RoleTaskWriter` → `NewTaskWriterAgent`
    - `SpawnAll(ctx context.Context, basePort int) ([]Agent, error)` — creates each agent, assigns sequential ports (`basePort`, `basePort+1`, ...), starts each agent
    - `StopAll(ctx context.Context) error` — stops all spawned agents
    - Used by the orchestrator (M6) to spawn local agents
  - **Acceptance:** `SpawnAll` starts 4 agents on sequential ports. Each agent's Agent Card is discoverable at `http://localhost:{port}/.well-known/agent-card.json`. `StopAll` cleanly shuts down all agents.
