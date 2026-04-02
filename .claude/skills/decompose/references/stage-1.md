# Stage 1: Design Pack

**Output:** `docs/decompose/<name>/stage-1-design-pack.md`
**Prerequisites:** Stage 0 (recommended but not required)
**Template:** `assets/templates/stage-1-design-pack.md`

## Codebase Discovery

If code intelligence tools are available, build the dependency graph at the start of this stage. Use it to identify architectural boundaries, high-impact modules, and existing patterns before writing the design pack.

**Large codebases (500+ files):** Spawn Explore agents per cluster (up to 3-4) with focused discovery prompts: "Explore this cluster's files. Report: public API surface, key data types, external dependencies, and integration points with other clusters." Merge Explore agent outputs into the architecture and features sections.

**Smaller codebases:** The sequential approach (reading files directly, using Grep/Glob) is sufficient.

After Explore agents report (or after manual exploration on smaller codebases), spawn a Plan agent with the combined findings to draft the architecture section. The Plan agent operates read-only and produces a structured assessment: component boundaries, dependency directions, key abstractions, and identified patterns. Review and integrate the Plan agent's draft rather than writing the architecture section from scratch.

## Workflow

1. Ask the user to describe the project idea or change. What problem does it solve? Who is the user? What platform?
2. Research the target platform, frameworks, and key dependencies. Use code intelligence (`query_symbols`, `get_clusters`) if available and web search to verify current versions and API surfaces.
3. Work through each section of the template collaboratively:
   - Assumptions and constraints (ask the user -- identify the actual constraints: data volume, latency requirements, deployment target, team size, existing infrastructure)
   - Platform and tooling baseline (research + verify)
   - Data model (derive from `query_symbols` + features if available, validate with user)
   - Architecture (derive from the project's actual constraints, not from convention -- see reasoning guidance below)
   - UI/UX layout (if applicable -- ask for screen inventory)
   - Features (scope with user -- what's in v1, what's not)
   - Integration points (use `get_dependencies` on key files if available)
   - Security and privacy plan
   - ADRs (minimum 3 -- capture decisions as they're made during this stage, following the reasoning guidance below)
   - PDRs (minimum 2 -- capture product decisions)
   - Condensed PRD
   - Data lifecycle
   - Testing strategy
   - Implementation plan (ordered milestone list)
4. The implementation plan (last section) becomes the input for Stage 3. Ensure milestones are ordered with the foundation layer first and most experimental feature last.

## Reasoning from Constraints, Not Convention

For each architectural and technology decision in this stage, identify the project's actual constraints first, then derive the choice from those constraints. Do not select patterns or tools because they are common or popular. If the answer happens to match the conventional choice, that is fine, but the rationale must trace back to project-specific constraints.

For example: "Use PostgreSQL" is reasoning by convention. "The data access pattern requires concurrent writes from multiple workers, the deployment target is a managed cloud with native Postgres support, and the dataset will exceed 10GB within 6 months -- PostgreSQL satisfies all three constraints; SQLite does not" is reasoning from constraints.

## ADR Rationale Standard

Each ADR's rationale section must: name the specific constraints that drove the decision, explain why the chosen option satisfies those constraints, and explain why each rejected alternative fails to satisfy at least one constraint. If a decision cannot be justified by project constraints, it is either premature (defer it) or the constraints have not been fully identified (go back and ask).

## Review Checkpoint

After writing, verify the design pack against the codebase:
- Do the assumptions about existing code match reality?
- Are there modules, patterns, or conventions the design missed?
- If code intelligence is available, cross-reference `get_clusters` and `get_dependencies` output against the architecture section.

Fix any issues before proceeding to Stage 2.

## Done When

All required sections are filled. The verification checklist at the bottom of the template passes. The user confirms the design.
