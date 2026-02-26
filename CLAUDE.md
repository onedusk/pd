# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Repository Is

Progressive Decomposition is a **methodology repository**, not a software application. It defines a 5-stage spec-driven development pipeline for turning project ideas into executable task lists: **idea → specs → code shapes → milestone plan → task specs**. There is no build system, no test suite, and no application code to run.

The repository contains:
- `process-guide.md` — the full methodology reference (primary source of truth)
- `templates/` — fill-in stage templates (stages 0–4)
- `examples/` — real project excerpts illustrating each stage
- `skill/decompose/` — Claude Code skill that implements the pipeline interactively
- `docs/internal/` — internal design documents for future evolution (agent-parallel architecture)

## The 5-Stage Pipeline

| Stage | Name | Purpose |
|:-----:|------|---------|
| 0 | Development Standards | Team norms, written once per org |
| 1 | Design Pack | Research-grounded specification |
| 2 | Implementation Skeletons | Compilable type definitions before task planning |
| 3 | Task Index | Dependency-aware milestone plan |
| 4 | Task Specifications | Per-milestone executable tasks with acceptance criteria |

Stages 0 and 2 are the differentiators vs. other SDD tools. Stage 2 forces design into compilable code *before* planning, surfacing ambiguities that prose hides.

## The `/decompose` Skill

The skill at `skill/decompose/SKILL.md` (mirrored to `.claude/skills/decompose/SKILL.md`) is the interactive entry point. It routes arguments as:

```
/decompose                    → list existing decompositions
/decompose 0                  → run Stage 0 (shared, no name needed)
/decompose <name>             → detect state, recommend next stage
/decompose <name> <1-4>       → run specific stage
/decompose <name> next        → run next incomplete stage
/decompose status             → overview of all decompositions
```

Output convention: Stage 0 goes to `docs/decompose/stage-0-development-standards.md` (shared root). Stages 1–4 go to `docs/decompose/<name>/`. Task files are `tasks_m{NN}.md`. Task IDs follow `T-{MM}.{SS}` format.

## Key Conventions

- Decomposition names use **kebab-case** (2–3 words): `auth-system`, `v2-redesign`
- File actions in task specs are always uppercase: **CREATE**, **MODIFY**, **DELETE**
- Templates in `skill/decompose/assets/templates/` are the canonical versions; `templates/` at root are for manual use
- The skill reads `references/process-guide.md` (bundled copy) for methodology details

## Internal Design (Agent-Parallel Evolution)

`docs/internal/agent-parallel-design.md` is a Stage 1 design pack for evolving the pipeline into a multi-agent system using A2A protocol + MCP tools, implemented in Go. This is proposed/future work — the current system is single-agent. Key decisions: Go implementation (ADR-005), KuzuDB for code graph, local-only deployment for v1, graceful degradation to single-agent mode.

## License

PolyForm Shield 1.0.0 — see `LICENSE.txt`.
