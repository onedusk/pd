# Stage 4: Task Specifications — Milestone [N]: [Name]

> [One-sentence description of what this milestone delivers]
>
> Fulfills: [ADR/PDR references, if applicable]
>
> Create one copy of this file per milestone (e.g., tasks_m01.md, tasks_m02.md, ...).
> Delete the HTML comments as you fill in tasks.

---

<!-- Task template — copy and fill for each task in this milestone.
     Order tasks so dependencies come first. -->

- [ ] **T-[MM].[SS] — [Imperative title]**
  - **File:** `[exact/path/to/file.ext]` ([CREATE | MODIFY | DELETE])
  - **Depends on:** [Task IDs, or "None"]
  - **Outline:**
    - [What to implement — name specific types, methods, functions]
    - [Logic flow for non-trivial operations]
    - [Edge cases to handle]
    - [References to Stage 2 skeleton code if applicable]
  - **Acceptance:** [Concrete, testable "done" conditions]

---

- [ ] **T-[MM].[SS] — [Imperative title]**
  - **File:** `[exact/path/to/file.ext]` ([CREATE | MODIFY | DELETE])
  - **Depends on:** [Task IDs]
  - **Outline:**
    - [Details]
  - **Acceptance:** [Conditions]

---

<!-- Repeat for each task in the milestone.
     Typical milestone: 4–14 tasks. -->

---

## Task Writing Rules

<!-- Remove this section from your actual task files — it's here as a reference. -->

1. **One file per task** — exceptions: closely related pairs (implementation + test)
2. **Outline is implementation-specific** — name actual types, methods, parameters
3. **Acceptance criteria are binary** — either met or not met, no judgment calls
4. **Dependencies reference task IDs** — not milestone numbers
5. **MODIFY tasks specify what changes** — "add method X to class Y", not "update the file"
6. **Task size: 15 min to 2 hours** — split larger tasks, merge trivial ones

### Acceptance Criteria Examples

**Good:**
- "Project compiles. All model types are importable from the main target."
- "Search for 'lunch' returns matching records (case-insensitive)."
- "Second import with same external IDs creates zero new records."
- "Export produces a file. Import restores data to a fresh container."
- "Tab bar renders with N tabs. Switching works."

**Bad:**
- "Works correctly."
- "UI looks good."
- "No bugs."
- "Performance is acceptable."
