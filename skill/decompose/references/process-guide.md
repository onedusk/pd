# Progressive Decomposition — Process Guide

## Overview

Most software projects fail in one of two ways: they start coding without knowing what they're building, or they write detailed specs that nobody reads because the specs are disconnected from the actual code.

Progressive decomposition solves both problems by structuring the pre-coding work into five stages, where each stage takes the output of the previous stage and refines it into something more concrete. By the time you start writing implementation code, you have: shared engineering standards, a research-grounded specification, compilable type definitions, a dependency-aware milestone plan, and individually executable tasks with acceptance criteria.

The methodology is deliberately sequential — you don't skip ahead. But it also includes explicit feedback loops: writing Stage 2 code often reveals Stage 1 ambiguities, and building Stage 3's file tree often uncovers missing work. These discoveries flow backward to fix earlier stages before you write a line of implementation code.

### Relationship to Spec-Driven Development

This methodology is an implementation of [Spec-Driven Development](https://www.thoughtworks.com/en-us/radar/techniques/spec-driven-development) (SDD) — recognized by Thoughtworks' Technology Radar as one of 2025's key engineering practices. The SDD pattern, as identified across tools like GitHub Spec Kit, Amazon Kiro, and the BMAD Method, follows a core loop: **intent → spec → plan → execution**.

Progressive decomposition extends the emerging 3-stage SDD consensus with two additional stages:

- **Stage 0 (Development Standards)** — grounds the project in verified platform versions, tested tooling baselines, and explicit team norms. This prevents a class of errors where implementers (especially AI agents) hallucinate framework APIs or invent conventions that contradict the codebase.
- **Stage 2 (Implementation Skeletons)** — forces the design into compilable type definitions before task planning begins. This is the most distinctive stage in the pipeline, and it has no direct equivalent in existing SDD tools. See [Why Code Before Tasks](#why-code-before-tasks) for the full argument.

The 5-stage pipeline can be mapped to the SDD landscape:

| Stage | This Methodology | GitHub Spec Kit | Amazon Kiro | BMAD Method |
|:-----:|-----------------|----------------|-------------|-------------|
| 0 | **Development Standards** | — | — | Agent personas |
| 1 | Design Pack | `/specify` | requirements.md | PM/Architect agents |
| 2 | **Implementation Skeletons** | — | — | — |
| 3 | Task Index | `/plan` | design.md | Planning agents |
| 4 | Task Specifications | `/tasks` | tasks.md | Task agents |

---

## Stage 0: Development Standards

### Purpose

Generic engineering rules that exist before any specific project. The "how we work" ground rules. Written once per team or organization, reused across every project.

### What It Contains

**Code change checklist** — The steps every code change should follow, in order. A typical sequence:

1. **Plan** — define scope, map affected files, identify risks
2. **Implement** — execute the plan, one logical unit at a time
3. **Test** — write or update tests, cover expected and failure paths
4. **Changelog** — write a changeset entry categorized by type
5. **Report** — summarize what was actually implemented, note deviations
6. **Review** — compare implementation against plan and report
7. **Escalate** — route for approval based on severity (if applicable)
8. **Iterate** — address gaps, repeat until plan/report/code agree

**Changeset format** — How changes are categorized and versioned. Typically uses semver impact levels (major / minor / patch) and categories (added / changed / deprecated / removed / fixed / security).

**Escalation guidance** — Severity levels that determine how much oversight a change needs. A practical breakdown:

| Severity | Examples | Approval | Deploy |
|----------|----------|----------|--------|
| Low | Refactors, docs, style | Self-review | Standard |
| Medium | New features, non-breaking changes | Self-review + fresh-eyes pass | Monitor after |
| High | Breaking changes, data model, security | Formal review | Staged rollout |
| Critical | Billing logic, production migrations | All stakeholders | Maintenance window |

**Testing guidance** — Where to focus testing effort, in priority order:

1. Data integrity paths (writes, transforms, deletes)
2. Integration boundaries (external systems)
3. Business logic (rules and calculations)
4. UI and presentation (lowest priority for automation)

Plus guidance on test levels (unit / integration / E2E), what makes a good test, and practical tradeoffs for your team size.

### How To Write It

- Start from your team's existing norms — written or unwritten
- Keep it under 2 pages. If people won't read it, it doesn't exist.
- Write for the least experienced person who will use it
- Include concrete examples, not abstract principles
- Review quarterly, not per-project

### When To Skip

If you are a solo developer on a personal project, Stage 0 can be a mental checklist. Write it down when a second person joins, or when you want an AI assistant to follow your norms consistently.

**Template:** [`templates/stage-0-development-standards.md`](templates/stage-0-development-standards.md)
**Example:** [`examples/stage-0-excerpt.md`](examples/stage-0-excerpt.md)

---

## Stage 1: Design Pack

### Purpose

Research-based specification. The "what we're building" document. Produced through active investigation of the target platform, available frameworks, and API surfaces — not by guessing.

### What It Contains

#### Required Sections

**1. Assumptions & Constraints**

What you're taking as given. These are not requirements — they are the ground truth you're designing against.

Examples: "Single user, on one device." "Local-only persistence — no backend, no cloud sync." "Must work offline." "Budget: $0 infrastructure cost."

List 3–5 assumptions. If you can't articulate them, you don't understand the problem space well enough to design.

**2. Target Platform & Tooling Baseline**

Specific versions of languages, frameworks, and SDKs. Not "use React" but "React 19.1, TypeScript 5.7, Vite 6.x, targeting Chrome 120+ and Safari 18+."

This section forces you to research before designing. For each component:
- Find the current version and release notes
- Read the "getting started" guide for that specific version
- Identify API surfaces you'll actually use
- Note known limitations, bugs, or migration requirements
- Record version numbers with links to documentation

**3. Data Model / Schema Design**

Every entity, every field, every relationship. Field types, nullability, unique constraints, indexes, delete/cascade rules.

This is not an ER diagram — it is the actual schema specification. For each entity:
- Name and purpose (one sentence)
- Key / unique fields
- All fields with types and nullability
- Relationships with cardinality and delete rules

If you can't write the data model, you don't understand the problem well enough to start coding.

**4. Architecture**

Component diagram showing layers and their connections. Name the pattern (MVC, MVVM, Clean Architecture, Hexagonal, etc.) and explain why you chose it.

Include:
- Layer boundaries and what talks to what
- Data flow direction
- Concurrency / threading model (what runs where)
- Where external dependencies are isolated

**5. UI/UX Layout** *(skip for libraries, CLIs without interactive UI)*

Navigation model (tabs, stack, drawer, etc.), screen inventory (every screen with a one-sentence purpose), and wireframes for the 3–5 most important screens.

ASCII wireframes are sufficient. The goal is structural clarity — what information appears where — not visual design.

**6. Features**

Scoped feature list for the version you're building. Organized by category (core, retrieval, quality-of-life, integrations, etc.). Be explicit about what is in scope and what is not.

**7. Integration Points** *(if applicable)*

Every external system you touch: APIs, frameworks, OS services, third-party SDKs. For each:
- What API surface you'll use (specific methods/endpoints)
- Auth/permission requirements
- Known constraints or limitations

**8. Security & Privacy Plan**

Data protection at rest and in transit. Permissions needed. What's exposed to system-level search, analytics, or logs. Optional hardening measures.

#### Decision Records

**9. Architecture Decision Records (ADRs)**

Minimum 3. Each records:
- **Status:** Accepted / Proposed / Deprecated
- **Context:** Why this decision was needed
- **Decision:** What was decided
- **Consequences:** Trade-offs — what this enables and what it prevents

Cover at minimum: persistence strategy, primary framework choice, data storage approach. Strong design packs also cover: search strategy, AI/ML integration, import/sync patterns, analytics/telemetry, safety guardrails.

**10. Product Decision Records (PDRs)**

PDRs are the product-side counterpart to ADRs. Architecture Decision Records are well-established (Michael Nygard, 2011) and widely adopted — Thoughtworks, AWS, Azure, and Google Cloud all recommend them. But no standard format exists for *product* decisions, even though they are just as consequential and just as easily forgotten.

A PDR uses the same structured format as an ADR, but captures *why the product behaves the way it does for users* rather than *how the system is built*:

- **Status:** Accepted / Proposed / Deprecated
- **Problem:** What user/product problem prompted this
- **Decision:** What was decided
- **Rationale:** Why this is the right call for users

Examples of decisions that belong in PDRs (not ADRs):
- "Only `text` is required when logging an activity — time, kind, and tags are optional" (friction vs. data richness)
- "Export/backup is a v1 requirement, not a v2 nice-to-have" (data safety threshold)
- "The AI assistant is a first-class tab but never mandatory" (graceful degradation)
- "Journal content does not appear in system search by default" (privacy stance)

Minimum 2. Cover at minimum: core mental model (how users think about the data), friction/simplicity tradeoffs (what's required vs optional).

#### Supporting Sections

**11. Condensed PRD**

- **Goal:** One sentence
- **Primary user stories:** 3–5
- **Non-goals:** What you're explicitly not building this version
- **Success criteria:** Measurable conditions (not vanity metrics)

**12. Data Lifecycle & Retention**

What happens on delete (cascade rules). What an export contains. How long data lives.

**13. Testing Strategy**

What to test at each level. Specific scenarios, not "test everything."

**14. Implementation Plan**

Ordered milestone list — names and sequence only. Details go in Stages 3–4. This list becomes the input for Stage 3.

### How To Write It

- **Research first, write second.** Actually read the API docs for the frameworks you plan to use.
- **Decision records are produced during design, not after.** When you make a choice, write it down immediately.
- **Total length:** 10–30 pages depending on project complexity. If it's shorter, you probably skipped research. If it's longer, you're over-specifying.
- **ASCII wireframes are fine.** The goal is structural clarity, not visual fidelity.

### Research Protocol

When investigating a platform, framework, or API:

1. Find the current version and release notes
2. Read the getting-started guide for that specific version
3. Identify API surfaces you'll actually use (list them by name)
4. Note known limitations, bugs, or migration requirements
5. Record version numbers in the design pack with links

### When To Cut Corners

For a side project: skip wireframes if the UI is obvious, write 2 ADRs instead of 10, keep the PRD to 3 lines. But never skip the data model or the implementation plan — those are the load-bearing sections.

**Template:** [`templates/stage-1-design-pack.md`](templates/stage-1-design-pack.md)
**Example:** [`examples/stage-1-excerpt.md`](examples/stage-1-excerpt.md)

---

## Stage 2: Implementation Skeletons

### Purpose

Code-level starting points derived from Stage 1. Forces you to think in code before writing task specifications. Catches design issues that only become visible when you try to express the schema as actual types.

### What It Contains

**1. Data model code**

Complete model/schema definitions in the target language. Not pseudocode — actual compilable (or runnable) code. All entities, all fields, all relationships, all enums, all helper functions.

Translate every entity from Stage 1's schema section into real type definitions. Include initialization logic, validation constraints, and helper utilities (date normalization, ID generation, etc.).

**2. Interface contracts** *(if applicable)*

Typed request/response structures for any API surface: tool-call schemas, REST endpoint DTOs, GraphQL types, RPC definitions, CLI argument types. Include serialization format decisions (JSON encoding strategy, date format, etc.).

**3. Documentation artifacts**

Markdown reference documents derived from the code: entity-by-entity data model reference, operation-by-operation API contract reference, example payloads. These serve as human-readable summaries for onboarding and review.

### Why Code Before Tasks

This is the most distinctive stage in the pipeline, and the one most often skipped in other methodologies. The argument for it is simple: **prose is ambiguous in ways that code is not.**

A design doc can say "Activity has an optional start time." Does that mean the field is nullable? Does it mean there's a default? Does it mean the field exists but can be an empty string? In a type definition, the answer is unambiguous: `startAt: Date?` — it's nullable, period.

Concretely:

- **Schema definitions reveal ambiguities that prose descriptions hide.** When you try to write `@Relationship(deleteRule: .cascade)`, you're forced to decide what happens on delete. The design doc might say "DayEntry has activities" without specifying cascade behavior.
- **Type systems catch contradictions.** A field can't be both required and nullable in a type system — but it absolutely can be in a design doc, and frequently is.
- **Concrete code gives implementers unambiguous starting points.** When a Stage 4 task says "use the `DayEntry` model from Stage 2," the implementer has compilable code to reference, not a paragraph to interpret.
- **Skeleton code can be compiled/type-checked before implementation begins.** If the skeletons don't compile, the design has contradictions — and you've found them before writing a single task.

The Walking Skeleton pattern (Alistair Cockburn) and Contract-First / API-First design are conceptually adjacent, but they produce different artifacts. A walking skeleton is a minimal end-to-end runnable system. An API-first contract is an interface definition. Implementation skeletons are complete type-system definitions for the entire data model — closer to a database migration file than to either of those patterns.

### What It Does NOT Contain

- Business logic implementation
- UI code
- Test code
- Anything that requires runtime state or infrastructure

### Skeleton Types By Domain

| Project Type | Data Model Skeletons | Interface Skeletons |
|---|---|---|
| Mobile app | ORM/model classes, enums, date helpers | Tool-call schemas, Codable types |
| Web app (full-stack) | DB migrations, TypeScript types | API route types, request/response DTOs |
| CLI tool | Config structs, data types | Argument parsing types, output formats |
| Library | Public API types, error types | Protocol/interface definitions |
| Backend service | DB schema, domain entities | API contracts (OpenAPI / protobuf / GraphQL) |

### How To Write It

1. Start with the data model — translate Stage 1's schema into real code
2. Write interface contracts for any API or tool-call surface
3. Write documentation artifacts that summarize what you just wrote
4. Verify the skeletons compile/parse — they are not pseudocode
5. Add comments explaining non-obvious decisions

### When To Skip

For very small projects (under 3 entities, no API surface), you can fold the skeleton code into Stage 4 task outlines. But for anything with a data model of 4+ entities or an API surface, write the skeletons — the time investment pays for itself by catching design issues early.

**Template:** [`templates/stage-2-implementation-skeletons.md`](templates/stage-2-implementation-skeletons.md)
**Example:** [`examples/stage-2-excerpt.md`](examples/stage-2-excerpt.md)

---

## Stage 3: Task Index

### Purpose

Master build plan derived from Stages 1 and 2. Organizes all work into milestones, establishes dependencies, and maps the complete file tree that will be produced.

### What It Contains

**1. Progress table**

Milestones listed with task count and completion tracking.

| # | Milestone | File | Tasks | Done |
|---|-----------|------|:-----:|:----:|
| M1 | Foundation layer | tasks_m01.md | 7 | 0 |
| M2 | Core UI + CRUD | tasks_m02.md | 14 | 0 |
| ... | ... | ... | ... | ... |
| | **Total** | | **N** | **0** |

**2. Milestone dependency graph**

ASCII diagram showing which milestones depend on which, and which can run in parallel.

```
M1 ──► M2 ──┬──► M3 (parallel with M4)
             │
             ├──► M4
             │         \
             └──► M5    ──► M6
```

Identify the **critical path** (longest sequential chain) and **parallelizable work**.

**3. Target directory tree**

Complete listing of every file that will be created, modified, or deleted, annotated with the milestone that touches it.

```
project/
  src/
    models/
      User.ts              CREATE (M1)
      Order.ts             CREATE (M1)
    services/
      AuthService.ts       CREATE (M3)
    routes/
      api.ts               CREATE (M2), MODIFY (M4)
  tests/
    user.test.ts           CREATE (M1)
```

**4. Legend**

Conventions used in the task files: status markers, action types, ID format.

### How To Build It

1. Take the milestone list from Stage 1's implementation plan
2. For each milestone, enumerate every file it will create, modify, or delete
3. Draw the dependency graph — which milestones can start before others finish?
4. Identify the critical path and parallelizable work
5. Count tasks per milestone (details come in Stage 4)

### Milestone Design Rules

- Each milestone should be independently testable (ideally deployable)
- Order milestones so the project is usable at each checkpoint
- First milestone: foundation layer that everything else depends on
- Last milestone: most experimental or optional feature
- 5–12 milestones is typical. Fewer than 5 means decomposition is too coarse. More than 12 means milestones are too granular — merge related ones.

**Template:** [`templates/stage-3-task-index.md`](templates/stage-3-task-index.md)
**Example:** [`examples/stage-3-excerpt.md`](examples/stage-3-excerpt.md)

---

## Stage 4: Task Specifications

### Purpose

Per-milestone task files. Each file contains every task for one milestone, with enough detail that an implementer — human or AI — can execute without ambiguity.

### Task Format

Every task has:

- **ID** — `T-{milestone}.{sequence}` (e.g., T-01.03 = Milestone 1, task 3)
- **Title** — imperative description (e.g., "Create SwiftDataStore.swift")
- **File** — exact path + action (`CREATE` / `MODIFY` / `DELETE`)
- **Dependencies** — task IDs that must complete first
- **Outline** — what to implement: specific types, methods, logic flow, edge cases
- **Acceptance criteria** — concrete, testable "done" conditions

### How To Write Tasks

**Scope:** One task per logical unit of work. Usually one file, sometimes a closely related pair (implementation + test).

**Size:** Each task should take 15 minutes to 2 hours to complete. Split larger tasks. Merge trivial ones.

**Outline specificity:** Name actual types, methods, and parameters. The outline should be specific enough to implement without re-reading the design pack. Reference Stage 2 skeletons where applicable.

**MODIFY tasks:** Don't say "update the file." Say "add method X to class Y" or "replace placeholder in tab Z with ComponentName."

**Dependencies:** Reference task IDs, not milestone numbers. A task can depend on tasks in earlier milestones. Within a milestone, order tasks so dependencies come first. Circular dependencies mean the decomposition is wrong.

### Writing Acceptance Criteria

Acceptance criteria must be binary — either met or not met.

**Good:**
- "App compiles. Records persist across app relaunch."
- "Search for 'lunch' returns activities containing 'lunch' (case-insensitive)."
- "Second import with same events creates zero new records."
- "Export produces a file. Import restores data to a fresh container."

**Bad:**
- "Works correctly."
- "UI looks good."
- "No bugs."
- "Performance is acceptable."

### Milestone File Structure

Each milestone file starts with:
- Milestone name and number
- One-sentence description of what this milestone delivers
- Context: what design decisions (ADRs/PDRs) this milestone fulfills

Then lists all tasks in dependency order.

**Template:** [`templates/stage-4-task-specifications.md`](templates/stage-4-task-specifications.md)
**Example:** [`examples/stage-4-excerpt.md`](examples/stage-4-excerpt.md)

---

## The Flow Between Stages

```
Stage 0 (once per org)
    │
    v
Stage 1 ────► Stage 2 ────► Stage 3 ────► Stage 4 ────► Implementation
 research       code          plan          tasks
 "what"        "shapes"      "order"       "details"
    ^             │             │             │
    │             v             v             v
    └──── feedback loops (revise earlier stages as needed)
```

### Feedback Loops

These are expected and healthy:

- **Stage 2 → Stage 1:** Writing skeleton code reveals that the schema has an ambiguous relationship or a field that should be nullable. Go back and fix Stage 1.
- **Stage 3 → Stage 4:** Building the file tree reveals a missing utility or shared component. Add a task.
- **Stage 4 → Stage 2:** Writing a task outline reveals that a skeleton type needs an additional method or field. Update Stage 2.
- **Stage 4 → Stage 1:** Writing acceptance criteria reveals that a feature's behavior was never specified. Add it to Stage 1.

### When To Iterate vs. When To Ship

**Iterate** when:
- You find a contradiction between stages
- A task's acceptance criteria can't be written because the design is ambiguous
- Skeleton code doesn't compile because the schema has contradictions

**Ship** (move to implementation) when:
- Every Stage 4 task has clear acceptance criteria
- Skeleton code compiles without errors
- The dependency graph has no cycles

**Do NOT iterate** on cosmetic improvements to the documents. They are disposable scaffolding — their value is in the thinking they forced, not in the documents themselves.

### Living Specs: Handling Requirement Changes

The pipeline described above covers the forward pass — going from idea to tasks. But requirements change mid-project. A new integration becomes necessary, a feature gets cut, a data model needs a new field. The staged structure makes impact analysis tractable because changes propagate in a predictable direction.

**Change propagation rules:**

| Change type | Starts at | Propagates through |
|-------------|-----------|-------------------|
| New feature | Stage 1 (add to features + data model) | → Stage 2 (update skeletons) → Stage 3 (add to directory tree) → Stage 4 (add tasks) |
| Schema change | Stage 1 (update data model) | → Stage 2 (update model code) → Stage 4 (update affected task outlines) |
| Cut feature | Stage 1 (move to non-goals) | → Stage 3 (remove from milestone/tree) → Stage 4 (delete tasks) |
| New ADR/PDR | Stage 1 (add decision record) | → Stages 2–4 (update as needed by the decision) |
| Bug in skeleton | Stage 2 (fix code) | → Stage 4 (update task outlines that reference the fix) |
| Milestone reordering | Stage 3 (update dependency graph) | → Stage 4 (update cross-milestone dependencies) |

**Practical guidance:**

1. **Don't update documents you've already implemented past.** If Milestone 3 is done and a schema change only affects Milestones 6+, update Stages 1–2 for accuracy but only write new/revised Stage 4 tasks for the unfinished milestones.
2. **Mark superseded decisions.** When an ADR or PDR is replaced, change its status to `Deprecated` and reference the new decision. Don't delete it — the history of *why* you changed course is valuable.
3. **Version your design pack.** If the changes are substantial (new entity, removed feature), note the revision at the top of Stage 1: "v1.1 — added WeatherData entity, cut HealthKit import."
4. **Changes to Stage 0 are rare.** Development standards rarely change mid-project. If they do, it's usually because you learned something about the platform that affects conventions (e.g., "SwiftData predicates can't capture custom enums"). Update Stage 0 and propagate to affected tasks.

The goal is not to keep the documents perfectly synchronized at all times — it's to have a clear path for tracing the impact of a change. The staged structure gives you that path; a flat spec does not.

---

## Adapting The Methodology

### For AI-Assisted Development

The pipeline is simultaneously a human-readable project plan and an AI-agent-optimized execution context. Each stage maps to a specific context-engineering function:

- **Stage 0** becomes the system prompt, `CLAUDE.md`, `AGENTS.md`, or `.cursorrules` — the agent's operating constraints. This prevents hallucinated framework versions and inconsistent conventions.
- **Stage 1** is the design context provided at session start — the AI needs to understand the full spec to make correct implementation choices.
- **Stage 2** skeletons give the AI concrete code to build from. Without these, agents frequently invent their own type definitions that contradict the design pack.
- **Stage 3** task index is the work queue — the AI processes milestones in dependency order.
- **Stage 4** task files are the individual work items — each task is a self-contained instruction with acceptance criteria the agent can verify against.

The task format (ID, file, dependencies, outline, acceptance) maps directly to what an AI coding agent needs: unambiguous scope, concrete deliverables, and testable completion conditions.

#### AGENTS.md Compatibility

Stage 0 output can be packaged as an [`AGENTS.md`](https://github.com/agentsmd/agents.md) file — the emerging cross-tool standard (stewarded by the Agentic AI Foundation under the Linux Foundation) for providing project context to AI coding agents. This makes the methodology immediately usable with any tool that reads `AGENTS.md`, including Claude Code, Cursor, Aider, and GitHub Copilot.

The mapping:
- `AGENTS.md` ← Stage 0 (development standards + project conventions)
- `CLAUDE.md` / `.cursorrules` ← Stage 0 + project-specific build commands
- `docs/` ← Stages 1–4 (design pack, skeletons, task index, task specs)

### For Teams

| Stage | Owner | Process |
|-------|-------|---------|
| 0 | Tech lead | Written once, reviewed quarterly |
| 1 | Product + engineering (collaborative) | Design review meeting |
| 2 | Architect or senior engineer | Technical review |
| 3 | Collaborative (planning meeting) | Sprint/cycle planning |
| 4 | Implementers (each milestone owner) | Task refinement |

### For Solo Developers

All stages are by you, for you. Stage 0 can be brief (half a page). The discipline of writing Stages 1–2 before coding pays off even when you're the only reader — it catches design issues when they're cheap to fix. The task list (Stages 3–4) is your accountability mechanism.

### Scaling By Project Size

| Project Size | Stage 0 | Stage 1 | Stage 2 | Stage 3 | Stage 4 |
|---|---|---|---|---|---|
| Weekend hack | Skip | 1 page | Skip | Mental list | Mental list |
| Side project | Half page | 3–5 pages | Data model only | 1 page | Brief outlines |
| Production app | Full | 10–30 pages | Full | Full | Full |
| Large system | Full + RFC process | Per-subsystem packs | Per-service skeletons | Cross-team coordination | Per-team task files |

---

## FAQ

**Q: How long does the full process take?**
For a production-quality project: Stage 0 is a one-time cost (1–2 hours). Stages 1–4 together typically take 1–3 days for a moderately complex project. This is not wasted time — it's time you would have spent debugging design mistakes during implementation.

**Q: Can I start coding during Stage 2?**
Stage 2 IS code — it's model definitions and type contracts. But don't start implementation logic until Stage 4 tasks are written. The skeleton code is foundational; the implementation tasks depend on the milestone ordering.

**Q: What if requirements change mid-project?**
See [Living Specs: Handling Requirement Changes](#living-specs-handling-requirement-changes) for detailed guidance. The short version: update Stage 1, propagate changes forward through Stages 2–4 using the change propagation rules, and only revise tasks for unfinished milestones.

**Q: How does this relate to Spec-Driven Development (SDD)?**
This methodology is a specific implementation of SDD — the pattern recognized by Thoughtworks, GitHub (Spec Kit), Amazon (Kiro), and others. It extends the typical 3-stage SDD pipeline with two additional stages (development standards and code skeletons) that reduce ambiguity for implementers. See the [comparison table](#relationship-to-spec-driven-development) in the Overview.

**Q: Is this waterfall?**
No. Waterfall prohibits revisiting earlier stages. This methodology has explicit feedback loops — you're expected to revise earlier stages when later stages reveal issues. The stages are sequential for the initial pass, but iterative thereafter.

**Q: What about Agile?**
This methodology produces the artifacts you need for sprint planning (milestones = epics, tasks = stories). The difference is that you do the decomposition work upfront rather than discovering scope during sprints. This works especially well for projects with known requirements and bounded scope.

**Q: Do I need to use all 5 stages?**
Stage 0 is optional for solo work. For everything else, the minimum viable pass is: Stage 1 (data model + architecture + milestone list) → Stage 3 (progress table + dependency graph) → Stage 4 (task outlines). Skip Stage 2 only if your project has fewer than 3 entities and no API surface.
