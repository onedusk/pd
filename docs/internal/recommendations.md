# Recommendations: Architecture, Scope, and Next Steps

**Date:** 2026-03-13
**Status:** Active
**Supersedes:** architecture-assessment.md (open questions), evaluation-framework.md (status section)
**Prerequisites:** architecture-assessment.md, complete_flow.md, evaluation-framework.md

---

## Where Things Stand

The design-consistency test on thnklabs produced the single most important data point so far: content quality scored 9/10 without the binary's workflow MCP tools ever being called. The SKILL.md, templates, and Claude's own reasoning carried the decomposition. write_stage, get_stage_context, get_status, and set_input were never invoked. Claude used Write, Read, and Glob -- its native tools -- for every file operation.

Code intelligence tools told a different story. build_graph indexed 46 files, 64 edges, and 94 symbols. query_symbols, get_clusters, assess_impact, and get_dependencies were all called during Stage 1 discovery. They worked because they provide data Claude cannot get any other way. But they were not called again in Stages 2-4. By that point Claude already had the context from reading files and prior stage content.

Subsequent testing validated the methodology without the binary entirely. The flow -- idea, high-level plan, /decompose, review, iterate, implement per milestone, /review per task, /review the full implementation -- produced strong results across multiple projects. A particularly telling test: given an enormously vague prompt against a feature catalog, Claude explored the codebase with 3 parallel agents, identified the gap between the completed data layer and the empty application layer, proposed a 6-phase plan grounded in actual codebase conventions, then immediately kicked off /decompose to generate task specs. The methodology carried the reasoning. The binary was not involved.

complete_flow.md crystallized the insight: code intelligence is a query tool at two touchpoints -- Stage 1 discovery and /review validation. It is not a workflow tool, and the architecture should stop treating it as one.

The evaluation framework identified four measurable dimensions (completeness, coherence, execution fidelity, course-correction cost) with concrete rubric anchors and behavioral proxies. The behavioral proxies -- review cycles per task, acceptance criteria pass rate, clean /review -- are the most immediately useful because they are objective.

Meanwhile the codebase has outgrown the methodology it serves. Thirteen internal packages, 80+ Go source files, an 18MB binary with CGO dependencies. The orchestrator package alone is 5,500+ lines with fan-out, merge, verification, and scheduling. The agent package is 4,300+ lines with five specialist agent types. The A2A package is 3,100+ lines with full HTTP client/server, SSE streaming, and task store. Together: 13,000+ lines of multi-agent coordination infrastructure that has never coordinated multiple agents.

The CLAUDE.md already describes the repo as a "methodology repository, not a software application." The codebase no longer matches that description. The recommendations below aim to bring them back into alignment.

The next test will be against the dusk codebase (2,266 files, 7,572 nodes, 22,487 edges, 554 communities) with full code intelligence tools active. This is the scale where code intelligence should demonstrably matter -- Claude is not going to build a mental model of 22,000 dependency edges by reading files one at a time.

---

## Recommendation 1: Adopt Path B+C Hybrid for Workflow

Drop the workflow MCP tools. The skill is the workflow engine.

The architecture-assessment identified five paths forward. The test data narrows the field:

- **Path A (PostToolUse validation hook)** is reactive -- write, validate, rewrite. It adds latency and token cost for something that works 9/10 times without it. The marginal value of catching the drift from 9 to 8 does not justify write-intercept infrastructure.

- **Path B (Skill-Only)** is already proven. The 9/10 content quality came from SKILL.md + templates + Claude's reasoning, not from the binary's workflow wrappers. The open question from architecture-assessment ("Is 9/10 good enough?") is answered: yes, conditional on validation through Recommendation 4.

- **Path C (PreToolUse Advisory)** has one narrow use case worth keeping: when /review produces findings, injecting those findings as context before Claude implements tasks touching flagged files. This is advisory, not redirect. It does not ask Claude to use a different tool -- it gives Claude better information for the tool it was already going to use.

- **Path D (Drop binary entirely)** is premature. Code intelligence provides real value. The question is scope, not existence.

- **Path E (Hybrid binary+plugin)** is the correct long-term direction but premature. The plugin format is untested for binary-bundled distributions, and distribution is not a current goal.

**Decision: Path B as the primary architecture, with a scoped Path C advisory hook for /review findings.**

### Actions

1. Remove workflow MCP tools from the binary: `write_stage`, `get_stage_context`, `set_input`, `get_status`, `run_stage`, `list_decompositions`.
2. Keep code intelligence MCP tools: `build_graph`, `query_symbols`, `get_dependencies`, `assess_impact`, `get_clusters`, `generate_diagram`.
3. Keep `run_review` as an MCP tool -- it provides novel analysis Claude cannot replicate natively.
4. Rewrite SKILL.md to remove the MCP-first workflow. The manual fallback workflow becomes the only workflow. Code intelligence tools are documented as optional enhancements for Stage 1 and /review.
5. Scope the Path C advisory hook precisely: fires on PreToolUse for Write/Edit targeting files flagged in `review-findings.md`, injects the relevant finding as additionalContext. That is the entire scope. No tool redirection, no write blocking.
6. Remove the existing hook's write_stage redirection logic. Keep augment-based context injection only where it enriches without redirecting.

---

## Recommendation 2: Keep Code Intelligence Available Across All Stages

Keep the graph. Anchor it to Stage 1 and /review as primary touchpoints. Do not force usage at Stages 2-4, but do not lock it out either.

The test showed code intelligence was used exactly where complete_flow.md says it should be: Stage 1 discovery. build_graph indexed the project. get_clusters identified boundaries. assess_impact computed blast radius. get_dependencies mapped chains. This is the natural fit -- Claude is discovering a codebase it has not seen before and needs structural answers that brute-force file reading cannot efficiently provide.

Usage dropped off at Stages 2-4 not because of a bug or a SKILL.md failure, but because Claude already had the context by then. Stage 1 is discovery ("what exists, how it connects"). Stages 2-4 are generative ("write this based on what I now know"). The test showed Claude didn't reach for code intelligence at those stages -- but it did not show that code intelligence wouldn't help.

The distinction matters. Instead of directives like "use query_symbols at Stage 2," the SKILL.md should pose questions that code intelligence can answer: "before writing skeletons, verify that the type names you're about to define don't collide with existing exports in the codebase" (Stage 2), "before finalizing the dependency DAG, check that milestone ordering doesn't contradict the actual import graph" (Stage 3), "before writing file actions, verify that files listed as MODIFY exist with the signatures assumed" (Stage 4). If Claude reaches for query_symbols or get_dependencies to answer those questions, good. If it answers them from context, also good. The point is the question, not the tool.

The /review phase is the second primary touchpoint. The 5 mechanical checks already use graph data when available and fall back to filesystem when not. This graceful degradation is the right pattern.

The upcoming dusk test (2,266 files, 22,487 edges) is the real proving ground. At that scale, the questions posed at Stages 2-4 may not be answerable from context alone. Let the test reveal whether question-driven prompts produce natural tool usage at those stages.

### What This Means for the Binary

- The graph package (~4,600 lines) is justified. Tree-sitter parsing and KuzuDB storage provide capabilities Claude's native tools cannot replicate -- particularly at scale. The thnklabs test was 46 files; dusk is 2,266. The value proposition scales with the codebase.
- The CGO dependency cost (tree-sitter, KuzuDB) remains the biggest distribution pain point. Pre-built binaries via GoReleaser for common platforms (darwin/arm64, darwin/amd64, linux/amd64) are already configured. This is sufficient for now.
- Do not force code intelligence usage at any stage. Do not block it either. Let the SKILL.md pose questions and let Claude decide the best tool to answer them.

---

## Recommendation 3: Freeze Multi-Agent Development

The A2A agent pipeline is interesting engineering that solves a problem not yet demonstrated to exist.

The agent-parallel decomposition planned 73 tasks across 8 milestones. Zero have been completed. The code that exists was written speculatively, ahead of the decomposition's own task specs.

The fundamental insight from architecture-assessment applies here: Claude works well as a single agent with good instructions and targeted tools. The pipeline's 5 stages are sequential by design -- each builds on the previous. The parallelism opportunity exists within Stage 1 sections and Stage 4 milestones, but a single Claude session handles these in-context efficiently enough. The test produced a 409-line Stage 1 and an 883-line Stage 2 in single sessions without hitting context limits or quality degradation.

Building multi-agent infrastructure to parallelize what a single agent already does well adds complexity without proven benefit. The A2A protocol, specialist agents, orchestrator pipeline, and fan-out scheduling are 13,000+ lines of coordination machinery for a use case that has not been shown to need coordination.

### Actions

1. Move A2A, agent, and orchestrator packages to an `experimental/` branch or directory. Do not delete -- this represents real design thinking that may become relevant if the single-agent approach hits scaling limits.
2. Write an archival note in the experimental branch explaining why it was shelved and what signal would justify bringing it back. Specifically: "Revisit if single-agent decomposition hits context limits on projects with 20+ milestones, or if Stage 1 research parallelism produces measurably better design packs than sequential discovery."
3. Update CLAUDE.md to remove references to agent-parallel evolution as current work. It is archived exploration.
4. Keep `internal/review/` active -- the 5 mechanical checks operate as CLI command + MCP tool without multi-agent coordination. The A2A interpretive triage part (`a2a_interpret.go`) should be shelved with the rest.

---

## Recommendation 4: Operationalize the Evaluation Framework

The evaluation-framework defined the constructs. Now run the experiment.

The architecture-assessment's open questions ("Is 9/10 good enough?", "How much does the validation layer actually catch?", "Should code intelligence be pushed harder in Stages 2-4?") cannot be answered with n=1. The evaluation framework provides the methodology for n>1. Use it.

### Protocol

1. **Select 3 projects** of different types and scales:
   - One greenfield (pure CREATE, no existing codebase)
   - One brownfield/small (under 100 files, with MODIFY tasks)
   - One brownfield/large (500+ files)

   The thnklabs test was brownfield/small. The dusk test (2,266 files) covers brownfield/large with code intelligence active. A greenfield test is still needed.

2. **For each project, run two decompositions:**
   - **Condition A:** Skill-only. No binary, no code intelligence. SKILL.md + templates + Claude's native tools.
   - **Condition B:** Skill + code intelligence. Binary running, build_graph + query tools available.

   Note: for greenfield projects, Condition B may show minimal difference since there is no existing codebase to graph. This is a useful data point -- it establishes the baseline where code intelligence adds nothing.

3. **Score each decomposition** on the four dimensions from evaluation-framework.md using the evaluation template. Apply LLM-as-judge with position-swapped scoring (evaluate A then B, and B then A, average).

4. **Prioritize behavioral proxies:** Review cycles per task, acceptance criteria pass rate, whether /review comes back clean. These are objective and do not require calibration.

5. **Acknowledge evaluator bias.** The methodology's author is also the evaluator. LLM-as-judge with position-swapped scoring partially mitigates this. Note the limitation transparently when interpreting results. It is not a blocker for internal decision-making but would need addressing if results are shared externally.

### What This Proves or Disproves

- If Condition A consistently scores within 1 point of Condition B across all dimensions: code intelligence is a nice-to-have, not essential. Proceed with the binary as optional enhancement.
- If Condition B scores 2+ points higher on Decomposition Completeness or Cross-Stage Coherence for brownfield projects: code intelligence is essential for existing codebases. Invest in distribution.
- If only the brownfield/large project shows divergence: code intelligence has a scale threshold. Document it and use it to guide recommendations about when to install the binary.
- If question-driven prompts at Stages 2-4 (Recommendation 2) produce natural code intelligence usage in Condition B: the tool is more broadly useful than the initial test suggested. Update the SKILL.md to include these questions permanently.

---

## Recommendation 5: Reduce the Binary to Its Essential Surface

After implementing Recommendations 1 and 3, the binary should contain exactly:

| Package | Purpose | ~Lines |
|---------|---------|--------|
| `internal/graph/` | Tree-sitter parsing, KuzuDB/memstore, clustering, impact, dependency traversal | 4,600 |
| `internal/mcptools/` | Code intelligence + review MCP tool handlers, unified server | 1,000 |
| `internal/review/` | 5 mechanical checks, parsers, report formatting (minus A2A interpret) | 2,000 |
| `internal/export/` | JSON export, Mermaid diagram generation | 260 |
| `internal/config/` | YAML config loading | 40 |
| `internal/status/` | Stage completion scanning | 140 |
| `internal/skilldata/` | Embedded skill files for `decompose init` | 20 |
| `cmd/decompose/` | CLI entry, init, status, review, export, diagram, augment | 500 |

**Total: ~8,500 lines across 7 internal packages.** Down from ~24,000 lines across 13 packages.

What gets shelved: `internal/a2a/` (3,100), `internal/agent/` (4,300), `internal/orchestrator/` (5,500). What gets removed: workflow tool handlers in `internal/mcptools/decompose_handlers.go`, decompose-specific MCP server registrations for workflow tools.

### CGO Consideration

The reduced binary still requires CGO for tree-sitter and KuzuDB. This is the price of real graph analysis. Mitigations:

- Pre-built binaries via GoReleaser for common platforms (already configured).
- Document `go install` with CGO prerequisites for source builds.
- Evaluate whether memstore-only mode (no KuzuDB) can handle projects under a certain file count, eliminating the KuzuDB CGO dep for small projects. The memstore already exists in `graph/memstore.go`. Testing in Recommendation 4 can surface where the threshold lies.

---

## Recommendation 6: Update SKILL.md and CLAUDE.md

This is the lowest-risk, highest-immediate-impact action and should happen first regardless of the other recommendations.

### SKILL.md

1. Remove the "CRITICAL -- MCP TOOLS REQUIRED" block. It fights the model's natural behavior and was not followed in practice.
2. Remove the "MCP-First Stage Workflow" section. Replace with: "When code intelligence tools are available, use `build_graph` at the start of Stage 1 to index the target project. Use `query_symbols`, `get_clusters`, `get_dependencies`, and `assess_impact` to inform research during Stage 1. For all file operations, use native tools (Write, Read, Glob, Grep)."
3. Rename "Manual Fallback Workflow" to "Stage Workflow." It is no longer a fallback -- it is the primary workflow.
4. Add question-driven prompts at each stage that give Claude a reason to use code intelligence when it would help:
   - Stage 1: "Build the dependency graph. Use it to identify architectural boundaries, high-impact modules, and existing patterns."
   - Stage 2: "Before defining type names, verify they don't collide with existing exports in the codebase."
   - Stage 3: "Before finalizing milestone ordering, check that the dependency DAG doesn't contradict the actual import graph."
   - Stage 4: "Before writing file actions, verify that files listed as MODIFY exist with the signatures assumed."
5. Keep the Review Phase section -- it correctly documents `run_review` usage.
6. Remove `set_input`, `get_stage_context`, `write_stage`, `get_status`, `run_stage` from the Available Tools table.
7. Add review checkpoints at every stage boundary, not just post-Stage 4. Between each stage: lightweight self-review ("read what was just written, check it against what exists, flag problems before the next stage builds on bad assumptions"). After Stage 4 completes: full codebase validation comparing task specs against the live dependency graph. During implementation: /review (Claude Code native) after each task and after all milestones. The per-stage reviews are the key differentiator from the earlier flow -- they prevent assumption drift from compounding across stages.

### CLAUDE.md

1. Update the opening paragraph. "There is no build system, no test suite, and no application code to run" is no longer accurate. The repo has a Makefile, Go tests, and a Go binary.
2. Remove or archive the "Internal Design (Agent-Parallel Evolution)" section. Replace with a brief note that agent-parallel design docs exist in `experimental/` or `docs/archive/` as exploratory work.
3. Add a section on the Go binary: what it provides (code intelligence, review checks, project initialization), how to build it, how to run it as MCP server.

---

## Open Questions

1. **Memstore threshold.** Is the in-memory graph store sufficient for projects under N files? If so, the binary could ship without KuzuDB for small projects. N needs testing.

2. **Review granularity and automation.** Testing revealed that per-stage review checkpoints during decomposition (not just post-Stage 4) catch real issues: type collisions at Stage 2, milestone ordering contradictions at Stage 3, stale MODIFY targets at Stage 4. The SKILL.md should instruct lightweight self-review between stages as standard practice, with full /review (Claude Code native) reserved for implementation validation. The deeper question is whether self-review is sufficient or whether delegated review via A2A (another Claude session, Gemini, Codex) would catch issues the authoring session misses. Current evidence: self-review already catches substantive bugs (Decimal serialization, non-atomic Redis, double-serialization). The signal to invest in A2A-delegated review would be a demonstrated class of issues that self-review consistently misses. No such class has appeared yet.

3. **Distribution model.** If the project goes external, the reduced binary (Recommendation 5) makes the skill-binary split cleaner: plugin carries skill + templates, binary carries code intelligence. The plugin format for local binary MCP servers remains untested. See docs/internal/plugin-packaging-decision.md.

4. **Graph-backed review quality.** The review checks fall back to filesystem when the graph is unavailable. How much better are graph-backed findings? Testable through Recommendation 4's Condition A vs. Condition B on the Execution Fidelity dimension.

5. **Question-driven tool usage at scale.** Will the question-driven prompts added to SKILL.md (Recommendation 6, item 4) produce natural code intelligence usage at Stages 2-4 on large codebases? The dusk test (2,266 files) is the first opportunity to observe this. If Claude uses query_symbols to answer "do these type names collide?" without being told which tool to use, the question-driven approach is validated.

6. **Document consolidation.** After implementation, consider collapsing architecture-assessment.md, evaluation-framework.md, and this recommendations document into a single living document. The decision log below is the natural home for the canonical record of what was decided and why.

---

## Decision Log

| # | Recommendation | Decision | Rationale |
|---|---------------|----------|-----------|
| 1 | Workflow architecture | Path B+C hybrid (skill-driven, scoped advisory hook) | 9/10 quality without workflow tools; stop fighting the model |
| 2 | Code intelligence scope | Keep all tools, anchor to Stage 1 + /review, question-driven at 2-4 | Proven at two touchpoints; don't force elsewhere but don't lock out |
| 3 | Multi-agent development | Freeze, shelve to experimental with archival note | 13K lines of unproven coordination infrastructure |
| 4 | Evaluation | Run pairwise comparison on 3 projects (including dusk at scale) | Need n>1 evidence; dusk test is brownfield/large condition |
| 5 | Binary scope | Reduce to graph + review + CLI (~8.5K lines) | Cut 65% of code surface, keep what works |
| 6 | Skill/CLAUDE.md | Remove MCP-first steering, add question-driven prompts, add /review points | Align documentation with proven behavior |

---

## Implementation Sequence

1. **SKILL.md and CLAUDE.md updates** (Rec 6) -- lowest risk, highest immediate impact. Aligns steering with proven behavior. Add question-driven prompts and /review invocation points.
2. **Remove workflow MCP tools** (Rec 1) -- delete tool registrations and handlers. The SKILL.md update makes native tools primary. Scope the advisory hook for /review findings.
3. **Shelve multi-agent code** (Rec 3) -- move A2A, agent, orchestrator to experimental branch. Write archival note documenting why and what signal justifies revival. Mechanical refactor.
4. **Run dusk evaluation** (Rec 4, partial) -- decompose against 2,266-file codebase with code intelligence active. This is the brownfield/large test. Observe whether question-driven prompts produce natural tool usage at Stages 2-4.
5. **Complete evaluation** (Rec 4, remaining) -- run greenfield and brownfield/small tests. Compare Condition A vs B. Generate decision data.
6. **Binary reduction** (Rec 5) -- simplify MCP server registration and CLI subcommands after shelved code is separated.

Steps 1-3 can be done in a single focused session. Step 4 happens naturally as the next real project. Steps 5-6 follow from accumulated evidence.
