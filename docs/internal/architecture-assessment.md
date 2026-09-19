# Architecture Assessment: Go Binary Approach

**Date:** 2026-03-05
**Status:** Open -- evaluating alternatives

---

## Test Results: design-consistency on thnklabs

### What Worked

Content quality across all stages scored 9/10. The decomposition was thorough, internally consistent, and properly tailored to the target project (solo dev / Next.js / Bun / Playwright):

- Stage 0 -- Correctly adapted, not Go-specific
- Stage 1 -- 409 lines, all required sections, 5 ADRs, 3 PDRs, proper milestone ordering, comprehensive screen inventory
- Stage 2 -- 883 lines of compilable TypeScript skeletons with exact imports, prop interfaces, inline comments
- Stage 3 -- 20 tasks across 5 milestones, correct dependency DAG, full directory tree with milestone annotations
- Stage 4 -- All 5 task files with proper T-{MM}.{SS} IDs, exact file paths, CREATE/MODIFY actions, dependencies, acceptance criteria

Cross-stage coherence was solid: Stage 2 skeletons match Stage 1's design token model exactly, Stage 3 directory tree accounts for every Stage 2 file, Stage 4 specs reference Stage 2 skeletons, all ADR/PDR-to-milestone mappings complete.

Code intelligence MCP tools were used in Stage 1: build_graph (46 files, 64 edges, 94 symbols), get_clusters, assess_impact, get_dependencies.

### What Did Not Work

Tool compliance scored 4/10. The core workflow MCP tools were never called:

| Tool | Expected Use | Actual Use |
|------|-------------|------------|
| write_stage | All 9 stage output files | Never called. Used native Write tool instead. |
| get_stage_context | Load templates + prior stage content | Never called. Read templates manually from assets/templates/. |
| get_status | Verify prerequisites before each stage | Never called. Used Grep/Glob to check file existence. |
| set_input | Store user-provided reference material | Never called. Content stayed in context from @file annotation. |
| query_symbols | Stage 2 skeleton validation | Never called (only used in Stage 1). |
| assess_impact | Stages 3-4 blast radius analysis | Never called after Stage 1. |
| get_dependencies | Stage 4 dependency verification | Never called after Stage 1. |

The binary's validation layer (section ordering, coherence checking, required fields) was entirely bypassed because write_stage was never called. All 9 files went through the raw Write tool.

### Root Cause

Issues 1-4 share a root cause: Claude prefers its native tools (Write, Read, Glob) over MCP workflow tools that duplicate native capabilities. The SKILL.md says "MUST use MCP tools" but the steering is not strong enough to override the model's default behavior.

Code intelligence tools worked because they provide data Claude cannot get any other way. Workflow tools failed because they wrap operations Claude already knows how to perform. The model correctly identifies that it can write a file with Write -- it does not perceive the validation wrapper as necessary.

Code intelligence usage dropped off after Stage 1 for a related reason: by Stages 2-4, Claude already has the project context from reading files and prior stage content, so it doesn't feel the need for query_symbols or assess_impact even though they would add value.

---

## Architectural Analysis

The Go binary is doing two fundamentally different things, and one of them works while the other fights the model.

### Category 1: Novel Capability (works)

Code intelligence tools provide data Claude cannot obtain through native tools:

- build_graph -- indexes the codebase into a dependency graph (KuzuDB)
- query_symbols -- searches for type definitions, functions, exports
- get_dependencies -- maps upstream/downstream dependency chains
- get_clusters -- identifies architectural boundaries
- assess_impact -- calculates blast radius of changes

These were used because they are genuinely additive. There is no native equivalent.

### Category 2: Workflow Orchestration (does not work)

Workflow tools wrap operations Claude can already do:

- write_stage -- Write tool + validation + section ordering
- get_stage_context -- Read tool on templates + prior stages
- get_status -- Glob/Grep on expected file paths
- set_input -- storing context that's already in the conversation

The validation and coherence checking inside these wrappers is real value. But delivering it through tool-call redirection -- asking Claude to use write_stage instead of Write -- fights the model's natural behavior.

### The Fundamental Problem

Adding hook enforcement (blocking Write to docs/decompose/, forcing write_stage) would technically work, but it means building enforcement infrastructure to prevent the model from doing what it naturally does well, in order to force it through a wrapper. That is a lot of machinery to fight the model's grain.

---

## Possible Paths Forward

### Path A: PostToolUse Validation Hook

Keep the binary for code intelligence. Remove workflow MCP tools (write_stage, get_stage_context, set_input). Move validation into a PostToolUse hook on Write.

When Claude writes to `docs/decompose/` using the native Write tool, a PostToolUse hook fires, calls the binary (e.g., `decompose validate-stage --file <path>`), and checks section ordering, required fields, cross-stage coherence. If validation fails, the hook returns feedback as additionalContext and Claude fixes the file.

The validation still happens. Claude does not need to change its natural workflow to get it.

Pros: Works with the model's grain. Validation still exercised. Binary scope reduced to what it's good at.
Cons: Reactive (fix after write) rather than proactive (prevent bad write). Multiple write-fix cycles possible.

### Path B: Skill-Only (Drop Binary for Workflow)

Keep code intelligence binary. Remove all workflow MCP tools. Strengthen SKILL.md instructions and templates to carry the full workflow load. Accept that content quality at 9/10 without workflow tools means the skill is already doing the heavy lifting.

Pros: Simplest. Already proven by the test (9/10 quality without workflow tools).
Cons: No validation layer at all. Relies entirely on SKILL.md quality and Claude's adherence.

### Path C: PreToolUse Advisory Hook

Keep the existing hook but change it from "suggest write_stage" to "inject validation context." When Claude is about to Write to docs/decompose/, the PreToolUse hook calls the binary to check what a valid file at that path should contain (expected sections, required fields, prior stage references), and injects that as additionalContext. Claude writes with the validation criteria in its context.

Pros: Proactive (guidance before write, not after). No tool redirection.
Cons: Adds latency to every write. Advisory only -- Claude may still ignore it.

### Path D: Rethink the Binary Entirely

If the only tools that work are code intelligence, and those are only used in Stage 1 (discovery), question whether a persistent Go binary with KuzuDB and tree-sitter is justified. Could Stage 1 code intelligence be delivered differently -- e.g., a lighter analysis pass, or inline in the skill using Claude's native code reading?

Pros: Eliminates the binary maintenance burden entirely.
Cons: Loses dependency graph, symbol search, cluster detection, impact analysis. These are genuinely useful even if underutilized in Stages 2-4.

### Path E: Hybrid -- Binary for Intelligence, Plugin for Workflow

Split the distribution: code intelligence stays as a binary MCP server (installed separately), workflow and skill layer becomes a plugin. The plugin is self-contained and portable. The binary is optional -- when present, it enhances Stage 1-4 with graph data; when absent, the skill works fine with Claude's native tools (graceful degradation already designed in).

Pros: Clean separation. Plugin is portable. Binary is optional enhancement.
Cons: Two things to maintain and distribute. Plugin format untested with local binary MCP servers.

---

## Open Questions

1. Is 9/10 content quality without the workflow tools "good enough"? If so, Path B is the simplest.
2. How much does the validation layer actually catch? We know it was bypassed in this test and quality was still high. Is that because the SKILL.md and templates are strong, or because this particular decomposition happened to be clean?
3. Should code intelligence be pushed harder in Stages 2-4, or is Stage 1 the natural fit? The test suggests Claude doesn't feel the need after Stage 1 because it already has the context.
4. Is the Go binary justified purely for code intelligence, or is there a lighter way to deliver the same value?
5. Does the answer change if this is distributed externally vs. used by a single team?
