# Progressive Decomposition

A 5-stage spec-driven development pipeline for taking a software project from idea to executable task list. Each stage refines the previous one: **idea → specs → code shapes → milestone plan → task specs**.

Extends the emerging [Spec-Driven Development](https://www.thoughtworks.com/en-us/radar/techniques/spec-driven-development) (SDD) consensus with two additional stages that existing tools omit: **engineering-standards verification** and **pre-implementation type materialization** (code skeletons before task planning).

Extracted from using in multiple medium-large codebases using this approach. Generalized to work with any stack, any platform, and any team — human or AI-assisted.

## Who This Is For

Builders. Solo developers, small teams, and AI-assisted development workflows. If you have an idea and need to turn it into a structured implementation plan before writing code, this is your process.

## When To Use This

- Starting a new project from scratch
- Restructuring an existing project that grew without a plan
- Onboarding an AI coding assistant onto a greenfield project
- Any time you need to go from "I know roughly what I want" to "here are the exact files to create and what goes in them"

## When NOT To Use This

- Trivial scripts or throwaway prototypes
- Projects where you already have a mature spec and just need to code
- Problems that are genuinely unknowable until you prototype (do a spike first, then apply this)

## The Pipeline

| Stage | Name | Input | Output | Frequency |
|:-----:|------|-------|--------|-----------|
| 0 | Development Standards | Team norms | Ground rules document | Once per org |
| 1 | Design Pack | Project idea + research | Full specification | Once (updated as requirements change) |
| 2 | Implementation Skeletons | Design pack | Code-level starting points | Once (updated as requirements change) |
| 3 | Task Index | Design pack + skeletons | Master build plan | Once (updated as requirements change) |
| 4 | Task Specifications | Task index | Per-milestone executable tasks | Once (updated as requirements change) |

```
Stage 0 (once)
    │
    v
Stage 1 ──── Stage 2 ──── Stage 3 ──── Stage 4
research       code         plan         tasks
 "what"       "shapes"     "order"      "details"
    ^            │            │            │
    │            v            v            v
    └─── feedback loops (revise earlier stages as needed)
```

## How This Compares

Most SDD tools implement a 3-stage pipeline. Progressive Decomposition adds two stages that specifically reduce ambiguity for implementers (human or AI).

| Stage | This Methodology | GitHub Spec Kit | Amazon Kiro | BMAD Method |
|:-----:|-----------------|----------------|-------------|-------------|
| 0 | **Development Standards** | — | — | Agent personas |
| 1 | Design Pack | `/specify` → requirements.md | requirements.md | PM/Architect agents |
| 2 | **Implementation Skeletons** | — | — | — |
| 3 | Task Index | `/plan` → design.md | design.md | Planning agents |
| 4 | Task Specifications | `/tasks` → tasks.md | tasks.md | Task agents |

**Stages 0 and 2 are the differentiators:**

- **Stage 0 (Development Standards)** grounds the project in verified platform versions, tooling baselines, and team norms — preventing hallucinated framework APIs and inconsistent conventions.
- **Stage 2 (Implementation Skeletons)** forces the design into compilable type definitions *before* task planning begins. Schema definitions reveal ambiguities that prose descriptions hide — specifically in data models and interface contracts, where type systems enforce unambiguous field types, nullability, and relationships. A field can't be both required and nullable in a type system — but it can be in a design doc.

## Quick Start (Manual)

1. Copy `templates/` into your project's `docs/` directory
2. Read [`process-guide.md`](process-guide.md) for the full methodology
3. Fill in Stage 0 once for your team/org
4. For each new project, work through Stages 1–4 in order
5. Refer to [`examples/`](examples/) for concrete illustrations from a real project

## decompose CLI

The `decompose` binary automates the pipeline with optional multi-agent parallelism via the A2A protocol.

### Installation

```bash
go install github.com/dusk-indust/decompose/cmd/decompose@latest
```

Or download a prebuilt binary from [GitHub Releases](https://github.com/dusk-indust/decompose/releases).

**Prerequisites:** Go 1.25+, C toolchain (CGO required for tree-sitter and KuzuDB)

### Usage

```bash
# Run all stages (0-4)
decompose myproject

# Run a single stage
decompose myproject 1

# Run with explicit agent endpoints
decompose --agents http://localhost:9100,http://localhost:9101 myproject

# Force single-agent mode (no A2A dispatch)
decompose --single-agent myproject

# Run as MCP server for Claude Code integration
decompose --serve-mcp --project-root /path/to/project
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--project-root` | `.` | Path to the target project |
| `--output-dir` | `docs/decompose/<name>` | Output directory for decomposition files |
| `--agents` | (auto-detect) | Comma-separated A2A agent endpoint URLs |
| `--single-agent` | `false` | Force single-agent mode |
| `--serve-mcp` | `false` | Run as MCP server on stdio |
| `--verbose` | `false` | Enable verbose output |
| `--version` | | Print version and exit |

### Capability Levels

The binary auto-detects available infrastructure and selects the highest capability level:

| Level | Name | Agents | MCP Tools | Code Intelligence | Execution Mode |
|:-----:|------|:------:|:---------:|:-----------------:|----------------|
| 0 | Basic | — | — | — | Template generation with TODO markers |
| 1 | MCP-Only | — | Yes | — | Sequential MCP-backed generation |
| 2 | A2A+MCP | Yes | Yes | — | Parallel agent fan-out with merge |
| 3 | Full | Yes | Yes | Yes | Full pipeline with code graph analysis |

### Development

```bash
make build          # Build binary to bin/decompose
make test           # Run unit tests with race detector
make test-e2e       # Run end-to-end tests
make vet            # Run go vet
make lint           # Run golangci-lint
make clean          # Remove build artifacts
make update-golden  # Regenerate golden test files
```

## Principles

1. **Progressive decomposition** — each stage refines the previous. You don't jump from idea to tasks in one step.
2. **Research before design** — investigate what's actually available (frameworks, APIs, platform capabilities) before making decisions.
3. **Decision records are first-class** — capture "why" alongside "what." ADRs for architecture, [PDRs](#on-pdrs) for product.
4. **Code before tasks** — writing skeletons (Stage 2) forces design issues to surface before you plan the work.
5. **Dependencies are explicit** — at both milestone level (what can parallelize) and task level (what blocks what).
6. **File-level granularity** — every task names exact files and actions (CREATE / MODIFY / DELETE). No ambiguity about scope.
7. **Acceptance criteria everywhere** — every task has concrete, binary "done" conditions. Not "works correctly" but "X returns Y when given Z."

## On PDRs

This methodology introduces **Product Decision Records (PDRs)** — a structured format for product decisions, parallel to the well-established Architecture Decision Record (ADR) format. ADRs capture *how* the system is built; PDRs capture *why* it behaves the way it does for users. Same structure (Status, Problem/Context, Decision, Rationale/Consequences), different domain. No widely adopted standard for product decision records exists, though some practitioners have proposed formats. This methodology provides a structured PDR template inspired by the ADR pattern — because product decisions are just as consequential as architectural ones and just as easily forgotten.

## AGENTS.md Compatibility

Stage 0 output (development standards) can be packaged as an [`AGENTS.md`](https://github.com/agentsmd/agents.md)-compatible file — the emerging cross-tool standard for providing project context to AI coding agents (Claude Code, Cursor, Aider, GitHub Copilot). This makes the methodology immediately usable with any SDD tool or AI agent that reads `AGENTS.md`.

## Contents

```
progressive-decomposition/
├── README.md                                  ← you are here
├── process-guide.md                           ← full methodology reference
├── templates/
│   ├── stage-0-development-standards.md       ← fill-in template
│   ├── stage-1-design-pack.md                 ← fill-in template
│   ├── stage-2-implementation-skeletons.md    ← fill-in template
│   ├── stage-3-task-index.md                  ← fill-in template
│   └── stage-4-task-specifications.md         ← fill-in template
└── examples/
    ├── stage-0-excerpt.md                     ← real project example
    ├── stage-1-excerpt.md                     ← real project example
    ├── stage-2-excerpt.md                     ← real project example
    ├── stage-3-excerpt.md                     ← real project example
    └── stage-4-excerpt.md                     ← real project example
```

## License

[PolyForm Shield 1.0.0](LICENSE)

---

## Formal Foundations

The pipeline maps to several established formalisms. This section is not required reading for practitioners — it grounds the methodology for academic evaluation and for anyone comparing it to formal methods.

- **Stepwise refinement (Dijkstra, 1972; Wirth, 1971).** The stage sequence is a refinement chain: each stage produces a strictly more concrete representation than the previous. The pipeline refines a project idea into an executable task list, applying the same philosophy Dijkstra and Wirth applied to refining abstract programs into executable code.

- **DAG scheduling (PERT/CPM).** The milestone dependency graph (Stage 3) and task dependency graph (Stage 4) are directed acyclic graphs. Critical path analysis, slack time computation, and optimal scheduling follow directly from the well-established PERT/CPM framework (1950s).

- **Fixed-point iteration (Kleene/Tarski).** The feedback loops (Stage 2 → Stage 1, Stage 4 → Stage 2, etc.) are modeled as fixed-point computation over a lattice of specification states. Each iteration adds specificity without removing it (monotonicity). The convergence heuristic (additive-only changes) is an informal check for reaching the fixed point.

- **MSO transductions (Courcelle).** The graph transformations from milestone DAG to task dependency graph are MSO (Monadic Second-Order) transductions. A key result: these transductions preserve decidability of graph properties on structures of bounded treewidth. Verification at the milestone level carries through to the task level — not just by convention, but by mathematical guarantee.

- **Composition closure.** Staged decomposition inherits consistency preservation from the composition closure property of rational transductions. If each stage individually preserves consistency, the pipeline as a whole preserves it. This formally justifies why staged decomposition is structurally safer than single-step decomposition.

The creative phases (Stages 1 and 2 — specification and skeleton writing) are outside the formalizable boundary. They require judgment, domain knowledge, and research. The pipeline does not attempt to formalize these; it structures the work around them so that their outputs can be verified by the algorithmic phases that follow.
