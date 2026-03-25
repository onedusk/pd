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
| `0` | Run Stage 0 (shared, no name needed). |
| `<name>` | Detect state for that decomposition, report progress, recommend next stage. |
| `<name> 1` | Run Stage 1 for the named decomposition. |
| `<name> 2` | Run Stage 2 for the named decomposition. |
| `<name> 3` | Run Stage 3 for the named decomposition. |
| `<name> 4` | Run Stage 4 for the named decomposition. |
| `<name> status` | Report stage completion for that specific decomposition. |
| `<name> next` | Run the next incomplete stage for that decomposition. |
| `<name> review` | Run the review phase (5 mechanical codebase-plan cross-reference checks). |
| `status` | Overview of ALL decompositions -- list each with its completion state. |

If a stage number (1-4) is provided without a name, ask the user which decomposition to run it against. If only one decomposition exists, confirm and use that one.

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

## Stage Workflow

For every stage:

1. **Check prerequisites** -- verify that required earlier-stage files exist. For Stages 1-4, check within `docs/decompose/<name>/`. For Stage 1, also check that Stage 0 exists at `docs/decompose/` (warn if missing, but don't block).
2. **Read the template** -- load the corresponding template from `assets/templates/` (relative to this skill's directory).
3. **Explore and gather context** -- for Stage 0, ask about team norms. For Stages 1+, explore the codebase, read existing docs, and ask the user about the project. If code intelligence tools are available, use them to answer structural questions (see stage-specific instructions).
4. **Produce the output** -- write the completed stage file. Stage 0 goes to `docs/decompose/`. Stages 1-4 go to `docs/decompose/<name>/`. Create directories as needed.
5. **Review before moving on** -- after writing the stage, review what was just produced against the codebase. Check for incorrect assumptions, naming collisions, references to things that don't exist, or gaps in coverage. Fix any issues before proceeding to the next stage. This review is lightweight -- read what was written, compare it to what exists, flag problems before the next stage builds on bad assumptions.
6. **Summarize** -- tell the user what was produced, what key decisions were captured, any issues found during review, and what the next stage expects as input.

## Stage-Specific Instructions

### Stage 0: Development Standards

**Output:** `docs/decompose/stage-0-development-standards.md`
**Prerequisites:** None

Workflow:
1. Ask the user about their team's existing norms: code review process, testing expectations, change management, escalation rules.
2. If an `AGENTS.md`, `CLAUDE.md`, or `.cursorrules` file exists in the project root, read it -- it likely contains conventions to incorporate.
3. Fill in the template with the user's norms. Keep it under 2 pages.
4. This stage is optional for solo developers. If the user says "skip," note that Stage 0 was skipped and proceed to ask which decomposition to start.

**Done when:** The file exists and covers code change checklist, changeset format, escalation guidance, and testing guidance.

### Stage 1: Design Pack

**Output:** `docs/decompose/<name>/stage-1-design-pack.md`
**Prerequisites:** Stage 0 (recommended but not required)

If code intelligence tools are available, build the dependency graph at the start of this stage. Use it to identify architectural boundaries, high-impact modules, and existing patterns before writing the design pack.

Workflow:
1. Ask the user to describe the project idea or change. What problem does it solve? Who is the user? What platform?
2. Research the target platform, frameworks, and key dependencies. Use code intelligence (`query_symbols`, `get_clusters`) if available and web search to verify current versions and API surfaces.
3. Work through each section of the template collaboratively:
   - Assumptions and constraints (ask the user)
   - Platform and tooling baseline (research + verify)
   - Data model (derive from `query_symbols` + features if available, validate with user)
   - Architecture (propose pattern informed by `get_clusters` if available, explain trade-offs)
   - UI/UX layout (if applicable -- ask for screen inventory)
   - Features (scope with user -- what's in v1, what's not)
   - Integration points (use `get_dependencies` on key files if available)
   - Security and privacy plan
   - ADRs (minimum 3 -- capture decisions as they're made during this stage)
   - PDRs (minimum 2 -- capture product decisions)
   - Condensed PRD
   - Data lifecycle
   - Testing strategy
   - Implementation plan (ordered milestone list)
4. The implementation plan (last section) becomes the input for Stage 3. Ensure milestones are ordered with the foundation layer first and most experimental feature last.

**Review checkpoint:** After writing, verify the design pack against the codebase. Do the assumptions about existing code match reality? Are there modules, patterns, or conventions the design missed? If code intelligence is available, cross-reference `get_clusters` and `get_dependencies` output against the architecture section.

**Done when:** All required sections are filled. The verification checklist at the bottom of the template passes. The user confirms the design.

### Stage 2: Implementation Skeletons

**Output:** `docs/decompose/<name>/stage-2-implementation-skeletons.md`
**Prerequisites:** Stage 1 must exist

Before defining type names, verify they don't collide with existing exports in the codebase. If code intelligence is available, use `query_symbols` to check. Otherwise, use Grep/Glob to search for the names you plan to use.

Workflow:
1. Read Stage 1.
2. Translate the data model into compilable code in the target language. This is NOT pseudocode -- it must parse/compile.
3. Write interface contracts (request/response types) for any API surface described in Stage 1.
4. Write documentation artifacts: entity reference, operation reference, example payloads.
5. If ambiguities are found while writing code (e.g., a field's nullability is unclear, a relationship's delete rule is unspecified), go back and update Stage 1 before continuing.

**Review checkpoint:** After writing, verify skeletons against the codebase. Do the type names collide with existing exports? Do the interface signatures match the codebase's conventions (naming style, error handling patterns, parameter ordering)? Are there missing error types that the codebase's existing code would expect?

**Done when:** All entities from Stage 1 have corresponding type definitions. The code compiles/parses. The skeleton checklist passes.

### Stage 3: Task Index

**Output:** `docs/decompose/<name>/stage-3-task-index.md`
**Prerequisites:** Stages 1 and 2 must exist

Before finalizing milestone ordering, check that the dependency DAG doesn't contradict the actual import graph. If code intelligence is available, use `get_dependencies` to verify. Otherwise, trace imports manually through key files.

Workflow:
1. Read both the design pack and the skeletons from prerequisites.
2. Take the milestone list from Stage 1's implementation plan.
3. For each milestone, enumerate every file that will be created, modified, or deleted. Use `assess_impact` if available to validate file lists.
4. Draw a mermaid dependency graph showing which milestones depend on which others.
5. Identify the critical path and parallelizable work.
6. Count tasks per milestone (details come in Stage 4).
7. Build the complete target directory tree with milestone annotations.

**Review checkpoint:** After writing, verify the task index against the codebase. Does milestone ordering match the actual dependency structure? Are there files that should be in the plan but aren't? Are there dependencies between milestones that the index missed?

**Done when:** Every feature from Stage 1 maps to at least one milestone. The dependency graph has no cycles. The directory tree accounts for every file from Stage 2. The index checklist passes.

### Stage 4: Task Specifications

**Output:** `docs/decompose/<name>/tasks_m01.md`, `tasks_m02.md`, ... (one file per milestone)
**Prerequisites:** Stage 3 must exist

Before writing file actions, verify that files listed as MODIFY exist with the signatures assumed. If code intelligence is available, use `query_symbols` and `get_dependencies` to check. Otherwise, read the files directly.

Workflow:
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

**Review checkpoint:** After writing all task files, do a final validation of the full task spec set against the codebase. This is the most important review checkpoint. Compare every MODIFY target against the live codebase -- do the files exist? Have they changed since Stage 1? Do the assumed signatures still match? Check that CREATE targets don't already exist. Verify that the acceptance criteria aren't duplicating functionality that already exists. If code intelligence is available, use `assess_impact` to check for files that import MODIFY targets but aren't accounted for in the plan.

**Done when:** Every file in Stage 3's directory tree has at least one task. Every task has acceptance criteria. Cross-milestone dependencies reference specific task IDs. The final codebase validation passes.

### Review Phase: Codebase-Plan Cross-Reference

**Output:** `docs/decompose/<name>/review-findings.md`
**Prerequisites:** Stages 1-4 must be complete

The review phase sits between Stage 4 completion and implementation start. If `run_review` is available as an MCP tool, use it. It runs 5 mechanical checks comparing the decomposition plan against the actual codebase:

1. **File existence** -- validates CREATE/MODIFY/DELETE actions against filesystem state
2. **Symbol verification** -- confirms that symbols referenced in task outlines exist in the expected files
3. **Dependency completeness** -- finds files that import MODIFY targets but aren't in the plan
4. **Cross-milestone consistency** -- detects conflicting actions and ordering issues across milestones
5. **Coverage gap scan** -- identifies files within scope that may be affected but aren't planned

Workflow:
1. Call `run_review` with the decomposition name (or run the checks manually if the tool is unavailable).
2. Review the findings. Findings are classified as MISMATCH (must fix), OMISSION (evaluate), or STALE (update plan).
3. Resolve any MISMATCH findings by updating Stage 3/4 files before implementing.

**Done when:** No MISMATCH findings remain. OMISSION findings have been triaged (confirmed or reclassified as OK).

## Implementation Flow

After the decomposition is complete and the review phase passes, implementation follows this pattern:

1. **Implement one task at a time** from the first milestone, following the dependency order in the task specs.
2. **After each task**, run `/review` on the changed files. Fix any issues before moving to the next task.
3. **After completing a milestone**, run `/review` on the full milestone scope before starting the next milestone.
4. **After all milestones are complete**, run `/review` on the entire implementation. This final review catches cross-cutting concerns: error handling consistency, missing migrations, configuration gaps.

The `/review` command is built into Claude Code. It handles code review, catches bugs, and validates implementation against the plan. The key is invoking it at the right moments -- after each task, after each milestone, and once at the end.

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
