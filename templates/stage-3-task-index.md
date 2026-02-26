# Stage 3: Task Index — [Project Name]

> Master build plan derived from the Design Pack (Stage 1) and Implementation Skeletons (Stage 2).
>
> Delete the HTML comments as you fill in each section.

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
| M1 | [Name] | [tasks_m01.md](tasks_m01.md) | | 0 |
| M2 | [Name] | [tasks_m02.md](tasks_m02.md) | | 0 |
| M3 | [Name] | [tasks_m03.md](tasks_m03.md) | | 0 |
| | **Total** | | **0** | **0** |

<!-- Add or remove rows to match your milestone count. -->

---

## Milestone Dependencies

<!-- ASCII dependency graph. Show:
     - Sequential dependencies (arrows)
     - Parallel paths (separate lines)
     - Convergence points (where paths rejoin)
     Identify the critical path and parallelizable work.

     Optional: annotate edges with the specific artifact that creates the dependency.
     This granularity helps identify when milestones can start earlier than the
     coarse ordering suggests (e.g., M3 only needs TodayView from M2, not all of M2). -->

```
M1 ──► M2 ──┬──► M3
             │
             └──► M4
```

With dependency rationale (optional):

```
M1 ──► M2 [via: DataStore, CoreModels]
       ├──► M3 [via: date navigation from TodayView]
       ├──► M4 [via: searchActivities method]
```

**Critical path:** M1 → M2 → ...

**Parallelizable:** [which milestones can run simultaneously]

---

## Target Directory Tree

<!-- Complete listing of every file that will be created, modified, or deleted.
     Annotate each with the action and milestone.
     Organize by your project's directory structure. -->

```
project/
  src/
    models/
      [File.ext]               CREATE (M1)
    services/
      [File.ext]               CREATE (M3)
    views/
      [File.ext]               CREATE (M2), MODIFY (M4)
  tests/
    [File.ext]                 CREATE (M1)
  config/
    [File.ext]                 MODIFY (M3)
```

**Totals:** [N] files created, [N] modifications, [N] deletions

---

## Before Moving On

Verify before proceeding to Stage 4:

- [ ] Every file from Stage 2 skeletons appears in the directory tree
- [ ] Every feature from Stage 1 maps to at least one milestone
- [ ] Every ADR/PDR is fulfilled by at least one milestone
- [ ] The dependency graph has no cycles
- [ ] The critical path is identified
- [ ] Parallel work is maximized where possible
- [ ] Each milestone is independently testable
- [ ] First milestone creates the foundation layer
- [ ] Last milestone is the most experimental/optional feature
