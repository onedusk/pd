# Stage 4: Task Specifications

**Output:** `docs/decompose/<name>/tasks_m01.md`, `tasks_m02.md`, ... (one file per milestone)
**Prerequisites:** Stage 3 must exist
**Template:** `assets/templates/stage-4-task-spec.md`

## Before You Start

Before writing file actions, verify that files listed as MODIFY exist with the signatures assumed. If code intelligence is available, use `query_symbols` and `get_dependencies` to check. Otherwise, read the files directly.

## Workflow

1. Read the task index from prerequisites.
2. For each milestone, create a separate task file.
3. For each task within a milestone:
   - Assign ID: `T-{milestone}.{sequence}` (e.g., T-01.03)
   - Name the exact file path and action (CREATE / MODIFY / DELETE)
   - List dependencies (task IDs, not milestone numbers)
   - Write a specific outline: name actual types, methods, and parameters.
   - Write binary acceptance criteria (met or not met, no judgment calls)
4. Order tasks within each milestone so dependencies come first.
5. Each task should represent 15 minutes to 2 hours of work. Split larger tasks. Merge trivial ones.
6. Reference Stage 2 skeleton code where applicable (e.g., "copy the `User` model from Stage 2").

## Context Management for Stage 4

Stage 4 is the most context-intensive stage. Write one milestone file at a time, review it, then proceed to the next. Do not try to hold all milestones in context simultaneously. If token budget warnings appear, complete the current milestone file and its review checkpoint before continuing.

## Review Checkpoint

After writing all task files, do a final validation of the full task spec set against the codebase. This is the most important review checkpoint:
- Compare every MODIFY target against the live codebase -- do the files exist? Have they changed since Stage 1? Do the assumed signatures still match?
- Check that CREATE targets don't already exist.
- Verify that the acceptance criteria aren't duplicating functionality that already exists.
- If code intelligence is available, use `assess_impact` to check for files that import MODIFY targets but aren't accounted for in the plan.

Fix any issues before moving to the review phase.

## Done When

Every file in Stage 3's directory tree has at least one task. Every task has acceptance criteria. Cross-milestone dependencies reference specific task IDs. The final codebase validation passes.
