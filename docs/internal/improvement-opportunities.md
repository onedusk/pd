# Improvement Opportunities

**Date:** 2026-06-18
**Status:** Active — findings + prioritized plan
**Prerequisites:** `architecture-assessment.md` (2026-03-05), `recommendations.md` (2026-03-13)
**Method:** Multi-agent audit — 9 finders (6 subsystem-deep + 3 cross-cutting lenses)
fanned out over the repo, each finding adversarially re-verified against the source and
against the existing internal docs. 75 raw findings → **74 survived verification** (1 refuted).
84 agents, ~4.3M tokens. Every claim below carries a `file:line` citation; the headline
correctness bug and the featured doc fixes were additionally re-checked by hand.

---

## Reading this document

This is a *net-new* improvement record, written deliberately to **not re-litigate** decisions
already captured in `recommendations.md`. Each finding is tagged:

- **net-new** — not covered by any existing internal doc (50 of 74).
- **drift-from-decided** — a prior recommendation that was never implemented (24 of 74).

> **Scope change since `recommendations.md` (important).** That document's **Recommendation 3**
> decided to *shelve* the multi-agent stack (`internal/a2a`, `internal/agent`,
> `internal/orchestrator`). **That decision has been reversed: A2A is being re-included and
> kept.** The chosen scope is **documentation realignment only** — make the docs reflect that
> A2A is a supported part of the product; do **not** scope runtime-activation work right now.
> Consequently, `recommendations.md` is now **partially superseded**: its Rec 3 is void, and its
> dependent recommendations (Rec 5 "reduce the binary," and the A2A-removal half of Rec 1) need
> **explicit reconfirmation** rather than being treated as settled. Findings that previously read
> "delete this dead A2A code" are reframed below as either (a) docs accuracy fixes or (b) the
> well-tested kept code it now is. The ~13k lines of A2A/agent/orchestrator are no longer
> "dead weight to remove"; their strong test coverage is a **revival asset**.

---

## Executive summary

The methodology and skill layer are in good shape; the issues concentrate in the **Go binary**.
Six themes, in rough priority order:

1. **One real correctness bug in kept, user-facing code** — `decompose review`'s blast-radius
   check computes the *opposite* of what it claims (graph-1). Plus a cluster of smaller
   correctness traps in the review parsers and the graph's TypeScript/`.tsx` handling.
2. **Test coverage is mal-distributed.** The code users actually exercise — the CLI (0%), the
   review checks (3 of 5 checks + `RunReview` + report rendering at literal 0%), and several
   kept packages (`config`/`status`/`export` at 0%) — is the *least* tested, while the
   best-tested packages are internal infrastructure. This is the single largest structural risk.
3. **Documentation drift and internal contradiction.** The README still markets a turnkey
   multi-agent product that does not actually function out of the box; several docs disagree
   with each other and with the code; the documented install path and quick-start links are
   broken.
4. **Onboarding is broken at the front door.** `go install …@latest` resolves to nothing (no
   tags, no release pipeline), the README never mentions the recommended entry point
   (`decompose init` / the `/decompose` skill), and the quick-start link 404s.
5. **A2A is kept but incomplete at runtime** (agents can't be hosted by any shipped command;
   `CapFull` is statically unreachable; SSE is a stub). Per the chosen scope these are recorded
   as **accuracy/known-gap notes**, not scoped work — but the README must stop implying they work.
6. **Maintainability and architecture clarity** — duplicate extractors, dead constructors,
   tripled template/guide copies, and a couple of cross-package couplings.

The fastest wins are a batch of **small, high-value fixes**: flip one bug, correct ~8 broken doc
references, and make a handful of CLI/parse edge cases fail loudly instead of silently.

---

## Theme 1 — Correctness in kept code

### ⛔ graph-1 (high / effort S) — `decompose review` blast-radius is inverted — **fixed 2026-09-18**
`KuzuStore.AssessImpact` walks `DirectionDownstream` (`kuzustore.go:348,357`), but `fileNeighbors`
defines downstream as `(a:File{path})-[:IMPORTS]->(b)` — the files the changed file *imports*
(its dependencies), not the files that import it (its dependents). The doc comment
(`kuzustore.go:336`) claims the opposite. `MemStore.AssessImpact` (`memstore.go:191-199`) does the
*correct* thing, so the two `Store` implementations return **opposite sets for the same graph**.
The split reaches production: the MCP `assess_impact` tool runs on `MemStore` (correct), while the
`decompose review` coverage check (Check 5) runs on `KuzuFileStore` (`cmd/decompose/review.go:38` →
`check_coverage.go:169`) and is **inverted**. The bug is locked in by a test that asserts the wrong
direction (`kuzustore_test.go:289-327`). **Fix:** switch both calls to `DirectionUpstream`, rewrite
the tests that codify the inversion, and add a cross-store conformance test (see graph-3). *Verified
by hand.*

### review-4 (medium / M) — Stage 3 and Stage 4 run in different path namespaces
Stage 3 entry paths pass through `stripRootLabel` (`parse_stage3.go:88`); Stage 4 `task.File`
paths are taken verbatim (`parse_stage4.go:91-94`) with no normalization. Checks that compare the
two (e.g. `sym.FilePath == task.File`) silently mismatch when a project uses a root label.
**Fix:** canonicalize both stages through one function.

### review-3 (medium / M) — `stripRootLabel` is an all-or-nothing heuristic that corrupts paths
If every parsed entry shares a first path component, it is stripped as a "project label"
(`parse_stage3.go:216-244`) — indistinguishable from a legitimate common root. For
single-root decompositions this silently rewrites every path. **Fix:** strip only a literal known
label (match the decomposition name) or verify the stripped path exists before committing.

### review-6 (medium / M) — the MISMATCH gate fails *open*
The CLI's pass/fail gate re-parses the rendered Markdown report by splitting the `**Total**` row on
`|` and hard-coding the mismatch column (`report.go:170-197`); it returns `0` (clean) whenever the
row is absent or non-numeric. Any format drift makes `decompose review` report success on a broken
plan. 0% covered. **Fix:** read `MismatchCount()` off the in-memory report instead of re-parsing,
or make it fail closed.

### graph-6 (medium / M) — `.tsx` parsed with the non-JSX grammar; `.js/.jsx` never indexed
`.tsx` maps to `LangTypeScript` (`handlers.go:37`) but the parser registers only the plain
TypeScript grammar, never `LanguageTSX()` (`treesitter.go:33`) — which is vendored and available.
JSX is parsed via error-recovery, degrading symbol extraction on exactly the React/Next.js
codebases the methodology targets. Separately, `.js/.jsx/.mjs/.cjs` are absent from the language
map, yet the resolver probes for them (`resolve.go:104`), so those probe extensions are dead.
**Fix:** register `LanguageTSX()` and route `.tsx` to it; decide deliberately whether to index JS.

### graph-2 (medium / M) — `CALLS` edges are malformed and consumed by nothing
All four extractors emit `CALLS` edges with `SourceID` set to the *file path* instead of a symbol
id (`treesitter_go.go:196-200` et al.). `KuzuStore`'s insert requires both endpoints to be symbol
ids, so the `MATCH` no-ops and the `CALLS` table is **always empty**; `MemStore` stores them
verbatim as garbage. No consumer reads `CALLS` anywhere. Result: wasted parse/persist work and an
`EdgeCount` that diverges between the two stores. **Fix:** drop `CALLS` extraction until a real
call-graph feature exists, or make it emit the enclosing symbol id.

### Smaller correctness traps (low)
| ID | Effort | Issue & fix |
|----|:--:|------|
| review-8 | S | Stage 4 parser returns zero tasks on heading-format drift → false all-clear for Checks 2/3/4 (`parse_stage4.go:67-70`). Warn when a matched file parses to zero tasks. |
| review-10 | S | Extensionless files with no inline action (`Makefile`, `Dockerfile`, `LICENSE`) are misclassified as directories and dropped from the plan (`parse_stage3.go:150-153`). |
| graph-9 | S | Parse/persist errors are swallowed with bare `return nil`/`continue` and no counter (`handlers.go:109-122,227-242`); a file that fails to parse vanishes with zero signal. Count and report skips. |
| status-2 | S | `NextStage` uses `max+1`, mis-reporting the next stage when there's a gap (stages 0,1,3 present → reports 4) (`status/status.go:53-68`). Return the earliest gap. |
| impl-1 | S | `decompose implement` requires Stage 0, but Stage 0 is the org-shared root file `review` explicitly treats as optional (`main.go:293-297`). Skip Stage 0 like `runReview` does. |
| graph-3 | M | `MemStore` and `KuzuStore` disagree on `maxDepth<=0` defaults and transitive-closure caps behind one "interchangeable" interface, with no conformance suite. Add a table-driven cross-store equivalence test. |
| agent-4 | M | `decompose implement` (reachable) attributes the **whole repo's** `git diff` to each milestone (`implementer.go:130-194`); with `--max-concurrent >1`, concurrent milestones cross-contaminate each other's artifacts. Scope the diff or isolate per worktree. |

---

## Theme 2 — Test coverage (the largest structural risk)

Real per-package coverage:

| Package | Coverage | Role |
|---|:--:|---|
| `cmd/decompose` | **0.0%** | the CLI users invoke |
| `internal/config` | **0.0%** | consumed by CLI |
| `internal/export` | **0.0%** | `decompose export` |
| `internal/status` | **0.0%** | review/implement gates |
| `internal/skilldata` | none | embedded assets |
| `internal/review` | **37.0%** | core kept value (3 of 5 checks at literal 0%) |
| `internal/mcptools` | **41.7%** | `run_review` handler at 0% |
| `internal/agent` | 67.6% | A2A (kept) |
| `internal/graph` | 70.2% | core kept value |
| `internal/orchestrator` | 74.6% | A2A (kept) |
| `internal/a2a` | 81.8% | A2A (kept) |

The pattern (test-1, test-2, test-7, review-7): **the surfaces users actually exercise are the
least tested.** With A2A now kept, its high coverage is appropriate — the problem is purely the
*under-tested kept core*, not "wasted investment." Concretely:

- **test-2 / review-7 (high / L):** `CheckSymbols`, `CheckDependencyCompleteness`, `CheckCoverageGaps`,
  `RunReview`, `report.go`, and both stage loaders are at **0%**. These are the product's promoted
  value. Add table-driven tests using the existing `graph.NewMemStore` plus a temp-dir filesystem,
  covering both the graph-backed and fallback branches.
- **test-4 (medium / M):** `cmd/decompose` routing is entirely untested. Add black-box tests on
  `run([]string)` capturing stdout/stderr/error for the common subcommands and error cases.
- **mcp-3 / mcp-4 / test-7 (medium):** the **production** unified MCP server has no test (the only
  server tests pin two *dead* constructors — see mcp-2); `run_review` and `generate_diagram`
  handlers are at 0%. Add a test wiring an in-memory client to `NewUnifiedMCPServer`.
- **test-8 (medium / S):** `config`/`status`/`export` (0%) are consumed directly by the CLI and the
  review/implement gates. Small unit tests would cover high-risk mechanical paths cheaply.
- **test-3 (medium / M):** the entire e2e/golden suite exercises only the orchestrator pipeline; the
  kept product (build_graph → run_review) has **zero** end-to-end coverage. Add a kept-path e2e over
  `testdata/fixtures/go_project`.
- **test-9 (low / S):** several A2A tests use sleep-based synchronization (flaky); replace fixed
  sleeps with channel/condition sync if they're to be relied on long-term.

---

## Theme 3 — Documentation drift & internal contradiction

Now that A2A is **kept**, the realignment direction is: make every doc agree that A2A is a
supported (if currently runtime-incomplete) capability, and fix the broken references.

| ID | Sev/Eff | Issue | Realigned action |
|----|:--:|------|------|
| drift-1d / agent-7 | high/L | Docs internally contradict on A2A's status: `recommendations.md` Rec 3 says "shelve," CLAUDE.md says "archived," code keeps it live. | **Retract Rec 3 explicitly** (A2A kept), and change CLAUDE.md "archived" → "supported / runtime-incomplete." |
| drift-3d | med/M | README markets A2A as a turnkey headline product (capability levels, `--agents`). | Keep A2A in the README but describe it **accurately** as an A2A *client* requiring external agent endpoints / experimental, not turnkey (see Theme 5). |
| drift-4d | med/S | CLAUDE.md MCP config example uses `"args":["mcp"]`; **no `mcp` subcommand exists** — real flag is `--serve-mcp`. Breaks on copy-paste. *Verified by hand.* | Change to `["--project-root","<abs>","--serve-mcp"]`; use one canonical invocation in CLAUDE.md, README, and `.mcp.json`. |
| drift-7d | med/S | Two divergent copies of `recommendations.md` (`docs/` vs `docs/internal/`) with conflicting decided content. | Keep `docs/internal/` as canonical; delete the root copy. |
| drift-10d | med/S | README "Contents" tree omits `cmd/`, `internal/`, `docs/` (the entire binary) and its `process-guide.md` links 404. | Update the tree; fix links to `docs/process-guide.md`. |
| drift-5d | med/S | CLAUDE.md still steers the `/decompose` skill to `get_stage_context`/`write_stage`/`get_status` (SKILL.md was already cleaned). | *Rec 1 decision — reconfirm (see below).* If keeping the cleanup, remove these from CLAUDE.md:104-107. |
| drift-6d / onboard-8 | low/S | CLAUDE.md:87 points to `docs/internal/agent-parallel-design.md`; the file is at `docs/archive/agent-parallel-design.md` (gitignored). | Fix the path; decide whether the archive should be tracked. |
| drift-9d | low/S | The only complete worked-example decomposition shipped (`docs/decompose/agent-parallel/`) is the A2A design itself. | With A2A kept this is fine; optionally add a kept-capability example. |
| drift-8d | med/L | The gating "dusk evaluation" (Rec 4) never ran, so Open Questions 1–5 in `recommendations.md` are permanently unanswered. | Either run it or downgrade the evidence bar so dependent work can proceed; document the stall. |
| config-1 | med/M | Four `decompose.yml` fields (`Languages`, `ExcludeDirs`, `TemplatePath`, `GraphExcludes`) are parsed and documented but **never consumed** (`config/config.go:14-18`). | Wire them through (graph build / stage context) or remove them and trim the CHANGELOG claim. |
| drift-11d / onboard-7 | low/S | `process-guide.md` is triplicated byte-for-byte; templates exist in 3 copies and **have already drifted** (`templates/stage-3` differs from the embedded copies). | Designate one canonical source; generate/symlink the rest at build time. |

---

## Theme 4 — Onboarding & developer experience

| ID | Sev/Eff | Issue | Action |
|----|:--:|------|------|
| onboard-2 | med/M | **Install is broken.** `go install …@latest` has no tag to resolve (zero git tags), and the "prebuilt binary" fallback has no release pipeline — `.goreleaser.yml` exists but no Action runs it. | Publish a tagged release via a goreleaser GitHub Action; verify the public module path. |
| onboard-1 | high/M | README never mentions `decompose init` or the `/decompose` skill — the **recommended entry point** — and headlines the A2A CLI instead. | Rewrite install/usage around `decompose init` + the skill as the primary path; keep A2A as a secondary/advanced section. |
| onboard-3 | med/S | Quick-start step 2 links `process-guide.md` at repo root; file is at `docs/process-guide.md` → 404. *Verified by hand.* | Fix the link (and the same stale ref in CLAUDE.md). |
| flags-1 | med/M | Flags placed **after** the subcommand/name are silently dropped (Go `flag` stops at the first positional). `decompose review auth --verbose` silently ignores `--verbose`. | Detect leftover `-`-prefixed args after parsing and error with a clear message. |
| init-1 | med/M | `decompose init --force` **overwrites the user's existing `PreToolUse` hooks**, and without `--force` it silently skips its own hook if the project has *any* `hooks` key. | Merge into the `PreToolUse` array (append if absent) instead of replacing. |
| onboard-5 | med/M | Thin contributor ramp: no `CONTRIBUTING`, no `.golangci.yml`, lint absent from CI. | Add a short CONTRIBUTING (C-toolchain prereq, make targets), a golangci config, and a lint CI step. |
| onboard-6 / test-6 | med/S | E2E job is gated `if github.ref == 'refs/heads/main'`, so it is **silently skipped on every PR** and excluded from the default `go test` run — it never gates merges. | Run e2e on `pull_request` (or a label/changed-paths filter). |
| validate-1 | low/S | `export`/`status` emit phantom output (all-pending JSON, zero tasks) for a **non-existent** decomposition instead of erroring. | Return not-found when `docs/decompose/<name>/` is absent. |
| status-1 | low/S | Status table renders identical markers for complete and pending stages (`status.go:49-53`). | Use distinct markers (`[x]`/`[ ]`). |
| build-1 | low/S | `augment.go`/`diagram.go` carry `//go:build cgo` but are called unconditionally, so a `nocgo` build fails to compile (`main.go:148,155`). | Drop the tags (the binary needs cgo anyway) or add nocgo stubs. |

---

## Theme 5 — A2A is kept but runtime-incomplete (accuracy notes, not scoped work)

Per the chosen scope (**docs realignment only, no activation work now**), these are recorded so the
README can describe A2A honestly and so a future "activate A2A" effort has a starting map. **No code
work is requested here.**

- **agent-1 (medium):** the specialist agents (`research`/`schema`/`planning`/`taskwriter`/
  `verification`) and the entire `a2a` HTTP **server** transport are unreachable from any shipped
  command — only tests spawn them. The orchestrator can act only as an A2A *client* to external
  `--agents` endpoints (`fanout.go:85`); **nothing in the repo can host the agents being
  coordinated.** This is why the marketed "parallel agent fan-out" doesn't work out of the box.
- **orch-1 (low):** `CapFull` is **statically unreachable** — `probeCodeIntel` hardcodes
  `return false` (`detector_impl.go:132`) with a comment about a CGO build-tag variant that doesn't
  exist. So capability detection can only ever reach `CapA2AMCP`, never `CapFull`.
- **orch-3 (low):** `CapMCPOnly`'s `executeMCPOnly` writes a static placeholder string instead of
  querying MCP tools as its doc claims (`fallback.go:73-89`).
- **agent-5 (low):** SSE streaming is a permanent client stub (`SubscribeToTask` returns
  `ErrNotImplemented`, "wired in T-04.05" — a task that never landed) and unwired on the server.
- **agent-2 / agent-6 (medium/low):** `implementer.go` (the one production-reachable file in
  `internal/agent`, used by `decompose implement`) carries misleading "A2A-compatible agent"
  comments and a dead `buildA2AArtifacts` that is its only reason to import `a2a`. Reword the
  comments; the dead function can be dropped independent of any A2A decision.

> **README realignment from this theme (the only action in scope):** stop presenting capability
> levels 2–3 and `--agents` as a turnkey feature. Describe the binary as an A2A *client* that
> coordinates **external** agent endpoints you run yourself, and mark hosted multi-agent execution
> as experimental/in-progress. That keeps A2A in the product narrative (as decided) while being
> truthful about what runs today.

---

## Theme 6 — Maintainability & architecture clarity

| ID | Sev/Eff | Issue | Action |
|----|:--:|------|------|
| mcp-2 | low/S | Two **dead, duplicate** MCP server constructors (`NewDecomposeMCPServer`, `NewCodeIntelMCPServer`) with descriptions already drifting from the live one. | Delete them; make `NewUnifiedMCPServer` the single registration site. |
| review-1 / orch-5 | med/M | The kept review package imports `internal/orchestrator` (`check_crossms.go:9,42,48`) for milestone parse + topo-sort. With A2A kept this is no longer "blocking a shelve," but it's still an avoidable cross-package coupling. | Optional: extract the two pure functions (`ParseMilestones`, Kahn sort) into a small neutral `internal/plan` package. Lower priority now. |
| graph-8 | low/L | Four tree-sitter extractors duplicate the same `Extract`/`walk`/`extractCall` scaffolding verbatim; adding a 5th language means editing four near-identical files. | When next touching them, extract a shared generic walker + per-language dispatch table. |
| graph-7 | low/S | The `Direction` doc comments in `store.go:44-45` are **inverted** relative to every implementation and caller — the latent root cause that enabled graph-1. *Verified by hand.* | Swap the two comments to match the code. Do this **with** graph-1. **Done 2026-09-18.** |
| review-5 | low/S | Finding IDs/order are non-deterministic (map iteration), so `review-findings.md` isn't reproducible or diffable. | Sort keys before assigning IDs. |
| review-9 | low/M | Check 3's filesystem fallback walks the whole repo once **per** MODIFY target (M×N reads) (`check_deps.go:53,110-161`). | Walk once, build a file→imports map, resolve all targets against it. |
| graph-4 / graph-5 | low/M | KuzuStore re-`Prepare`s the same ~9 Cypher statements per row on every `build_graph`; file parsing is single-threaded though tree-sitter `Parse` is independently parallelizable. | Cache prepared statements / `UNWIND`-batch; fan parsing across a worker pool. Measure first — once-per-session cost. |

---

## Decisions to reconfirm (owner input needed)

The A2A reversal makes three previously "settled" items genuinely open. These are **not** things to
silently implement — they need an explicit call:

1. **Rec 1 — drop the six workflow MCP tools** (`write_stage`, `get_stage_context`, `set_input`,
   `run_stage`, `get_status`, `list_decompositions`; still registered at `unified_server.go:29-59`).
   The original rationale (Claude prefers native tools; the skill carries the workflow) is
   independent of A2A and still holds — but `run_stage` drives the now-kept orchestrator pipeline,
   so confirm whether these stay or go. This gates drift-5d, onboard-4, cli-drift-1, mcp-1.
2. **Rec 5 — reduce the binary to ~8.5k lines.** This was predicated on shelving A2A. With A2A
   kept, the target is void. Decide the new intended binary scope.
3. **A2A activation horizon.** Docs-only now (decided). If/when activation is wanted, Theme 5 is the
   work map (host agents, gate `CapFull`, wire SSE, scope the implement diff).

---

## Prioritized action plan

Sequenced by value ÷ effort. Phases 1–2 are almost entirely independent of the open decisions
above and can start immediately.

### Phase 1 — Quick wins (mostly S effort, high value)
*One focused session. Fix the bug, stop the silent failures, repair the broken references.*

1. **graph-1** + **graph-7** — flip `AssessImpact` to `DirectionUpstream`, correct the inverted
   `store.go` comments, fix the tests that lock in the bug. *(the one real user-facing correctness bug)*
   **Done 2026-09-18.** Also corrected the same inverted wording in the `get_dependencies` MCP
   schema (`codeintel.go`) and `MemStore.neighbors`; added a shallow Mem/Kuzu `AssessImpact`
   parity test, a CHANGELOG entry, and fixed the inverted Check 3 fallback step in
   `docs/review-phase.md`. graph-3 (full conformance suite) remains open.
2. **Doc reference sweep** — drift-4d (`mcp`→`--serve-mcp`), onboard-3 / drift-10d (process-guide
   link + Contents tree), drift-6d / onboard-8 (archive path), drift-7d (dedupe `recommendations.md`).
   **Done 2026-09-19, except drift-7d.** CLAUDE.md now points drift-6d at the tracked
   `docs/decompose/agent-parallel/stage-1-design-pack.md`; also fixed CLAUDE.md's `LICENSE.txt` ref.
   **drift-7d is blocked on a decision:** the root `docs/recommendations.md` is the *newer* copy
   (per-stage review checkpoints, matching CLAUDE.md and the skill), so deleting it as planned would
   lose content. Decide which review policy is current, then keep one copy.
3. **Fail-loud edge cases** — review-8, review-10, graph-9, status-2, validate-1, impl-1.
   **Done 2026-09-19 except graph-9** (separate commit). review-8 surfaces as a report warning, not
   an error. **New finding:** the binary detects Stage 4 by `stage-4-task-specifications.md`, but
   the skill writes only `tasks_mNN.md`, so `review`/`implement`/`status` treat every
   skill-produced decomposition as Stage 4-incomplete (reproduced on `docs/decompose/agent-parallel`).
4. **review-5** (deterministic findings), **status-1** (status markers), **build-1** (cgo tags).

### Phase 2 — Correctness & coverage of the kept core (M–L)
*The product's promoted value, currently least protected.*

5. **review-3 / review-4** — unify Stage 3/4 path normalization (correctness).
6. **review-6** — make the MISMATCH gate fail closed.
7. **graph-6** — register `LanguageTSX()`; decide JS indexing. **graph-2** — fix or drop `CALLS`.
8. **test-2 / review-7** — backfill the 0% review checks (highest-value test work).
9. **test-4, test-8, mcp-3, mcp-4, test-7** — CLI + production MCP server + small kept packages.
10. **graph-3** — cross-store conformance suite (prevents another graph-1).
11. **flags-1, init-1** — CLI flag-drop and hook-overwrite safety.
12. **agent-4** — scope the `implement` git diff (real bug in a reachable command).

### Phase 3 — Onboarding & CI
13. **onboard-2** — release pipeline + tagged release (unbreaks `go install`).
14. **onboard-1** — rewrite README around `decompose init` + the skill; realign the A2A section
    per Theme 5.
15. **onboard-5** — CONTRIBUTING + golangci config + lint in CI. **onboard-6 / test-6** — e2e on PRs.
16. **test-3** — kept-path e2e (build_graph → run_review).

### Phase 4 — After the open decisions are made
17. If Rec 1 confirmed: remove workflow tools + handlers (drift-2d, drift-5d, onboard-4, mcp-1,
    cli-drift-1) and mcp-2 dead constructors.
18. README/CLAUDE.md A2A status reconciliation (drift-1d, drift-3d, agent-7) per the final A2A scope.
19. Lower-priority refactors: review-1/orch-5 decoupling, graph-8 extractor dedup, graph-4/graph-5
    performance, review-9 M×N walk, config-1 wiring.

---

## Appendix — full finding index

74 findings, post-verification severity. `D` = drift-from-decided, `N` = net-new.

| ID | Sev | Eff | Dim | Title |
|----|:--:|:--:|---|---|
| graph-1 | high | S | correctness·N | AssessImpact inverted (returns dependencies, not dependents) — fixed |
| graph-2 | med | M | correctness·N | CALLS edges malformed; consumed by nothing |
| graph-3 | med | M | correctness·N | MemStore/KuzuStore not behavior-equivalent; no conformance suite |
| graph-4 | low | M | performance·N | KuzuStore re-prepares ~9 Cypher statements per row |
| graph-5 | low | M | performance·N | File parsing single-threaded though Parse is parallelizable |
| graph-6 | med | M | correctness·N | `.tsx` parsed with non-JSX grammar; `.js/.jsx` never indexed |
| graph-7 | low | S | docs·N | Store `Direction` doc comments inverted vs all callers — fixed |
| graph-8 | low | L | maint·N | Four extractors duplicate Extract/walk/extractCall verbatim |
| graph-9 | low | S | correctness·N | Silent error swallowing hides parse/persist data loss |
| orch-1 | low | S | correctness·D | `CapFull` statically unreachable |
| orch-3 | low | M | correctness·D | `CapMCPOnly` emits placeholder output, not MCP queries |
| orch-4 | low | S | maint·D | Orchestrator touched post-freeze (4-line test mutex only) |
| orch-5 | med | M | arch·D | review→orchestrator coupling (`check_crossms.go:48`) |
| orch-6 | low | S | test·D | Multi-agent `executeFullMode` path untested (coverage masks it) |
| agent-1 | med | M | maint·D | Specialist agents + a2a server unreachable from production |
| agent-2 | med | S | arch·N | `implementer.go` is the only reachable agent code; dead a2a dep |
| agent-3 | low | S | test·D | Best-tested code is internal infra (now a kept asset) |
| agent-4 | low | M | correctness·D | `implement` attributes whole-repo diff to each milestone |
| agent-5 | low | S | maint·D | SSE streaming stub on client, dead on server |
| agent-6 | low | S | docs·D | Misleading "A2A-compatible agent" comments on ImplementerAgent |
| agent-7 | high | L | arch·D | A2A status unreconciled across docs (reframed: keep + realign) |
| mcp-1 | med | M | arch·D | Six workflow MCP tools still registered (severity pending Rec 1 decision) |
| mcp-2 | low | S | maint·N | Two dead duplicate MCP server constructors |
| mcp-3 | med | M | test·N | Production unified server untested; tests pin dead constructors |
| mcp-4 | med | M | test·N | `run_review`/`generate_diagram` handlers at 0% |
| review-1 | med | M | arch·N | Check 4 depends on orchestrator (coupling) |
| review-2 | med | M | arch·D | `a2a_interpret.go` wires review to A2A path |
| review-3 | med | M | correctness·N | `stripRootLabel` all-or-nothing; corrupts single-root paths |
| review-4 | med | M | correctness·N | Stage 3 stripped, Stage 4 not — different path namespaces |
| review-5 | low | S | DX·N | Non-deterministic finding IDs/order |
| review-6 | med | M | correctness·N | MISMATCH gate hand-rolled, 0% covered, fails open |
| review-7 | high | L | test·N | Checks 2/3/5 + RunReview + report.go at literal 0% |
| review-8 | low | S | correctness·N | Stage 4 parser → zero tasks on drift → false all-clear |
| review-9 | low | M | performance·N | Check 3 fallback walks repo once per MODIFY target (M×N) |
| review-10 | low | S | correctness·N | Extensionless files misclassified as directories, dropped |
| init-1 | med | M | correctness·N | `init --force` overwrites user's existing PreToolUse hooks |
| impl-1 | low | S | correctness·N | `implement` requires Stage 0, which review treats as optional |
| flags-1 | med | M | ease-of-use·N | Flags after subcommand silently dropped |
| config-1 | med | M | docs·N | Four `decompose.yml` fields parsed/documented but unused |
| validate-1 | low | S | ease-of-use·N | export/status emit phantom output for unknown decompositions |
| build-1 | low | S | maint·N | cgo build tags half-applied; nocgo build won't compile |
| status-1 | low | S | ease-of-use·N | Status table identical markers for complete/pending |
| status-2 | low | S | correctness·N | `NextStage` max+1 mis-reports next stage on a gap |
| cli-test-1 | med | L | test·N | Zero tests on cmd/decompose, config, status, export (covered by test-4 + test-8) |
| test-1 | med | L | test·N | Test investment concentrated off the kept surface |
| test-2 | high | M | test·N | 3 of 5 review checks + RunReview + loaders at 0% |
| test-3 | med | M | test·N | e2e suite tests only orchestrator; kept path has no e2e |
| test-4 | med | M | test·N | `cmd/decompose` 0%; subcommand routing untested |
| test-5 | low | S | test·D | Golden tests assert byte-equality on CapBasic placeholders |
| test-6 | low | S | test·N | e2e excluded from default run; never gates PRs |
| test-7 | med | S | test·N | Coverage inversion inside mcptools (run_review 0%) |
| test-8 | med | S | test·N | config/status/export 0%, consumed by CLI + gates |
| test-9 | low | S | test·N | Removal-era flaky sleep-based sync in best-covered pkgs |
| onboard-1 | high | M | docs·D | README omits `decompose init` / the skill; headlines A2A CLI |
| onboard-2 | med | M | ease-of-use·N | Install path broken: no tags, no release pipeline |
| onboard-3 | med | S | docs·N | Quick-start `process-guide.md` link 404s |
| onboard-4 | med | S | DX·D | `init` steers users to the dropped workflow tools |
| onboard-5 | med | M | DX·N | No CONTRIBUTING; lint absent from CI; no golangci config |
| onboard-6 | med | S | test·N | e2e silently skipped on every PR |
| onboard-7 | low | S | maint·N | Templates in 3 copies, already drifted |
| onboard-8 | low | S | docs·N | CLAUDE.md points to gitignored archive path |
| cli-drift-1 | low | S | docs·D | `init` re-injects MCP-first steering into adopters' CLAUDE.md |
| cli-drift-2 | low | M | arch·D | Default CLI path does network A2A detection unless --single-agent |
| drift-1d | high | L | docs·D | A2A status contradiction across docs (keep + realign) |
| drift-2d | med | M | arch·D | Rec 1 unimplemented — 6 workflow tools still registered |
| drift-3d | med | M | docs·D | README markets A2A as turnkey headline product |
| drift-4d | med | S | correctness·N | CLAUDE.md MCP config uses non-existent `mcp` subcommand |
| drift-5d | med | S | docs·D | CLAUDE.md still steers to dropped workflow tools |
| drift-6d | low | S | docs·N | CLAUDE.md archive path points to non-existent file |
| drift-7d | med | S | docs·N | Two divergent `recommendations.md` copies |
| drift-8d | med | L | docs·D | Gating "dusk evaluation" never ran; open questions stalled |
| drift-9d | low | S | docs·D | Only worked example is the A2A feature (fine; A2A kept) |
| drift-10d | med | S | docs·N | README Contents tree stale; process-guide links broken |
| drift-11d | low | S | maint·N | `process-guide.md` triplicated with no sync mechanism |

*(IDs were made globally unique after the parallel finders numbered independently: `drift-Nd` =
the docs-consistency finder's series; `cli-drift-1`/`cli-drift-2`/`cli-test-1` = the cmd-small
finder's findings that originally collided with the docs and test finders. `cli-test-1` overlaps
`test-4`+`test-8` and is retained as a distinct row for full traceability — 74 findings, 74 rows.)*
