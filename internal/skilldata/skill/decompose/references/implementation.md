# Implementation Flow and Version Control

After the decomposition is complete and the review phase passes, implementation follows this workflow.

## Implementation Steps

1. **Create a feature branch** from the current base branch: `git checkout -b feature/<decomposition-name>`. Commit the decomposition plan files (stages 1-4 + review) as the first commit on the branch. Push.
2. **Implement one task at a time** from the first milestone, following the dependency order in the task specs.
3. **After each task**, run `/review` on the changed files. Fix any issues before moving to the next task.
4. **After completing a milestone**, run `/review` on the full milestone scope. Once clean, commit and push. One commit per milestone -- do not batch multiple milestones into one commit.
5. **After all milestones are complete**, run `/review` on the entire implementation. This final review catches cross-cutting concerns: error handling consistency, missing migrations, configuration gaps.
6. **Open a pull request** from the feature branch to the base branch. Use a merge commit (not squash) to preserve per-milestone history.

The `/review` command is built into Claude Code. It handles code review, catches bugs, and validates implementation against the plan. The key is invoking it at the right moments -- after each task, after each milestone, and once at the end.

## Branching

Before starting implementation, create a feature branch from the current base branch: `git checkout -b feature/<decomposition-name>`. All decomposition plan files and implementation work happen on this branch. Never commit implementation work directly to main.

## During Decomposition

Commit all stage files after Stage 4 validation passes and before implementation begins. This preserves the plan as one commit on the feature branch (e.g., `decompose <name>: stages 1-4`). If the decomposition spans multiple sessions, commit after each completed stage to avoid losing work. Push the branch after committing.

## During Implementation

Commit after each milestone passes /review -- not after each task, not as a single commit at the end. Each milestone commit should be self-contained: all the files for that milestone, tests passing, /review clean. Push after each milestone commit.

## Commit Messages

Follow a consistent format:
- Decomposition plan: `decompose <name>: stages 1-4`
- Per-milestone: `<name> M01: <brief description of what the milestone delivers>`
- Review fixes: `<name> M01: address review findings` (if fixes happen after the initial milestone commit)

## After Final /review Passes

Open a pull request from the feature branch to the base branch. The PR title should reference the decomposition name. The PR body should summarize the milestone breakdown and link to the decomposition plan files. Use a merge commit (not squash) to preserve the per-milestone history on main.

## Do Not

- Let work accumulate uncommitted.
- Collapse multiple milestones into a single commit.
- Commit implementation work directly to main.
- Squash merge the PR (this destroys per-milestone history).

The git history should make it possible to trace what changed where, roll back a specific milestone, and review work incrementally.
