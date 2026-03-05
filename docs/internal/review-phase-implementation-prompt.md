# Implementation Prompt: Review Phase (Codebase-Plan Cross-Reference)

> Claude Code session prompt for implementing the review phase with A2A task delegation.
> Feed this prompt to a fresh Claude Code session with the progressive-decomposition repo as the working directory.

---

## Prompt

You are implementing the **review phase** for progressive decomposition -- a two-pass verification system that sits between Stage 4 completion and implementation start. The Go binary runs 5 mechanical checks (producer), then delegates an interpretive triage pass to a separate agent session via A2A (consumer).

Read these files before writing any code:

1. `docs/review-phase.md` -- the full spec for the 5 checks, output format, and integration points
2. `docs/internal/agent-parallel-design.md` -- the A2A architecture, agent roles, task lifecycle mapping, and graceful degradation tiers
3. `docs/internal/decompose-flow-analysis.md` -- flow analysis with known issues (especially Issue #7: IMPORTS edge gaps, and the direction semantics in memstore.go)
4. `internal/graph/kuzustore.go` -- the graph store interface (Store, MemStore, KuzuFileStore) and edge direction semantics
5. `internal/mcptools/unified_server.go` -- how MCP tools are registered (pattern for adding `run_review`)
6. `internal/mcptools/decompose_handlers.go` -- existing handler patterns (DecomposeService structure)
7. `cmd/decompose/main.go` -- subcommand routing and flag parsing
8. `internal/orchestrator/verification.go` -- existing directory tree regex at line ~397 (reuse for Stage 3 parsing)

### What you are building

**Package:** `internal/review/`

**New files:**

| File | Purpose |
|------|---------|
| `review.go` | Core types, `GraphProvider` interface, `RunReview` entry point |
| `parse_stage3.go` | Stage 3 directory tree parser -> `[]FileEntry` |
| `parse_stage4.go` | Stage 4 task spec parser -> `[]TaskEntry` |
| `check_existence.go` | Check 1: file existence vs plan actions (pure filesystem) |
| `check_symbols.go` | Check 2: symbol verification via graph or grep fallback |
| `check_deps.go` | Check 3: dependency completeness audit |
| `check_crossms.go` | Check 4: cross-milestone consistency (programmatic heuristics only) |
| `check_coverage.go` | Check 5: coverage gap scan (high recall, moderate precision) |
| `report.go` | Markdown report formatter |
| `review_test.go` | Unit + integration tests |
| `cmd/decompose/review.go` | CLI subcommand wiring |

**Modified files:**

| File | Change |
|------|--------|
| `cmd/decompose/main.go` | Add `review` subcommand routing, usage text, two-level implement warning |
| `internal/mcptools/unified_server.go` | Register `run_review` MCP tool |
| `internal/mcptools/decompose_handlers.go` | Add `RunReview` handler, `SetCodeIntel` method |

### Key types (internal/review/review.go)

```go
type Classification string

const (
    ClassMismatch Classification = "MISMATCH"
    ClassOmission Classification = "OMISSION"
    ClassStale    Classification = "STALE"
    ClassOK       Classification = "OK"
)

type ReviewFinding struct {
    ID             string         // "R-1.03" format
    Check          int            // 1-5
    Classification Classification
    FilePath       string         // always populated -- findings are self-locating
    TaskID         string         // optional, populated when finding relates to a specific task
    Milestone      string         // optional
    Description    string
    Suggestion     string
}

// String returns a self-locating representation:
// "R-1.03 [MISMATCH] `path/to/file.go`: File already exists, task specifies CREATE"
func (f ReviewFinding) String() string

type CheckSummary struct {
    Check      int
    Name       string
    Total      int
    Mismatches int
    Omissions  int
    Stale      int
}

type ReviewReport struct {
    Name         string
    Timestamp    string
    CommitHash   string
    GraphIndexed bool
    Checks       []CheckSummary
    Findings     []ReviewFinding
    HasMismatches bool
}

type FileEntry struct {
    Path       string
    Actions    map[string]string // milestone -> action (CREATE/MODIFY/DELETE)
    Milestones []string
}

type TaskEntry struct {
    ID         string
    Milestone  string
    File       string
    Action     string
    DependsOn  []string
    SymbolRefs []string
    Outline    string
}

type GraphProvider interface {
    Available() bool
    QuerySymbols(ctx context.Context, query string, limit int) ([]graph.SymbolNode, error)
    GetDependencies(ctx context.Context, nodeID string, direction graph.Direction, maxDepth int) ([]graph.DependencyChain, error)
    GetClusters(ctx context.Context) ([]graph.ClusterNode, error)
    AssessImpact(ctx context.Context, changedFiles []string) (*graph.ImpactResult, error)
}

type ReviewConfig struct {
    ProjectRoot    string
    DecompName     string
    DecompDir      string // docs/decompose/<name>/
    GraphProvider  GraphProvider // nil = no graph available
}
```

Finding IDs use `R-{check}.{NN}` format -- distinct from the existing verification `V-{stage}.{NN}` format.

### Parser requirements

**Stage 3 parser (`parse_stage3.go`):**
- Must handle format variation: tree-drawing characters (the box-drawing set used in `agent-parallel/stage-3-task-index.md`), plain indentation, mixed tabs/spaces
- Algorithm: strip tree-drawing characters to normalize to plain indentation, track directory stack by depth, extract file paths + `(CREATE|MODIFY|DELETE)\s*\(([^)]+)\)` annotations
- Check `internal/orchestrator/verification.go` around line 397 for the existing `directoryTreeRe` pattern -- reuse where possible, but the parser needs to be more permissive
- Handle multiple actions per line: `CREATE (M1), MODIFY (M6, M7)`

**Stage 4 parser (`parse_stage4.go`):**
- Parse `tasks_mNN.md` files
- Task headings: `**T-MM.SS -- Title**`
- File line: `` **File:** `path` (ACTION) ``
- Depends line: `**Depends on:** T-01.01, T-01.02` or `None`
- Outline section: indented bullets after `**Outline:**`
- Symbol extraction from outlines: backtick-delimited identifiers and PascalCase names

### Check implementations

**Check 1 (check_existence.go):** Pure filesystem. `os.Stat(filepath.Join(projectRoot, entry.Path))`. CREATE+exists -> MISMATCH. MODIFY+missing -> MISMATCH. DELETE+missing -> STALE.

**Check 2 (check_symbols.go):** Filter to MODIFY tasks. Extract symbol refs via `ExtractSymbolRefs` (backtick identifiers + PascalCase from outlines). With graph: `gp.QuerySymbols(ctx, name, 10)`, filter by expected file. Without graph: read file, string search. Missing in expected file but found elsewhere -> STALE. Missing everywhere -> MISMATCH.

**Check 3 (check_deps.go):** For each MODIFY target, find dependents (files that import it).

CRITICAL -- direction semantics: Before building this check, write `TestDirectionSemantics` first. The memstore.go implementation: `DirectionDownstream` matches `SourceID == id` (outgoing edges = what this file imports). `DirectionUpstream` matches `TargetID == id` (incoming edges = who imports this file). This check needs dependents, so use `DirectionUpstream`. The test creates a known A->B import edge and confirms `GetDependencies(B, DirectionUpstream, 1)` returns A.

Without graph: walk source files, regex for import/require matching the MODIFY target's filename or package name. Dependent not in plan -> OMISSION.

**Check 4 (check_crossms.go):** Find files with `len(Milestones) > 1`. Programmatic checks only:
- Conflicting actions (CREATE + DELETE on same file, MODIFY before CREATE in milestone order)
- Use `orchestrator.ParseMilestones` from scheduler.go for milestone ordering
- Overlapping symbol modifications (two tasks reference same symbol in same file) -> OMISSION
- Semantic conflicts are NOT attempted here -- deferred to the A2A interpretive pass

**Check 5 (check_coverage.go):** Determine scope boundary from planned file paths. Walk filesystem within scope for files not in plan. With graph: `GetClusters` for same-cluster files, `AssessImpact` for transitively affected. Without graph: test file naming conventions, import reference search. Intentionally high recall / moderate precision -- the interpretive session triages false positives.

### CLI integration (cmd/decompose/review.go)

Subcommand: `decompose review <name>`

Flow:
1. Verify stages 1-4 complete via `status.GetDecompositionStatus`
2. Read Stage 3 from `docs/decompose/<name>/stage-3-task-index.md`
3. Glob Stage 4 files: `docs/decompose/<name>/tasks_m*.md`
4. Attempt to open `.decompose/graph` (KuzuDB file store); wrap in `GraphProvider` or pass nil
5. Get git commit hash via `git rev-parse HEAD`
6. Call `review.RunReview(ctx, cfg)`
7. Write `docs/decompose/<name>/review-findings.md`
8. Print summary to stderr, file path to stdout

In `cmd/decompose/main.go`:
- Add routing: `if positional[0] == "review"` between the `augment` and `implement` blocks (~line 155)
- Add to `printUsage`: `decompose [flags] review <name>    Run review phase`
- Two-level implement warning:
  - `review-findings.md` doesn't exist -> warn "Review phase has not been run"
  - `review-findings.md` exists, parse summary table, mismatch count > 0 -> warn "Review found N unresolved MISMATCHes. Resolve before implementing or pass --skip-review."
  - Add `--skip-review` flag (does not block, just warns louder)

### MCP tool (single tool, all 5 checks together)

Register in `unified_server.go`:

```go
mcp.AddTool(server, &mcp.Tool{
    Name:        "run_review",
    Description: "Run the review phase: compare decomposition plan against the actual codebase. Runs 5 checks (file existence, symbol verification, dependency completeness, cross-milestone consistency, coverage gaps). Produces structured findings. Must be run after Stage 4 is complete.",
}, decomposeSvc.RunReview)
```

Pass `*CodeIntelService` to `DecomposeService` via `SetCodeIntel(ci *CodeIntelService)` -- cleanest since `DecomposeService` is constructed before codeintel is wired in `main.go`.

### A2A integration: interpretive triage task

After the Go binary writes the mechanical findings, the review phase delegates an interpretive pass to a separate agent via A2A. This is the consumer half of the producer-consumer architecture.

**A2A task structure** (follows the mapping in agent-parallel-design.md Section 3.4):

The `run_review` handler (or the CLI `review` subcommand) creates an A2A task after writing the findings file:

```
Task:
  id:           "review-interpret-{decomp-name}-{timestamp}"
  contextId:    "{decomp-name}"  // groups with other tasks for this decomposition
  status:       SUBMITTED
  kind:         "review-interpret"
  description:  "Triage mechanical review findings, resolve false positives, write recommended plan updates"

  artifacts:
    - name: "review-findings"
      path: "docs/decompose/{name}/review-findings.md"
      role: "input"
    - name: "stage-3-task-index"
      path: "docs/decompose/{name}/stage-3-task-index.md"
      role: "context"
    - name: "stage-4-tasks"
      paths: "docs/decompose/{name}/tasks_m*.md"
      role: "context"
    - name: "stage-1-design-pack"
      path: "docs/decompose/{name}/stage-1-design-pack.md"
      role: "context"

  instructions: |
    You are a review agent for progressive decomposition. You have been given
    mechanical review findings produced by the `run_review` tool. Your job is
    the interpretive pass -- work that requires reading code and understanding
    intent, not just pattern matching.

    READ FIRST:
    - docs/decompose/{name}/review-findings.md (the mechanical findings)
    - docs/decompose/{name}/stage-3-task-index.md (the milestone plan)
    - All docs/decompose/{name}/tasks_m*.md files (the task specs)
    - docs/decompose/{name}/stage-1-design-pack.md (the original design)

    YOUR TASKS:

    1. TRIAGE FALSE POSITIVES (especially from Check 5)
       For each OMISSION finding from Check 5 (coverage gap scan), read the
       unlisted file and the planned files it relates to. Determine whether
       the planned changes will actually break the unlisted file. If not,
       reclassify the finding as OK with a brief rationale.

    2. CATCH SEMANTIC CONFLICTS (Check 4 gaps)
       The mechanical Check 4 catches structural conflicts (action ordering,
       symbol overlap). You need to catch semantic conflicts: cases where two
       milestones modify the same function or module with incompatible intent.
       Read the task outlines for every multi-milestone file and assess whether
       the changes are compatible. Add new findings with classification
       MISMATCH or OMISSION as appropriate.

    3. VERIFY DEPENDENCY DIRECTION (Check 3 validation)
       For each OMISSION from Check 3, read the dependent file and the MODIFY
       target. Assess whether the planned change is backward-compatible (additive
       export, no signature change). If so, reclassify as OK. If not, confirm
       the OMISSION and suggest what task should be added.

    4. WRITE RECOMMENDED PLAN UPDATES
       Replace the <!-- INTERPRETIVE PASS NEEDED --> stub in the
       ## Recommended Plan Updates section with actionable recommendations
       grouped by milestone. Each recommendation should specify:
       - Which Stage 3 or Stage 4 file to update
       - What change to make (add task, update outline, add dependency edge,
         change action from CREATE to MODIFY, etc.)
       - Why (reference the finding ID)

    5. UPDATE THE SUMMARY TABLE
       After triage, recalculate the summary table counts. Some OMISSIONs may
       have been reclassified to OK. Some new MISMATCHes may have been added
       from semantic conflict detection.

    OUTPUT:
    Write the updated review-findings.md back to
    docs/decompose/{name}/review-findings.md, preserving the mechanical
    findings but updating classifications where your triage changed them,
    adding new findings from your semantic analysis, replacing the
    Recommended Plan Updates stub, and updating the summary table.

    Use the code intelligence MCP tools (build_graph, query_symbols,
    get_dependencies, get_clusters) if available. If not, use Read/Grep/Glob
    to examine the codebase directly.

    Do NOT modify any Stage 3 or Stage 4 files. Your output is the updated
    review-findings.md only. The human or orchestrator applies the
    recommendations.
```

**Graceful degradation for A2A:**

| Available | Behavior |
|-----------|----------|
| A2A infrastructure running | Submit task to review-interpret agent, return task ID to caller |
| A2A not available, Claude Code session available | Print the task instructions to stdout with a message: "Run this in a new Claude Code session for interpretive triage" |
| Neither | Write findings with the stub intact, print: "Mechanical review complete. Run `decompose review-interpret {name}` for interpretive triage." |

In all cases, the mechanical findings are written immediately. The A2A task is best-effort -- the review is useful without the interpretive pass, just noisier.

**Implementation in Go:**

Add a `review-interpret` subcommand to `cmd/decompose/main.go` that:
1. Reads `review-findings.md`
2. Checks for A2A agent availability (agent card discovery at localhost or configured endpoint)
3. If available: creates the A2A task via HTTP POST to the agent endpoint, returns task ID
4. If not available: prints the instructions block to stdout for manual delegation

The A2A client code follows the patterns in `agent-parallel-design.md` Section 3.4:
- Task lifecycle: SUBMITTED -> WORKING -> COMPLETED
- `contextId` groups all tasks for a decomposition
- Artifacts carry file references between agents
- `INPUT_REQUIRED` state is not expected for this task (it's autonomous)

For v1, the A2A client can be minimal: create task, poll for completion, read the updated artifact. Full streaming/SSE support is not needed for this use case since the review runs asynchronously.

### Report format (report.go)

Generate the markdown format from `docs/review-phase.md`:
- Header: review date, commit hash, graph-indexed status
- Summary table: check name x findings/mismatches/omissions/stale
- Per-check sections: `R-N.NN [CLASSIFICATION] \`filepath\`: description` (file path inline)
- `## Recommended Plan Updates`: stub with raw mismatch/omission list grouped by milestone, marked with `<!-- INTERPRETIVE PASS NEEDED -->` for the A2A consumer to fill in

### Implementation order

| Step | Files | Depends on |
|------|-------|-----------|
| 1 | `internal/review/review.go` -- types, GraphProvider, RunReview shell | -- |
| 2 | `internal/review/parse_stage3.go` -- permissive directory tree parser | 1 |
| 3 | `internal/review/parse_stage4.go` -- task spec parser | 1 |
| 4 | `internal/review/check_existence.go` -- Check 1 | 1, 2 |
| 5 | `internal/review/check_symbols.go` -- Check 2 | 1, 3 |
| 6 | `internal/review/review_test.go` -- TestDirectionSemantics (validate before Check 3) | 1 |
| 7 | `internal/review/check_deps.go` -- Check 3 | 1, 2, 3, 6 |
| 8 | `internal/review/check_crossms.go` -- Check 4 | 1, 2, 3 |
| 9 | `internal/review/check_coverage.go` -- Check 5 | 1, 2 |
| 10 | `internal/review/report.go` -- markdown formatter | 1 |
| 11 | `internal/review/review_test.go` -- remaining unit + integration tests | 1-10 |
| 12 | `cmd/decompose/review.go` -- CLI subcommand (both review and review-interpret) | 1-10 |
| 13 | `cmd/decompose/main.go` -- routing + implement warning | 12 |
| 14 | `internal/mcptools/` -- MCP tool registration + handler | 1-10 |
| 15 | A2A client for review-interpret task submission | 12 |

Steps 2-3 can run in parallel. Steps 4, 5, 8, 9 can run in parallel after their deps complete.

### Verification

1. **Direction semantics:** `TestDirectionSemantics` -- MemStore with A->B import edge, verify `GetDependencies(B, DirectionUpstream, 1)` returns A. Must pass before Check 3 is built.
2. **Parser tests:** Table-driven with both tree-character and plain-indent formats, multi-action lines, empty trees, nested dirs.
3. **Check tests:** Table-driven per check using temp dirs + MemStore. Check 1: CREATE+exists -> MISMATCH. Check 3: dependent not in plan -> OMISSION. Check 4: MODIFY before CREATE -> MISMATCH.
4. **Integration test:** Temp project with Stage 3/4 markdown + graph, run full `RunReview`, assert expected findings.
5. **Manual test:** `decompose review agent-parallel` against the existing decomposition (will produce findings since codebase has evolved).
6. **Implement warning test:** Verify `runImplement` warns when review-findings.md has mismatches.
7. **A2A test:** Mock A2A server, verify task creation with correct artifacts and instructions.
8. **Build:** `go build ./...` compiles.
