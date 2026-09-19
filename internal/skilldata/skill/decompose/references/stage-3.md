# Stage 3: Task Index

**Output:** `docs/decompose/<name>/stage-3-task-index.md`
**Prerequisites:** Stages 1 and 2 must exist
**Template:** `assets/templates/stage-3-task-index.md`

## Before You Start

Before finalizing milestone ordering, check that the dependency DAG doesn't contradict the actual import graph. If code intelligence is available, use `get_dependencies` to verify. Otherwise, trace imports manually through key files.

## Workflow

1. Read both the design pack and the skeletons from prerequisites.
2. Take the milestone list from Stage 1's implementation plan.
3. For each milestone, enumerate every file that will be created, modified, or deleted. Use `assess_impact` if available to validate file lists.
4. Draw a mermaid dependency graph showing which milestones depend on which others.
5. Identify the critical path and parallelizable work.
6. Count tasks per milestone (details come in Stage 4).
7. Build the complete target directory tree with milestone annotations.

## Review Checkpoint

After writing, verify the task index against the codebase:
- Does milestone ordering match the actual dependency structure?
- Are there files that should be in the plan but aren't?
- Are there dependencies between milestones that the index missed?

Fix any issues before proceeding to Stage 4.

## Done When

Every feature from Stage 1 maps to at least one milestone. The dependency graph has no cycles. The directory tree accounts for every file from Stage 2. The index checklist passes.
