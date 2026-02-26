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
Existing stages: `!ls docs/decompose/stage-*.md 2>/dev/null | sed 's/.*\//  /' || echo "  (none found)"`
Task files: `!ls docs/decompose/tasks_m*.md 2>/dev/null | sed 's/.*\//  /' || echo "  (none found)"`

## Pipeline Overview

| Stage | Name | Output File | Prerequisites |
|:-----:|------|-------------|---------------|
| 0 | Development Standards | `docs/decompose/stage-0-development-standards.md` | None |
| 1 | Design Pack | `docs/decompose/stage-1-design-pack.md` | Stage 0 (recommended) |
| 2 | Implementation Skeletons | `docs/decompose/stage-2-implementation-skeletons.md` | Stage 1 |
| 3 | Task Index | `docs/decompose/stage-3-task-index.md` | Stages 1 + 2 |
| 4 | Task Specifications | `docs/decompose/tasks_m01.md`, `tasks_m02.md`, ... | Stage 3 |

## Argument Routing

Arguments: $ARGUMENTS

Route based on arguments:

| Argument | Action |
|----------|--------|
| *(empty)* | Detect current state from Project State above. Report which stages are complete and recommend the next stage to run. Ask the user if they want to proceed with the recommended stage. |
| `0` | Run Stage 0 |
| `1` | Run Stage 1 |
| `2` | Run Stage 2 |
| `3` | Run Stage 3 |
| `4` | Run Stage 4 |
| `status` | Report which stages exist and their completion state. Do not run any stage. |
| `next` | Identify the first incomplete stage and run it. |

## General Workflow

For every stage:

1. **Check prerequisites** — verify that required earlier-stage files exist in `docs/decompose/`. If missing, inform the user and recommend running the prerequisite stage first.
2. **Read the template** — load the corresponding template from `assets/templates/` (relative to this skill's directory).
3. **Explore and gather context** — for Stage 0, ask about team norms. For Stages 1+, explore the codebase, read existing docs, and ask the user about the project.
4. **Produce the output** — write the completed stage file to `docs/decompose/`. Create the directory if it does not exist.
5. **Summarize** — tell the user what was produced, what key decisions were captured, and what the next stage expects as input.

## Stage-Specific Instructions

### Stage 0: Development Standards

**Template:** `assets/templates/stage-0-development-standards.md`
**Output:** `docs/decompose/stage-0-development-standards.md`
**Prerequisites:** None

Workflow:
1. Ask the user about their team's existing norms: code review process, testing expectations, change management, escalation rules.
2. If an `AGENTS.md`, `CLAUDE.md`, or `.cursorrules` file exists in the project root, read it — it likely contains conventions to incorporate.
3. Fill in the template with the user's norms. Keep it under 2 pages.
4. This stage is optional for solo developers. If the user says "skip," note that Stage 0 was skipped and proceed to Stage 1.

**Done when:** The file exists and covers code change checklist, changeset format, escalation guidance, and testing guidance.

### Stage 1: Design Pack

**Template:** `assets/templates/stage-1-design-pack.md`
**Output:** `docs/decompose/stage-1-design-pack.md`
**Prerequisites:** Stage 0 (recommended but not required)

Workflow:
1. Ask the user to describe the project idea. What problem does it solve? Who is the user? What platform?
2. Research the target platform, frameworks, and key dependencies. Use web search to verify current versions and API surfaces.
3. Work through each section of the template collaboratively:
   - Assumptions and constraints (ask the user)
   - Platform and tooling baseline (research + verify)
   - Data model (derive from features, validate with user)
   - Architecture (propose pattern, explain trade-offs)
   - UI/UX layout (if applicable — ask for screen inventory)
   - Features (scope with user — what's in v1, what's not)
   - Integration points (if applicable)
   - Security and privacy plan
   - ADRs (minimum 3 — capture decisions as they're made during this stage)
   - PDRs (minimum 2 — capture product decisions)
   - Condensed PRD
   - Data lifecycle
   - Testing strategy
   - Implementation plan (ordered milestone list)
4. The implementation plan (last section) becomes the input for Stage 3. Ensure milestones are ordered with the foundation layer first and most experimental feature last.

**Done when:** All required sections are filled. The verification checklist at the bottom of the template passes. The user confirms the design.

### Stage 2: Implementation Skeletons

**Template:** `assets/templates/stage-2-implementation-skeletons.md`
**Output:** `docs/decompose/stage-2-implementation-skeletons.md`
**Prerequisites:** Stage 1 must exist

Workflow:
1. Read the Stage 1 design pack (`docs/decompose/stage-1-design-pack.md`).
2. Translate the data model into compilable code in the target language. This is NOT pseudocode — it must parse/compile.
3. Write interface contracts (request/response types) for any API surface described in Stage 1.
4. Write documentation artifacts: entity reference, operation reference, example payloads.
5. If ambiguities are found while writing code (e.g., a field's nullability is unclear, a relationship's delete rule is unspecified), go back and update Stage 1 before continuing.

**Done when:** All entities from Stage 1 have corresponding type definitions. The code compiles/parses. The skeleton checklist passes.

### Stage 3: Task Index

**Template:** `assets/templates/stage-3-task-index.md`
**Output:** `docs/decompose/stage-3-task-index.md`
**Prerequisites:** Stages 1 and 2 must exist

Workflow:
1. Read both the design pack and the skeletons.
2. Take the milestone list from Stage 1's implementation plan.
3. For each milestone, enumerate every file that will be created, modified, or deleted.
4. Draw an ASCII dependency graph showing which milestones depend on which others.
5. Identify the critical path and parallelizable work.
6. Count tasks per milestone (details come in Stage 4).
7. Build the complete target directory tree with milestone annotations.

**Done when:** Every feature from Stage 1 maps to at least one milestone. The dependency graph has no cycles. The directory tree accounts for every file from Stage 2. The index checklist passes.

### Stage 4: Task Specifications

**Template:** `assets/templates/stage-4-task-specifications.md`
**Output:** `docs/decompose/tasks_m01.md`, `tasks_m02.md`, ... (one file per milestone)
**Prerequisites:** Stage 3 must exist

Workflow:
1. Read the task index (`docs/decompose/stage-3-task-index.md`).
2. For each milestone, create a separate task file.
3. For each task within a milestone:
   - Assign ID: `T-{milestone}.{sequence}` (e.g., T-01.03)
   - Name the exact file path and action (CREATE / MODIFY / DELETE)
   - List dependencies (task IDs, not milestone numbers)
   - Write a specific outline: name actual types, methods, and parameters
   - Write binary acceptance criteria (met or not met, no judgment calls)
4. Order tasks within each milestone so dependencies come first.
5. Each task should represent 15 minutes to 2 hours of work. Split larger tasks. Merge trivial ones.
6. Reference Stage 2 skeleton code where applicable (e.g., "copy the `User` model from Stage 2").

**Done when:** Every file in Stage 3's directory tree has at least one task. Every task has acceptance criteria. Cross-milestone dependencies reference specific task IDs.

## Methodology Reference

For deeper guidance on any stage — including the research protocol, skeleton types by domain, milestone design rules, writing acceptance criteria, feedback loops, and handling requirement changes — read `references/process-guide.md` (bundled with this skill).

Load the reference when:
- The user asks "why" questions about the methodology
- Deciding between approaches for a specific stage
- The user asks about handling changes to earlier stages
- Writing Stage 2 skeletons for an unfamiliar project type

## Output Conventions

- All output files go in `docs/decompose/` (create the directory if it does not exist)
- Stage files: `stage-{N}-{name}.md`
- Task files: `tasks_m{NN}.md` (two-digit milestone number)
- Use the task ID format `T-{MM}.{SS}` consistently
- File actions in task specs: CREATE, MODIFY, DELETE (uppercase)
