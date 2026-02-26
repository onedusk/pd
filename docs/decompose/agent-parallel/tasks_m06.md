# Stage 4: Task Specifications — Milestone 6: Orchestrator Pipeline

> Implements the `Orchestrator` interface. Stage routing, fan-out/fan-in via errgroup, section-based merge, coherence checking, and progress event emission.
>
> Fulfills: PDR-001 (milestone-level progress), Stage 1 features — orchestrator engine, section-based merge, decomposition status

---

- [ ] **T-06.01 — Implement stage router**
  - **File:** `internal/orchestrator/router.go` (CREATE)
  - **Depends on:** T-01.08
  - **Outline:**
    - Define `Router` struct holding a `Config` and a map of `Stage` → `StageExecutor`
    - `StageExecutor` interface: `Execute(ctx context.Context, cfg Config, inputs []StageResult) (*StageResult, error)`
    - `Route(ctx context.Context, stage Stage) (*StageResult, error)`:
      - Check prerequisites: Stage 1 requires Stage 0 file exists (warn if missing, don't block). Stage 2 requires Stage 1 output. Stage 3 requires Stages 1+2. Stage 4 requires Stage 3.
      - Read prerequisite stage files from `cfg.OutputDir`
      - Delegate to the appropriate `StageExecutor`
    - `RouteRange(ctx context.Context, from, to Stage) ([]StageResult, error)`:
      - Execute stages sequentially from `from` to `to`, passing outputs forward
    - Prerequisites check: verify files exist on disk at expected paths (`stage-{N}-{name}.md` or `tasks_mNN.md`)
  - **Acceptance:** Router correctly identifies missing prerequisites and returns descriptive errors. Stage 0 missing produces a warning, not an error. Stages execute in order when using `RouteRange`.

---

- [ ] **T-06.02 — Write stage router tests**
  - **File:** `internal/orchestrator/router_test.go` (CREATE)
  - **Depends on:** T-06.01
  - **Outline:**
    - Use temp directories with mock stage files
    - Test: Route Stage 1 with Stage 0 present → succeeds
    - Test: Route Stage 1 without Stage 0 → warning logged, proceeds
    - Test: Route Stage 2 without Stage 1 → error with clear message
    - Test: RouteRange 1→3 → executes 3 stages in order
    - Test: RouteRange with failure at Stage 2 → stops, returns error, Stage 3 not attempted
    - Mock `StageExecutor` using testify/mock
  - **Acceptance:** `go test ./internal/orchestrator/ -run TestRouter` passes. Prerequisite checking is correct for all stage combinations.

---

- [ ] **T-06.03 — Implement fan-out engine**
  - **File:** `internal/orchestrator/fanout.go` (CREATE)
  - **Depends on:** T-04.01, T-01.08
  - **Outline:**
    - Define `FanOut` struct holding an `a2a.Client` and progress callback
    - `Run(ctx context.Context, tasks []AgentTask) ([]AgentResult, error)`:
      - `AgentTask` struct: `AgentEndpoint string`, `Message a2a.Message`, `Section string`
      - `AgentResult` struct: `Section string`, `Artifacts []a2a.Artifact`, `Err error`
      - Use `golang.org/x/sync/errgroup` with context — cancels all on first error
      - Launch one goroutine per `AgentTask`
      - Each goroutine: call `client.SendMessage(ctx, endpoint, req)` with `Blocking: true`
      - Collect results via channel or slice (mutex-protected)
      - Emit `ProgressEvent` when each task completes
    - Handle `INPUT_REQUIRED` state: if a task returns `TaskStateInputRequired`, emit a progress event and return the task — orchestrator must route the question to the user
    - Timeout: respect context deadline; if no explicit timeout, use 5-minute default per task
  - **Acceptance:** Given 3 mock agents, `FanOut.Run` dispatches tasks in parallel and collects all results. If one agent fails, all others are canceled. Progress events are emitted for each completion.

---

- [ ] **T-06.04 — Write fan-out engine tests**
  - **File:** `internal/orchestrator/fanout_test.go` (CREATE)
  - **Depends on:** T-06.03
  - **Outline:**
    - Mock `a2a.Client` with testify/mock
    - Test: 3 tasks, all succeed → 3 results collected
    - Test: 3 tasks, second fails → all canceled, error returned
    - Test: 1 task returns INPUT_REQUIRED → result captured with that state
    - Test: context cancellation during fan-out → all goroutines terminate
    - Test: progress events emitted in order of completion (not submission)
    - Verify no goroutine leaks via race detector
  - **Acceptance:** `go test ./internal/orchestrator/ -run TestFanOut` passes with `-race`. Parallel dispatch, error cancellation, and progress events all work correctly.

---

- [ ] **T-06.05 — Implement section-based merge**
  - **File:** `internal/orchestrator/merge_impl.go` (CREATE)
  - **Depends on:** T-01.08
  - **Outline:**
    - Define `Merger` struct holding a `MergePlan`
    - `Merge(sections []Section) (string, error)`:
      - Validate: every section name in `MergePlan.SectionOrder` has a corresponding `Section`
      - Sort sections by `MergePlan.SectionOrder` (template order)
      - Concatenate section content with `---` separators
      - Return the merged markdown string
    - Handle missing sections: return error listing which expected sections are absent
    - Handle duplicate sections: return error — each section should come from exactly one agent
    - Define per-stage merge plans as constants:
      - Stage 1: `["assumptions", "platform-baseline", "data-model", "architecture", "features", "integrations", "security", "adrs", "pdrs", "prd", "data-lifecycle", "testing", "implementation-plan"]`
      - Stage 2: `["data-model-code", "interface-contracts", "documentation"]`
      - Stage 3: `["progress", "dependencies", "directory-tree"]`
      - Stage 4: one section per milestone (no merge needed — separate files)
  - **Acceptance:** Given 5 sections in random order with a merge plan, `Merge` produces them in template order. Missing sections cause errors. Duplicate sections cause errors.

---

- [ ] **T-06.06 — Write merge tests**
  - **File:** `internal/orchestrator/merge_test.go` (CREATE)
  - **Depends on:** T-06.05
  - **Outline:**
    - Test: 3 sections in order → concatenated correctly with separators
    - Test: 3 sections out of order → reordered to template order
    - Test: missing section → error listing the missing section name
    - Test: duplicate section name → error
    - Test: extra section not in plan → included at the end (or error — decide)
    - Test: empty section content → included (empty sections are valid)
  - **Acceptance:** `go test ./internal/orchestrator/ -run TestMerge` passes. Section ordering, validation, and error reporting all work.

---

- [ ] **T-06.07 — Implement coherence checker**
  - **File:** `internal/orchestrator/coherence.go` (CREATE)
  - **Depends on:** T-06.05
  - **Outline:**
    - Define `CheckCoherence(merged string, sections []Section) ([]CoherenceIssue, error)`
    - Lightweight cross-section consistency scan:
      - Extract version numbers from each section (regex: semver pattern `\d+\.\d+\.\d+`)
      - Check: same dependency mentioned in multiple sections with different versions → flag as issue
      - Extract technology/framework names and check for contradictions (e.g., one section says "PostgreSQL", another says "SQLite" for the same role)
    - This is NOT an LLM call — it's a deterministic pattern-matching check
    - Return `[]CoherenceIssue` (empty if no contradictions found)
    - Issues are advisory — the orchestrator logs them but does not block output
  - **Acceptance:** Given two sections mentioning "React 18.2" and "React 19.0", checker flags a version mismatch. Given consistent sections, checker returns empty issues list. Does not false-positive on version numbers in code examples.

---

- [ ] **T-06.08 — Write coherence checker tests**
  - **File:** `internal/orchestrator/coherence_test.go` (CREATE)
  - **Depends on:** T-06.07
  - **Outline:**
    - Test: two sections with matching versions → no issues
    - Test: two sections with conflicting versions (React 18 vs React 19) → 1 issue
    - Test: version number inside a code block → not flagged (avoid false positives)
    - Test: no version numbers in any section → no issues
    - Test: same technology name, different contexts (acceptable) → no issues
  - **Acceptance:** `go test ./internal/orchestrator/ -run TestCoherence` passes. True contradictions are caught. Code examples don't trigger false positives.

---

- [ ] **T-06.09 — Implement progress reporter**
  - **File:** `internal/orchestrator/progress.go` (CREATE)
  - **Depends on:** T-01.08
  - **Outline:**
    - Define `ProgressReporter` struct with a `chan ProgressEvent` (buffered, size 64)
    - `Emit(event ProgressEvent)` — non-blocking send on channel (drop if full, log warning)
    - `Subscribe() <-chan ProgressEvent` — returns the read-only channel
    - `Close()` — closes the channel
    - Console formatter: `FormatProgress(event ProgressEvent) string`
      - Pending: `"  ○ {section} (pending)"`
      - Working: `"  ● {section}..."`
      - Complete: `"  ✓ {section} complete"`
      - Failed: `"  ✗ {section} failed: {message}"`
    - Stage header: `"[{decomposition}] Stage {N}: {name}"`
  - **Acceptance:** `Emit` does not block even if channel is full. `FormatProgress` produces the expected format strings from Stage 1's progress output example. Channel closes cleanly.

---

- [ ] **T-06.10 — Implement pipeline orchestrator**
  - **File:** `internal/orchestrator/pipeline.go` (CREATE), `internal/orchestrator/pipeline_test.go` (CREATE), `cmd/decompose/main.go` (MODIFY)
  - **Depends on:** T-06.01, T-06.03, T-06.05, T-06.07, T-06.09, T-05.09
  - **Outline:**
    - Define `Pipeline` struct implementing `Orchestrator` interface
    - Constructor: `NewPipeline(cfg Config, client a2a.Client, registry *agent.Registry) *Pipeline`
    - `RunStage(ctx, stage)`:
      1. Emit stage header progress event
      2. Check prerequisites via Router
      3. Based on `cfg.Capability`:
         - `CapFull` / `CapA2AMCP`: spawn agents via Registry, build section assignments per stage, fan out via FanOut, merge via Merger, check coherence, write output file
         - `CapMCPOnly` / `CapBasic`: delegate to fallback (M7)
      4. Write merged output to `cfg.OutputDir/stage-{N}-{name}.md` (or `tasks_mNN.md` for Stage 4)
      5. Emit completion progress event
      6. Return `StageResult` with file paths and sections
    - `RunPipeline(ctx, from, to)`: delegate to Router.RouteRange
    - `Progress()`: return ProgressReporter channel
    - Stage 4 special handling: fan out one Task Writer per milestone (parallel), write separate files
    - `cmd/decompose/main.go` MODIFY: wire `NewPipeline` in `run()`, parse name and stage from args, invoke `RunStage` or `RunPipeline`, print progress to stderr
  - **Acceptance:** `Pipeline` satisfies `Orchestrator` interface. `RunStage` with `CapFull` produces output files in the correct directory. Stage 4 produces separate `tasks_mNN.md` files. Progress events are emitted. CLI invocation `decompose myproject 1` triggers Stage 1. `go test ./internal/orchestrator/ -run TestPipeline` passes with mock agents.
