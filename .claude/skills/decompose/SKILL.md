---
name: decompose
description: |
  Guide through the 5-stage progressive decomposition methodology for software projects.
  This skill should be used when a user wants to decompose a project idea into an executable
  implementation plan, restructure an existing project, or run any stage of the pipeline:
  dev standards, design pack, code skeletons, task index, or task specifications.
hooks:
  PreToolUse:
    - matcher: "Read|Write|Edit|Glob|Grep"
      hooks:
        - type: command
          command: ".claude/hooks/decompose-tool-guard.sh"
---

# Progressive Decomposition

A 5-stage spec-driven development pipeline: **idea -> specs -> code shapes -> milestone plan -> task specs**.

> **CRITICAL — MCP TOOLS REQUIRED:** This skill has a `decompose` MCP server. Before doing ANYTHING, check if the MCP tools are available by calling `get_status`. If the tool call succeeds, you MUST use MCP tools for ALL operations: `get_status` (not Grep/Glob for files), `build_graph` (not manual Read), `get_stage_context` (not reading templates), `write_stage` (not the Write tool). Do NOT fall back to manual file operations when MCP tools are available. See the "MCP Integration" section below for the full tool list and workflow.

## Project State

Project: `!basename $(pwd)`
Shared standards: `!ls docs/decompose/stage-0-development-standards.md 2>/dev/null | sed 's/.*\//  /' || echo "  (none)"`
Decompositions: `!ls -d docs/decompose/*/ 2>/dev/null | sed 's|docs/decompose/||;s|/$||;s/^/  /' || echo "  (none)"`

## Directory Structure

```
docs/decompose/
  stage-0-development-standards.md          ← shared across all decompositions
  <name>/                                   ← one directory per decomposition
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

1. Examine the project context — codebase, any description the user has given, the nature of the work.
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
| `status` | Overview of ALL decompositions — list each with its completion state. |

If a stage number (1-4) is provided without a name, ask the user which decomposition to run it against. If only one decomposition exists, confirm and use that one.

## MCP Integration (Primary Workflow)

**IMPORTANT: When the `decompose` MCP server tools are available, you MUST use them. Do NOT manually read templates, do NOT manually write stage files, do NOT manually scan for stage completion. The MCP tools handle file I/O, validation, coherence checking, and code intelligence — use them.**

### Available Tools

#### Decomposition Tools
| Tool | Purpose |
|------|---------|
| `get_status` | Check which stages are complete for a decomposition |
| `list_decompositions` | List all decompositions with completion status |
| `get_stage_context` | Get template, section names, and prerequisite content for a stage |
| `write_stage` | Validate, merge, and write stage content (with coherence checking) |
| `set_input` | Store a high-level input file/content to seed the pipeline |
| `run_stage` | Execute a stage via the binary pipeline (produces scaffolds in basic mode) |

#### Code Intelligence Tools
| Tool | Purpose |
|------|---------|
| `build_graph` | Index repository with tree-sitter — parse source files, extract symbols, build dependency graph, compute file clusters |
| `query_symbols` | Search for functions, types, interfaces, classes by name. Filter by kind. |
| `get_dependencies` | Traverse dependency graph upstream (what it depends on) or downstream (what depends on it) |
| `assess_impact` | Compute blast radius of modifying files — direct and transitive dependents with risk score |
| `get_clusters` | Get file clusters (groups of tightly connected files with cohesion scores) |

### MCP-First Stage Workflow

For every stage when MCP tools are available:

1. **Check status** — call `get_status` to verify prerequisites are complete.
2. **Index the codebase** — call `build_graph` with the project root path (once per session; skip if already indexed). Exclude `node_modules`, `vendor`, `.git`, `dist`, `build`.
3. **Get stage context** — call `get_stage_context` to load the template, section names, and content from prior stages.
4. **Gather code intelligence** — call `query_symbols`, `get_dependencies`, `get_clusters`, `assess_impact` as needed for the current stage (see stage-specific instructions below).
5. **Generate sections** — for each section name returned by `get_stage_context`, generate rich markdown content informed by the template structure and code intelligence data. You generate the content; the binary validates it.
6. **Write stage** — call `write_stage` with all sections. The binary validates section order, runs coherence checking, merges, and writes the file. Review any coherence issues returned and fix if needed.
7. **Summarize** — report what was produced, key decisions captured, and what comes next.

### Stage-Specific Code Intelligence Usage

**Stage 0 (Development Standards):**
- No code intelligence needed. Ask about team norms, read CLAUDE.md/AGENTS.md if present.

**Stage 1 (Design Pack):**
- `build_graph` to index the target project.
- `query_symbols` with `kind=type` to discover data model entities.
- `query_symbols` with `kind=interface` to discover API contracts.
- `get_clusters` to identify architectural boundaries for the architecture section.
- `get_dependencies` on key files to understand integration points.
- If `set_input` was called with an input file, its content appears in `get_stage_context` output — use it as the seed for the design.

**Stage 2 (Implementation Skeletons):**
- `query_symbols` to list all types, interfaces, and functions.
- `get_dependencies` on data model files to verify relationship accuracy.
- Compare skeleton types against discovered symbols for completeness.

**Stage 3 (Task Index):**
- `get_clusters` to inform milestone boundaries (each cluster = potential milestone).
- `assess_impact` on files from Stage 2 to validate dependency ordering.
- `get_dependencies` to build the ASCII dependency graph.

**Stage 4 (Task Specifications):**
- `assess_impact` per milestone to identify affected files.
- `get_dependencies` per task file to set correct cross-task dependencies.

## Manual Fallback Workflow

**Only use this workflow if the MCP tools are NOT available** (no `decompose` server configured). If the tools are available, you MUST use the MCP-First Stage Workflow above.

For every stage:

1. **Check prerequisites** — verify that required earlier-stage files exist. For Stages 1-4, check within `docs/decompose/<name>/`. For Stage 1, also check that Stage 0 exists at `docs/decompose/` (warn if missing, but don't block).
2. **Read the template** — load the corresponding template from `assets/templates/` (relative to this skill's directory).
3. **Explore and gather context** — for Stage 0, ask about team norms. For Stages 1+, explore the codebase, read existing docs, and ask the user about the project.
4. **Produce the output** — write the completed stage file. Stage 0 goes to `docs/decompose/`. Stages 1-4 go to `docs/decompose/<name>/`. Create directories as needed.
5. **Summarize** — tell the user what was produced, what key decisions were captured, and what the next stage expects as input.

## Stage-Specific Instructions

### Stage 0: Development Standards

**Output:** `docs/decompose/stage-0-development-standards.md`
**Prerequisites:** None

Workflow:
1. Ask the user about their team's existing norms: code review process, testing expectations, change management, escalation rules.
2. If an `AGENTS.md`, `CLAUDE.md`, or `.cursorrules` file exists in the project root, read it — it likely contains conventions to incorporate.
3. Fill in the template with the user's norms. Keep it under 2 pages.
4. This stage is optional for solo developers. If the user says "skip," note that Stage 0 was skipped and proceed to ask which decomposition to start.

**Done when:** The file exists and covers code change checklist, changeset format, escalation guidance, and testing guidance.

### Stage 1: Design Pack

**Output:** `docs/decompose/<name>/stage-1-design-pack.md`
**Prerequisites:** Stage 0 (recommended but not required)

Workflow:
1. Ask the user to describe the project idea or change. What problem does it solve? Who is the user? What platform?
2. Research the target platform, frameworks, and key dependencies. Use code intelligence (`query_symbols`, `get_clusters`) and web search to verify current versions and API surfaces.
3. Work through each section of the template collaboratively:
   - Assumptions and constraints (ask the user)
   - Platform and tooling baseline (research + verify)
   - Data model (derive from `query_symbols` + features, validate with user)
   - Architecture (propose pattern informed by `get_clusters`, explain trade-offs)
   - UI/UX layout (if applicable — ask for screen inventory)
   - Features (scope with user — what's in v1, what's not)
   - Integration points (use `get_dependencies` on key files)
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

**Output:** `docs/decompose/<name>/stage-2-implementation-skeletons.md`
**Prerequisites:** Stage 1 must exist

Workflow:
1. Read Stage 1 from `get_stage_context` prerequisites (or read file directly in manual mode).
2. Translate the data model into compilable code in the target language. This is NOT pseudocode — it must parse/compile.
3. Write interface contracts (request/response types) for any API surface described in Stage 1.
4. Write documentation artifacts: entity reference, operation reference, example payloads.
5. If ambiguities are found while writing code (e.g., a field's nullability is unclear, a relationship's delete rule is unspecified), go back and update Stage 1 before continuing.

**Done when:** All entities from Stage 1 have corresponding type definitions. The code compiles/parses. The skeleton checklist passes.

### Stage 3: Task Index

**Output:** `docs/decompose/<name>/stage-3-task-index.md`
**Prerequisites:** Stages 1 and 2 must exist

Workflow:
1. Read both the design pack and the skeletons from prerequisites.
2. Take the milestone list from Stage 1's implementation plan.
3. For each milestone, enumerate every file that will be created, modified, or deleted. Use `assess_impact` to validate file lists.
4. Draw an ASCII dependency graph showing which milestones depend on which others. Use `get_dependencies` for accuracy.
5. Identify the critical path and parallelizable work.
6. Count tasks per milestone (details come in Stage 4).
7. Build the complete target directory tree with milestone annotations.

**Done when:** Every feature from Stage 1 maps to at least one milestone. The dependency graph has no cycles. The directory tree accounts for every file from Stage 2. The index checklist passes.

### Stage 4: Task Specifications

**Output:** `docs/decompose/<name>/tasks_m01.md`, `tasks_m02.md`, ... (one file per milestone)
**Prerequisites:** Stage 3 must exist

Workflow:
1. Read the task index from prerequisites.
2. For each milestone, create a separate task file.
3. For each task within a milestone:
   - Assign ID: `T-{milestone}.{sequence}` (e.g., T-01.03)
   - Name the exact file path and action (CREATE / MODIFY / DELETE)
   - List dependencies (task IDs, not milestone numbers)
   - Write a specific outline: name actual types, methods, and parameters. Use `query_symbols` and `get_dependencies` to verify references.
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

- Stage 0: `docs/decompose/stage-0-development-standards.md` (shared root)
- Stages 1-4: `docs/decompose/<name>/` (named subdirectory)
- Stage files: `stage-{N}-{name}.md`
- Task files: `tasks_m{NN}.md` (two-digit milestone number)
- Use the task ID format `T-{MM}.{SS}` consistently
- File actions in task specs: CREATE, MODIFY, DELETE (uppercase)
- Directory names: kebab-case, 2-3 words (e.g., `auth-system`, `v2-redesign`)
