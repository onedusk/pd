---
name: decompose
description: |
  Guide through the 5-stage progressive decomposition methodology for software projects.
  This skill should be used when a user wants to decompose a project idea into an executable
  implementation plan, restructure an existing project, or run any stage of the pipeline:
  dev standards, design pack, code skeletons, task index, or task specifications.
---

# Progressive Decomposition

A 5-stage spec-driven development pipeline: **idea -> specs -> code shapes -> milestone plan -> task specs**.

## Project State

Project: `!basename $(pwd)`
Shared standards: `!ls docs/decompose/stage-0-development-standards.md 2>/dev/null | sed 's/.*\//  /' || echo "  (none)"`
Decompositions: `!ls -d docs/decompose/*/ 2>/dev/null | sed 's|docs/decompose/||;s|/$||;s/^/  /' || echo "  (none)"`

## Directory Structure

```
docs/decompose/
  stage-0-development-standards.md          <- shared across all decompositions
  <name>/                                   <- one directory per decomposition
    stage-1-design-pack.md
    stage-2-implementation-skeletons.md
    stage-3-task-index.md
    tasks_m01.md
    tasks_m02.md
    ...
```

Stage 0 lives at the root of `docs/decompose/` because it is written once per team/org. All other stages live inside a named subdirectory. Multiple decompositions can coexist.

## Pipeline Overview

| Stage | Name | Output Location | Prerequisites |
|:-----:|------|-----------------|---------------|
| 0 | Development Standards | `docs/decompose/stage-0-development-standards.md` | None |
| 1 | Design Pack | `docs/decompose/<name>/stage-1-design-pack.md` | Stage 0 (recommended) |
| 2 | Implementation Skeletons | `docs/decompose/<name>/stage-2-implementation-skeletons.md` | Stage 1 |
| 3 | Task Index | `docs/decompose/<name>/stage-3-task-index.md` | Stages 1 + 2 |
| 4 | Task Specifications | `docs/decompose/<name>/tasks_m01.md`, `tasks_m02.md`, ... | Stage 3 |

## Naming

Every decomposition requires a name. Names use kebab-case: `auth-system`, `v2-redesign`, `calendar-import`.

When a name is not provided in the arguments:

1. Examine the project context -- codebase, any description the user has given, the nature of the work.
2. Suggest a short, descriptive kebab-case name (2-3 words).
3. Ask the user to confirm or provide their own.

Do not proceed past naming without a confirmed name.

## Argument Routing

Arguments: $ARGUMENTS

Parse arguments as: `/decompose [name] [stage|command]`

| Argument | Action |
|----------|--------|
| *(empty)* | List existing decompositions from Project State above. Offer to create a new one (suggest a name or ask for one). |
| `0` | Run Stage 0 (shared, no name needed). Read `references/stage-0.md` for the full workflow. |
| `<name>` | Detect state for that decomposition, report progress, recommend next stage. |
| `<name> 1` | Run Stage 1. Read `references/stage-1.md` for the full workflow. |
| `<name> 2` | Run Stage 2. Read `references/stage-2.md` for the full workflow. |
| `<name> 3` | Run Stage 3. Read `references/stage-3.md` for the full workflow. |
| `<name> 4` | Run Stage 4. Read `references/stage-4.md` for the full workflow. |
| `<name> status` | Report stage completion for that specific decomposition. |
| `<name> next` | Run the next incomplete stage. Read the corresponding `references/stage-{N}.md` for its workflow. |
| `<name> review` | Run the review phase. Read `references/review.md` for the full workflow. |
| `status` | Overview of ALL decompositions -- list each with its completion state. |

If a stage number (1-4) is provided without a name, ask the user which decomposition to run it against. If only one decomposition exists, confirm and use that one.

**When a stage is triggered, always read the corresponding reference file before beginning work.** The reference files contain the complete workflow, review checkpoints, and stage-specific guidance. This SKILL.md provides routing and shared context only.

## Code Intelligence (Optional)

When the `decompose` MCP server is available, these tools provide structural analysis of the codebase. They are query tools -- use them to answer questions about the codebase that would be slow or impractical to answer by reading files one at a time. All file operations (reading, writing, navigating) use native tools (Write, Read, Glob, Grep).

| Tool | Purpose |
|------|---------|
| `build_graph` | Index repository with tree-sitter -- parse source files, extract symbols, build dependency graph, compute file clusters |
| `query_symbols` | Search for functions, types, interfaces, classes by name or kind |
| `get_dependencies` | Traverse dependency graph upstream (what it depends on) or downstream (what depends on it) |
| `assess_impact` | Compute blast radius of modifying files -- direct and transitive dependents with risk score |
| `get_clusters` | Get file clusters (groups of tightly connected files with cohesion scores) |
| `run_review` | Run all 5 mechanical review checks comparing plan against codebase. Produces structured findings (MISMATCH/OMISSION/STALE). |

If these tools are not available, the methodology works without them. The skill uses native tools for everything. Code intelligence becomes increasingly valuable as codebase size grows -- on a 50-file project, reading files directly is fine; on a 2000+ file project, the graph answers structural questions that brute-force exploration cannot.

## Stage Workflow (Common to All Stages)

For every stage:

1. **Read the reference file** -- load `references/stage-{N}.md` for the stage being executed.
2. **Check prerequisites** -- verify that required earlier-stage files exist. For Stages 1-4, check within `docs/decompose/<name>/`. For Stage 1, also check that Stage 0 exists at `docs/decompose/` (warn if missing, but don't block).
3. **Read the template** -- load the corresponding template from `assets/templates/` (relative to this skill's directory).
4. **Explore and gather context** -- follow the stage-specific workflow in the reference file.
5. **Produce the output** -- write each stage file as a complete unit, not appended section by section. If a stage file already exists (resuming work), re-read it before writing to avoid overwriting completed sections. Stage 0 goes to `docs/decompose/`. Stages 1-4 go to `docs/decompose/<name>/`. Create directories as needed.
6. **Review before moving on** -- every reference file includes a review checkpoint. Complete it before proceeding.
7. **Summarize** -- tell the user what was produced, what key decisions were captured, any issues found during review, and what the next stage expects as input.

**Context management.** Long decompositions -- especially Stage 1 on large codebases and Stage 4 with many milestones -- can exhaust context. If token budget warnings appear mid-stage, complete the current section and review checkpoint before continuing. For Stage 4 specifically, write one milestone file at a time, review it, then proceed -- do not try to hold all milestones in context simultaneously. If a stage will clearly exceed context limits, split it across sessions: complete and write sections incrementally, committing after each completed stage to avoid losing work.

## Implementation and Version Control

After Stage 4 and the review phase are complete, read `references/implementation.md` for the full implementation workflow, `/review` cadence, branching strategy, commit conventions, and PR process.

## Methodology Reference

For deeper guidance on any stage -- including the research protocol, skeleton types by domain, milestone design rules, writing acceptance criteria, feedback loops, and handling requirement changes -- read `references/process-guide.md` (bundled with this skill).

Load the reference when:
- The user asks "why" questions about the methodology
- Deciding between approaches for a specific stage
- The user asks about handling changes to earlier stages
- Writing Stage 2 skeletons for an unfamiliar project type

## Output Conventions

- Stage 0: `docs/decompose/stage-0-development-standards.md` (shared root)
- Stages 1-4: `docs/decompose/<name>/` (named subdirectory)
- Stage files: `stage-{N}-{name}.md`
- Task files: `tasks_m{NN}.md` (two-digit milestone number)
- Use the task ID format `T-{MM}.{SS}` consistently
- File actions in task specs: CREATE, MODIFY, DELETE (uppercase)
- Directory names: kebab-case, 2-3 words (e.g., `auth-system`, `v2-redesign`)
