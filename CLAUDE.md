# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Repository Is

Progressive Decomposition is a **methodology repository** with a supporting Go binary. It defines a 5-stage spec-driven development pipeline for turning project ideas into executable task lists: **idea -> specs -> code shapes -> milestone plan -> task specs**.

The repository contains:
- `process-guide.md` -- the full methodology reference (primary source of truth)
- `templates/` -- fill-in stage templates (stages 0-4)
- `examples/` -- real project excerpts illustrating each stage
- `.claude/skills/decompose/` -- Claude Code skill that implements the pipeline interactively
- `cmd/decompose/` + `internal/` -- Go binary providing code intelligence (tree-sitter parsing, dependency graphs, impact analysis) and mechanical review checks
- `docs/internal/` -- architecture assessment, validated flow documentation, recommendations, evaluation framework

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

The skill at `.claude/skills/decompose/SKILL.md` is the interactive entry point. It routes arguments as:

```
/decompose                    -> list existing decompositions
/decompose 0                  -> run Stage 0 (shared, no name needed)
/decompose <name>             -> detect state, recommend next stage
/decompose <name> <1-4>       -> run specific stage
/decompose <name> next        -> run next incomplete stage
/decompose status             -> overview of all decompositions
```

Output convention: Stage 0 goes to `docs/decompose/stage-0-development-standards.md` (shared root). Stages 1-4 go to `docs/decompose/<name>/`. Task files are `tasks_m{NN}.md`. Task IDs follow `T-{MM}.{SS}` format.

## The Go Binary

The binary at `cmd/decompose/` provides two categories of functionality:

**Code intelligence** -- tree-sitter parsing, KuzuDB graph storage, dependency traversal, clustering, and impact analysis. These are MCP tools (`build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`) that provide structural data about a codebase that cannot be obtained efficiently by reading files one at a time. Value scales with codebase size.

**Mechanical review** -- 5 checks comparing a decomposition plan against the actual codebase: file existence, symbol verification, dependency completeness, cross-milestone consistency, coverage gaps. Available as `run_review` MCP tool or `decompose review <name>` CLI command.

The binary is optional. The skill and methodology work without it. Code intelligence becomes increasingly valuable on larger codebases (hundreds or thousands of files).

### Building

```
make build          # produces bin/decompose (requires CGO for tree-sitter + KuzuDB)
make test           # run tests
```

### Running as MCP Server

The binary runs as a stdio MCP server. Configure it in `.claude/mcp.json` or equivalent:

```json
{
  "mcpServers": {
    "decompose": {
      "command": "./bin/decompose",
      "args": ["mcp"]
    }
  }
}
```

## Key Conventions

- Decomposition names use **kebab-case** (2-3 words): `auth-system`, `v2-redesign`
- File actions in task specs are always uppercase: **CREATE**, **MODIFY**, **DELETE**
- Templates in `.claude/skills/decompose/assets/templates/` are the canonical versions; `templates/` at root are for manual use
- The skill reads `references/process-guide.md` (bundled copy) for methodology details
- Review checkpoints happen between every stage, not just after Stage 4 -- each stage is reviewed against the codebase before proceeding to the next
- During implementation: run `/review` after each task, after each milestone, and once across the full scope when all milestones are complete

## Archived: Agent-Parallel Design

`docs/internal/agent-parallel-design.md` contains a Stage 1 design pack for evolving the pipeline into a multi-agent system using A2A protocol. This work is archived -- testing showed the single-agent approach (one Claude session with good instructions and targeted tools) handles the pipeline effectively. The design is preserved as reference material for if/when single-agent decomposition hits scaling limits. See `docs/recommendations.md` for the decision rationale.

## License

PolyForm Shield 1.0.0 -- see `LICENSE.txt`.

<!-- decompose:start -->
## Decompose Code Intelligence

This project has a decompose MCP server with code intelligence tools powered by tree-sitter and a graph database. For code understanding tasks, these tools provide richer context than manual file operations:

- `mcp__decompose__build_graph` — index the codebase (run once per session, persists to .decompose/graph/)
- `mcp__decompose__query_symbols` — find functions, types, interfaces by name
- `mcp__decompose__get_dependencies` — trace upstream/downstream dependencies
- `mcp__decompose__assess_impact` — compute blast radius of file changes
- `mcp__decompose__get_clusters` — discover tightly-coupled file groups

For the /decompose skill specifically:
- `mcp__decompose__get_stage_context` — load templates and prerequisite content
- `mcp__decompose__write_stage` — write stage files with validation and coherence checking
- `mcp__decompose__get_status` — check decomposition progress
<!-- decompose:end -->
