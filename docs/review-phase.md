# Review Phase: Codebase-Plan Cross-Reference

this file you are reading was created after analyzing [tests ran](internal/decompose-flow-analysis.md)

> Inserted after Stage 4 completion, before implementation begins. This phase systematically compares decomposition outputs against the actual codebase to catch mismatches, omissions, and stale assumptions that accumulated across the planning stages.

---

## Motivation

The 5-stage pipeline (Stages 0-4) produces decomposition artifacts through a combination of code intelligence queries, manual exploration, and Claude's reasoning. Each stage builds on the previous one, but the chain is only as reliable as its weakest link. Several failure modes are specific to the handoff between planning and execution:

1. **Stale graph data.** The code intelligence graph (`build_graph`) is built once per session. If the codebase changed between graph indexing and Stage 4 completion (common in active repos), the plan may reference symbols, files, or relationships that no longer exist or have moved.

2. **Incomplete graph coverage.** Tree-sitter parsing covers Go, TypeScript, Python, and Rust. Files in other languages, configuration files, build scripts, and generated code are invisible to code intelligence. Tasks referencing these files are based entirely on Claude's direct file reads, which may be incomplete.

3. **IMPORTS edge gaps.** As documented in the flow analysis (Issue #7), TypeScript IMPORTS edges use raw package specifiers rather than resolved file paths. This means `get_dependencies` and `augment` return incomplete dependency information for TypeScript projects. Tasks derived from this data may miss upstream or downstream impacts.

4. **Accumulating drift.** Each stage introduces a small amount of interpretation error. By Stage 4, the cumulative drift between what the plan assumes about the codebase and what the codebase actually contains can be significant -- particularly for MODIFY tasks that assume specific file contents, function signatures, or module structures.

5. **Cross-cutting blind spots.** The milestone structure naturally groups work by feature or layer. Files that span multiple milestones (shared utilities, configuration, type re-exports, middleware) are easy to under-specify because no single milestone "owns" them.

The review phase addresses these by running a structured comparison before any implementation begins.

---

## When To Run

Run the review phase when ALL of the following are true:

- Stages 1 through 4 are complete for the decomposition
- No Stage 4 tasks have been executed yet
- The codebase has not been substantially modified since the decomposition began (if it has, re-run `build_graph` first)

Skip the review phase for decompositions targeting greenfield projects with no existing codebase (pure CREATE plans). In that case, the review reduces to verifying internal consistency across plan files, which is already covered by the Stage 3-to-4 feedback loop.

---

## Review Procedure

The review has five checks, run in order. Each check produces a findings list. Findings are classified as:

- **MISMATCH** -- the plan contradicts the codebase (must fix before implementation)
- **OMISSION** -- the plan misses something present in the codebase (evaluate whether it matters)
- **STALE** -- the plan references something that has changed (update the plan)
- **OK** -- no issue found

### Check 1: File Existence Verification

Compare the Stage 3 target directory tree against the actual filesystem.

For every file listed in the directory tree:

| Action in Plan | Expected State | Finding if Wrong |
|----------------|---------------|-----------------|
| CREATE | File does NOT exist | MISMATCH -- file already exists, task should be MODIFY or plan needs revision |
| MODIFY | File DOES exist | MISMATCH -- file missing, task should be CREATE or dependency is wrong |
| DELETE | File DOES exist | OK (expected) |
| DELETE | File does NOT exist | STALE -- file already removed or was never created |

**How to run:** Walk the directory tree from Stage 3. For each entry, check `os.Stat` or equivalent. No code intelligence needed -- this is a pure filesystem check.

**What this catches:** Renamed files, files that were created or deleted between planning and review, incorrect path assumptions (especially common with monorepo package boundaries).

### Check 2: Symbol and Signature Verification

For every MODIFY task in Stage 4 that references specific symbols (functions, types, classes, methods), verify that those symbols exist in the codebase with the expected signatures.

Procedure:

1. Extract all symbol references from Stage 4 task outlines. These appear as specific function names, type names, method signatures, or class names that the task assumes exist.
2. Query each symbol against the code intelligence graph (`query_symbols`) or fall back to direct `Grep` if the graph is unavailable.
3. For each symbol found, compare the actual signature against what the task outline assumes.

| Outcome | Classification |
|---------|---------------|
| Symbol exists, signature matches | OK |
| Symbol exists, signature differs (different params, return type, generics) | MISMATCH -- update task outline |
| Symbol not found in expected file | MISMATCH -- file may have been refactored |
| Symbol found in a different file than expected | STALE -- update file path in task |

**What this catches:** API changes between planning and execution, incorrect assumptions about function signatures (especially common when Stage 2 skeletons diverge from existing code), symbol renames that happened in a parallel branch.

### Check 3: Dependency Completeness Audit

For each file listed in the Stage 3 directory tree with a MODIFY action, check whether ALL files that import from or depend on it are accounted for in the plan.

Procedure:

1. For each MODIFY target, run `get_dependencies` (downstream direction) or `Grep` for import/require statements referencing the file.
2. Compare the downstream dependents against the plan's file list.
3. Any dependent file that is NOT listed in the Stage 3 directory tree is a potential OMISSION.

Not every dependent needs a task -- some changes are backward-compatible and dependents need no update. The review flags them for human judgment.

| Outcome | Classification |
|---------|---------------|
| All dependents are in the plan | OK |
| Dependent is not in plan, but the change is additive (new export, no signature changes) | OK (note for awareness) |
| Dependent is not in plan, and the change breaks the dependent's assumptions (removed export, changed signature, altered behavior) | OMISSION -- add tasks or verify backward compatibility |

**What this catches:** The most common planning failure -- changing a module's interface without updating all consumers. This is the check that directly compensates for the IMPORTS edge gap in TypeScript projects (Issue #7 from the flow analysis). Where code intelligence falls short, this check fills in by using direct grep-based import tracing.

### Check 4: Cross-Milestone Consistency

Verify that files touched by multiple milestones have consistent assumptions across those milestones.

Procedure:

1. From the Stage 3 directory tree, identify every file that appears in more than one milestone.
2. For each such file, read the task outlines from every milestone that touches it.
3. Check for conflicts: does Milestone N assume a file structure that Milestone M's changes would invalidate? Does one milestone add a field that another milestone's outline doesn't account for?

| Outcome | Classification |
|---------|---------------|
| All milestone references are consistent | OK |
| Later milestone assumes state that earlier milestone will change | MISMATCH -- reorder tasks or update later milestone's outline |
| Two milestones modify the same function/section independently | OMISSION -- add a dependency edge or merge the tasks |

**What this catches:** Ordering errors where parallel milestones step on each other, tasks that will produce merge conflicts when executed sequentially, shared files where later milestones have stale assumptions about what earlier milestones will produce.

### Check 5: Coverage Gap Scan

Scan the codebase for files that are semantically related to the decomposition's scope but absent from the plan entirely.

Procedure:

1. Identify the decomposition's scope boundary -- what directories, packages, or modules does it touch?
2. Within that scope, list all files NOT mentioned in the Stage 3 directory tree.
3. For each unlisted file, assess relevance:
   - Does it import from or export to any file in the plan?
   - Does it contain types, constants, or utilities referenced by planned files?
   - Is it a test file, config file, or build artifact for a planned file?

This is the broadest check and produces the most false positives. Use `get_clusters` (if available) to focus on files in the same module clusters as planned files, rather than scanning the entire repo.

| Outcome | Classification |
|---------|---------------|
| Unlisted file has no connection to planned files | OK -- out of scope |
| Unlisted file imports from a planned MODIFY target | OMISSION -- evaluate impact |
| Unlisted file is a test for a planned file, and no test task exists | OMISSION -- add test task or justify exclusion |
| Unlisted file is a config/build file that references planned modules | OMISSION -- evaluate whether config changes are needed |

**What this catches:** Missing test files, configuration updates (tsconfig paths, package.json exports, build scripts), type re-export barrels, middleware registration, route mounting -- the "glue" files that are easy to forget because they don't contain domain logic.

---

## Output Format

The review produces a single document: `docs/decompose/<name>/review-findings.md`

Structure:

```markdown
# Review Findings: <decomposition-name>

**Review date:** YYYY-MM-DD
**Codebase state:** commit hash or branch name
**Graph indexed:** yes/no (and timestamp if yes)

## Summary

| Check | Findings | Mismatches | Omissions | Stale |
|-------|----------|-----------|-----------|-------|
| 1. File existence | N | x | - | y |
| 2. Symbol verification | N | x | - | y |
| 3. Dependency completeness | N | - | x | - |
| 4. Cross-milestone consistency | N | x | x | - |
| 5. Coverage gap scan | N | - | x | - |
| **Total** | **N** | **x** | **x** | **y** |

## Findings

### Check 1: File Existence

[itemized findings with classification]

### Check 2: Symbol Verification

[itemized findings with classification]

...

## Recommended Plan Updates

[consolidated list of changes to Stage 3/4 files, grouped by milestone]
```

---

## Integration With the Existing Pipeline

The review phase does not replace any existing feedback loop. It is an additional verification pass that sits between the final Stage 4 output and the start of implementation:

```
Stage 0 (once per org)
    |
    v
Stage 1 --> Stage 2 --> Stage 3 --> Stage 4 --> REVIEW --> Implementation
 research     code       plan       tasks      verify
 "what"      "shapes"   "order"   "details"   "check"
    ^           |          |          |           |
    |           v          v          v           v
    <---- feedback loops (revise earlier stages as needed) ----
```

The feedback loops still apply during the review phase. If the review finds MISMATCHes, the fix may require updating Stage 3 (add/remove files from directory tree), Stage 4 (update task outlines), or in rare cases Stage 2 (update skeletons to match actual codebase signatures).

### Relationship to Code Intelligence Tools

The review phase uses the same code intelligence tools as the planning stages but in a verification role rather than a generative one:

| Tool | Planning Use (Stages 1-4) | Review Use |
|------|---------------------------|------------|
| `build_graph` | Index codebase for planning queries | Re-index if stale (optional, recommended) |
| `query_symbols` | Discover symbols for skeleton/task writing | Verify symbols referenced in tasks still exist |
| `get_dependencies` | Map dependencies for milestone ordering | Verify all dependents are accounted for |
| `get_clusters` | Group files into milestones | Identify related files missing from plan |
| `assess_impact` | Estimate change scope | Validate that task scope matches actual impact |

### When Code Intelligence Is Unavailable

If MCP tools are not available (manual fallback mode), run the review using built-in tools:

- **Check 1:** `Glob` for file existence
- **Check 2:** `Grep` for symbol names in expected files
- **Check 3:** `Grep` for import/require statements referencing modified files
- **Check 4:** Manual reading of task outlines for multi-milestone files
- **Check 5:** `Glob` + `Read` to scan directories within scope boundary

The manual path is slower but still catches the most consequential issues (file existence mismatches, missing dependents, cross-milestone conflicts).

---

## Practical Guidance

**Time budget.** The review should take 10-30 minutes depending on decomposition size. If it takes longer, the decomposition likely has scope issues that the review is surfacing -- which is the point.

**False positive rate.** Check 5 (coverage gap scan) produces the most noise. Apply judgment: not every related file needs a task. The question is whether the planned changes will break unlisted files, not whether unlisted files exist.

**When to re-run.** Re-run the review if more than 48 hours pass between review completion and implementation start, or if the codebase receives significant commits in the interim.

**Automation potential.** Checks 1-3 are fully automatable. Check 4 requires reading task outlines and comparing intent, which benefits from an LLM but can be done manually. Check 5 is best done as a human-in-the-loop scan guided by cluster data.

---

*Derived from flow analysis findings (2026-02-27). Addresses gaps identified in Issues #3 and #7 of the decompose flow analysis: incomplete graph coverage and IMPORTS edge resolution limitations.*
