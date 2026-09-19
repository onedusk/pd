# Claude Code Capabilities Cross-Reference: What to Build Into Progressive Decomposition

**Date:** 2026-04-01
**Status:** Active
**Prerequisites:** recommendations.md, complete_flow.md, free-code/FEATURES.md (2026-03-31 audit), free-code/changelog.md
**Architecture context:** Path B+C hybrid (skill-primary, code intel as query tools, PreToolUse advisory for review findings)

---

## Why This Document Exists

Claude Code is gaining capabilities -- agent triggers, verification agents, memory systems, parallel subagents, token awareness -- that are currently behind feature flags but moving toward general availability. Progressive-decomposition's architecture (skill-driven, single-agent, code intelligence as query tools) is well-positioned to absorb these capabilities because the skill is already the workflow engine and Claude Code is already the runtime.

This document identifies what we should **build into progressive-decomposition** -- new skill logic, custom agents, hooks, and workflow patterns -- that leverage Claude Code's evolving native features. The goal is to close the pipeline's known automation gaps by designing pd features that compose with Claude Code capabilities rather than reimplementing them.

The FEATURES.md audit (2026-03-31) of a feature-unlocked Claude Code fork provides the source-of-truth for what's coming. Flags that bundle cleanly today and have working infrastructure are likely to ship. Flags that are broken or have large missing subsystems are noted for future reference only.

**Framing:** For each opportunity below, the Claude Code capability is the foundation. The question is: what does progressive-decomposition need to add on top?

---

## Capability Readiness Reference

Claude Code features relevant to pd, with their current readiness. Features marked GA are already shipping. Features marked "experimental (working)" bundle cleanly and have complete infrastructure. Features marked "broken" are excluded from actionable recommendations.

| Capability | Claude Code Feature | Status | Notes |
|-----------|-------------------|--------|-------|
| Cron/recurring tasks | `AGENT_TRIGGERS`, `/loop` skill | Experimental (working) | Durable + session-only jobs, idle-only firing |
| Remote scheduled agents | `AGENT_TRIGGERS_REMOTE`, `/schedule` skill | Experimental (working) | Requires claude.ai OAuth |
| Adversarial verification | `VERIFICATION_AGENT` | Experimental (working) | Gated, defaults off |
| Explore/Plan subagents | `BUILTIN_EXPLORE_PLAN_AGENTS` | Experimental (working) | Defaults on |
| Post-query memory extraction | `EXTRACT_MEMORIES` | Experimental (working) | Fire-and-forget at session end |
| Agent memory snapshots | `AGENT_MEMORY_SNAPSHOT` | Experimental (working) | AppState persistence |
| Skill self-improvement | `SKILL_IMPROVEMENT` | Experimental (working) | Gated, defaults off |
| Token budget tracking | `TOKEN_BUDGET` | Experimental (working) | Observability surface |
| Context management | `CACHED_MICROCOMPACT`, `COMPACTION_REMINDERS` | Experimental (working) | Performance + UX |
| Resilient retry | `UNATTENDED_RETRY` | Experimental (working) | Automatic API retry |
| Team memory | `TEAMMEM` | Experimental (working) | Shared memory files |
| Plugin executables | Plugin `bin/` directory | GA (2.1.91) | Plugins ship pre-built binaries, invokable as bare Bash commands |
| Workflow automation | `WORKFLOW_SCRIPTS` | Broken | Missing command + tool impl |
| Implementation monitoring | `MONITOR_TOOL` | Broken | Missing tool impl |
| Template jobs | `TEMPLATES` | Broken | Missing CLI handler |

---

## What to Build: Opportunities by Category

### 1. Automated Review During Implementation

**Claude Code provides:** Cron tools (`CronCreate`, `CronDelete`, `CronList`), the `/loop` skill for recurring prompts, and remote agent triggers (`/schedule`) for cross-session automation.

**What pd should build:**

#### 1A. A `/decompose review-loop` Pattern in SKILL.md

Today the SKILL.md says: run `/review` after each task, after each milestone, and once across the full scope. The user must remember every time.

**Build:** Add a dedicated section to the SKILL.md implementation flow that instructs Claude to offer setting up a review loop when implementation begins. The instruction should:
- Suggest `/loop 30m run decompose review <name> and report new findings since last check` at the start of any implementation session
- Track review-loop state -- if findings accumulate between loops, surface them at the next idle point rather than waiting for the user to ask
- On milestone completion, automatically trigger a full `/review` before proceeding to the next milestone (not just on a timer)

This is a SKILL.md workflow addition -- no binary changes. It turns Claude Code's cron infrastructure into a review cadence that the skill manages.

**Complexity:** Low. SKILL.md addendum only.

#### 1B. A Review Agent Prompt for Remote Scheduling

For multi-day implementations, the local session dies. Claude Code's remote triggers can schedule agents that clone the repo and run on a cron.

**Build:** A self-contained review prompt template (stored in `.claude/skills/decompose/assets/`) that a remote agent can execute without the decompose binary. This prompt would:
- Read the Stage 3 directory tree and Stage 4 task specs
- Walk the filesystem to verify file existence (Check 1)
- Grep for expected symbols in MODIFY targets (Check 2, approximate)
- Compare milestone ordering against import statements (Check 3, approximate)
- Report findings in the same format as `review-findings.md`

This is a "native-tool-only review" -- a degraded but still useful version of the 5 mechanical checks that works anywhere Claude Code runs, without CGO dependencies. The decompose binary's graph-backed checks remain the gold standard for local sessions.

**Complexity:** Medium. Requires writing and testing the review prompt template. The prompt must be self-contained (remote agents start with zero context).

**Prerequisite:** Claude Code remote triggers reaching GA, or testing with the current experimental flag.

---

### 2. Adversarial Post-Stage Verification

**Claude Code provides:** A built-in verification agent with an adversarial prompt designed to combat "verification avoidance." It operates read-only (cannot edit files) and produces structured PASS/FAIL/PARTIAL verdicts.

**What pd should build:**

#### 2A. A Decomposition Verification Agent

The built-in verification agent is designed for code ("try to break the implementation"). Plan verification needs a different adversarial lens: "try to break the spec."

**Build:** A custom agent definition at `.claude/agents/decompose-verifier.md` that:
- Receives one stage output + all prior stages as context
- Checks **completeness** -- are all template sections present and non-trivial?
- Checks **cross-stage coherence** -- do Stage 2 skeletons match Stage 1 data model? Does Stage 3 DAG reference every Stage 2 skeleton? Do Stage 4 tasks cover every Stage 3 milestone?
- Checks **grounding** -- do file paths in task specs exist (or are correctly marked CREATE)? Do type names match the codebase?
- Produces a structured verdict: PASS (proceed), PARTIAL (specific sections need revision), FAIL (stage needs rework)
- Cannot write files (enforced via `disallowedTools` in agent frontmatter)

Then update SKILL.md to spawn this agent after each stage is written. This replaces the current self-review ("read what you just wrote") with a genuine second opinion -- the verification agent has a separate context window, separate system prompt, and adversarial framing.

**Complexity:** Medium. Writing the agent definition is straightforward (the adversarial verification pattern from Claude Code's built-in agent is the template). The SKILL.md change is a conditional spawn at each stage boundary. Testing requires running it against real decompositions to calibrate false positive rate.

**Why this matters:** complete_flow.md identified the "same agent reviewing its own output" tension. This solves it using Claude Code's native agent spawning, without reintroducing multi-agent pipeline coordination (A2A remains shelved).

#### 2B. Convergence Tracking in the Skill

Claude Code's skill improvement hooks automatically detect recurring user corrections and propose permanent skill updates.

**Build:** Add convergence-aware language to the SKILL.md iteration sections. When a user corrects a stage output, the skill should:
- Note the correction category (data model, architecture, naming, scope, dependency)
- If the same category is corrected 3+ times across sessions, flag it as a methodology gap rather than a one-time issue
- Suggest the correction be added to Stage 0 (Development Standards) if it's a team norm, or to the SKILL.md if it's a process improvement

This works with or without the `SKILL_IMPROVEMENT` flag. With the flag enabled, Claude Code's hook automates the "propose permanent skill update" step. Without it, the SKILL.md instruction makes the agent track corrections manually within the session.

**Complexity:** Low (SKILL.md addendum). Medium if also writing the skill improvement integration to ensure the decompose skill is recognized as a project skill by the hook.

#### 2C. `run_coherence` -- Mechanical Cross-Stage Verification in the Binary

Items 2A (verification agent) and the existing `run_review` address different verification surfaces: `run_review` checks the plan against the codebase (do referenced files and symbols exist?), while 2A uses an LLM to check the plan against itself (is the spec internally consistent?). Neither provides deterministic, repeatable cross-stage structural verification.

**The gap:** Certain cross-stage consistency checks are mechanical -- they don't require LLM judgment, just parsing and comparison. An LLM-based agent might catch these, or might not, depending on context pressure and attention. A binary command catches them every time.

**Build:** A new `run_coherence` subcommand in the decompose binary (`decompose coherence <name>`) and a corresponding `run_coherence` MCP tool. It reads all stage files for a named decomposition and runs structural checks across stages:

1. **Stage 1 -> Stage 2: Data model coverage.** Parse entity names from Stage 1's data model section. Verify each has a corresponding type definition in Stage 2. Report any Stage 1 entities missing from Stage 2 skeletons.
2. **Stage 2 -> Stage 3: Skeleton coverage.** Parse type/interface names from Stage 2. Verify each appears in at least one Stage 3 milestone's file list. Report skeletons that aren't assigned to any milestone.
3. **Stage 3 -> Stage 4: Milestone coverage.** Parse milestone IDs and file lists from Stage 3. Verify each milestone has a corresponding `tasks_mNN.md` file. Verify every file listed in Stage 3 appears in at least one Stage 4 task. Report gaps.
4. **Stage 4 -> Stage 2: Task-skeleton alignment.** For tasks that reference Stage 2 types (e.g., "copy the User model from Stage 2"), verify the referenced type exists in Stage 2. Report stale references.
5. **Cross-stage naming consistency.** Collect all entity/type/file names across stages. Flag cases where the same concept uses different names in different stages (e.g., `UserAccount` in Stage 1 but `User` in Stage 2).

Findings use the same classification as `run_review`: MISMATCH (stages contradict each other), OMISSION (something in an earlier stage has no downstream coverage), STALE (a later stage references something removed from an earlier stage).

**How it relates to 2A:** `run_coherence` and the verification agent are complementary, not redundant. `run_coherence` catches structural mismatches deterministically -- it will never miss a Stage 1 entity that's absent from Stage 2. The verification agent catches semantic problems that require judgment -- an entity is present in Stage 2 but its field types don't match the Stage 1 description. Run `run_coherence` first (fast, deterministic), then the verification agent (slower, judgment-based) on whatever passes.

**How it relates to `run_review`:** `run_review` checks plan-vs-codebase (do the files and symbols the plan references actually exist?). `run_coherence` checks plan-vs-plan (do the stages reference each other correctly?). They run at different times: `run_coherence` runs between stages as they're written, `run_review` runs after Stage 4 is complete.

**Integration with SKILL.md:** Update `references/review.md` to call `run_coherence` before `run_review` in the review phase. Optionally, add per-stage coherence checks to each stage reference file (e.g., `references/stage-2.md` could instruct: "after writing, run `run_coherence` to verify all Stage 1 entities have skeleton coverage"). This catches drift early rather than waiting until the full review phase.

**Complexity:** Medium-High. Requires parsing stage markdown files to extract entity names, milestone IDs, file lists, and type definitions. The parsing doesn't need to be perfect -- heuristic extraction (heading patterns, code block contents, file path patterns) is sufficient since the stage templates enforce consistent structure. The MCP tool wrapper and CLI subcommand are straightforward given the existing `run_review` pattern.

**Prerequisite:** The decompose binary must be available (CGO build with tree-sitter + KuzuDB). Environments without the binary fall back to the verification agent (2A) for cross-stage checks.

---

### 3. Cross-Session Decomposition State

**Claude Code provides:** Post-query memory extraction that persists observations as memory files, and agent memory snapshots that capture state in AppState.

**What pd should build:**

#### 3A. A Decomposition State Summary Hook

Today, when a decomposition session ends, all in-flight context is lost. The next session re-reads stage files but loses: which architectural decisions were debated and why, which review findings were addressed vs. deferred, which Stage 1 sections are complete vs. in-progress.

**Build:** A PostToolUse or Stop hook (in `.claude/hooks/`) that fires when a decomposition session is ending (detected by: the session wrote to `docs/decompose/<name>/`). The hook produces a `decompose-session-state.md` file in the decomposition directory containing:
- Current stage and section progress
- Key decisions made (and their rationale)
- Open questions and unresolved review findings
- Next recommended action

This file becomes a structured resumption artifact. The SKILL.md instruction for "continuing a decomposition" reads this file first, before re-reading stage outputs.

Claude Code's `EXTRACT_MEMORIES` provides the underlying memory extraction capability. The hook channels it into decomposition-specific structure rather than generic memories.

**Complexity:** Medium. Requires writing the hook, designing the state summary format, and adding "read session state on resume" logic to the SKILL.md.

#### 3B. Stage 0 as Team Memory

Claude Code's `TEAMMEM` feature enables shared memory files with watcher hooks.

**Build:** Recognize that Stage 0 (Development Standards) is conceptually identical to team memory -- it's norms written once and shared across all decompositions. If `TEAMMEM` ships as a native feature:
- Stage 0 content could live in `.claude/team-memory/` (or equivalent) where Claude Code natively loads it into every session
- This eliminates the current pattern where Stage 0 is read from `docs/decompose/stage-0-development-standards.md` and injected into context manually
- The SKILL.md Stage 0 workflow would shift from "write this file" to "populate team memory with these norms"

This is a forward-looking alignment. Stage 0 and team memory serve the same function -- making org-level norms automatically available. If Claude Code ships team memory natively, pd should plug into it rather than maintaining a parallel mechanism.

**Complexity:** Low (design alignment now, implementation when TEAMMEM ships). No immediate code changes -- just a documented intent to converge.

---

### 4. Parallel Codebase Discovery for Stage 1

**Claude Code provides:** Built-in Explore and Plan subagents. Explore is a fast, read-only agent optimized for codebase search. Plan is a software architect agent that designs implementation approaches.

**What pd should build:**

#### 4A. Cluster-Guided Parallel Exploration in SKILL.md

Stage 1 discovery on large codebases is sequential today. Claude Code's Explore agent can parallelize it.

**Build:** Add a conditional workflow to SKILL.md Stage 1:

1. If the decompose binary is available, run `build_graph` first. Use `get_clusters` to identify architectural boundaries.
2. Spawn one Explore agent per cluster (up to 3) with a focused discovery prompt: "Explore this cluster's files. Report: public API surface, key data types, external dependencies, and integration points with other clusters."
3. Merge the Explore agent outputs into the Stage 1 architecture and features sections.
4. If the binary is not available, fall back to the current sequential approach.

This leverages Claude Code's native subagent spawning and pd's code intelligence together. The graph provides the map (which clusters exist), the Explore agents provide the depth (what's in each cluster).

**Complexity:** Low-Medium. The SKILL.md change is a conditional block. The Explore agents need well-scoped prompts that produce mergeable output. Test on the dusk codebase (2,266 files) to validate that parallel exploration actually outperforms sequential for Stage 1 quality.

#### 4B. Plan Agent for Architecture Drafting

**Build:** After Explore agents report their findings, spawn a Plan agent with the combined results to draft the architecture section of Stage 1. The Plan agent operates in read-only mode and produces a structured architecture assessment: component boundaries, dependency directions, key abstractions, and identified patterns.

The main agent then reviews and integrates the Plan agent's draft rather than writing the architecture section from scratch. This is faster and produces a second perspective on architectural boundaries.

**Complexity:** Low. Additive SKILL.md instruction. The Plan agent already exists.

---

### 5. Token-Aware Stage Pacing

**Claude Code provides:** Token budget tracking with prompt triggers and warning UI. Compaction reminders. Cached microcompact state.

**What pd should build:**

#### 5A. Stage-Aware Context Budget Management in SKILL.md

Long decompositions (especially Stage 1 on large codebases and Stage 4 with many milestones) can exhaust context. Today there's no guidance for the agent on how to handle this.

**Build:** Add context management instructions to the SKILL.md:
- Before starting a stage, estimate its context cost (Stage 1: proportional to codebase size; Stage 4: proportional to milestone count)
- If token budget warnings appear mid-stage, complete the current section and review checkpoint before proceeding to the next section
- If a stage will clearly exceed context limits, split it across sessions: complete and write sections incrementally, relying on the session state hook (3A) for continuity
- For Stage 4 specifically: write one milestone file at a time, review it, then proceed. Don't try to hold all milestones in context simultaneously.

This is methodology guidance that becomes actionable because Claude Code's token budget feature provides the visibility. Without token budget tracking, the agent can't make informed pacing decisions.

**Complexity:** Low. SKILL.md addendum only. Depends on `TOKEN_BUDGET` being available at runtime.

---

### 6. Resilient Long-Running Sessions

**Claude Code provides:** Unattended retry for transient API failures.

**What pd should build:**

#### 6A. Idempotent Stage Outputs

Claude Code's unattended retry means a failed API call mid-stage-write will be retried automatically. But for this to be safe, stage outputs need to be idempotent -- writing the same section twice shouldn't corrupt the stage file.

**Build:** Add guidance to the SKILL.md that stage output sections are written atomically:
- Write each stage file as a complete unit (not appended section by section)
- If a stage file already exists, re-read it before writing to avoid overwriting completed sections
- Stage 4 milestone files are naturally idempotent (one file per milestone, complete replacement)

This is a minor SKILL.md refinement that makes unattended retry safe for decomposition workflows. No binary changes.

**Complexity:** Low. SKILL.md guidance only.

---

### 7. Plugin Distribution for the Decompose Binary

**Claude Code provides:** As of 2.1.91, plugins can ship executables under `bin/` and invoke them as bare commands from the Bash tool. This is a GA capability, not behind a feature flag.

**What pd should build:**

#### 7A. Package Progressive Decomposition as a Plugin with Pre-Built Binaries

Today the decompose binary requires users to run `make build` with CGO enabled (tree-sitter + KuzuDB dependencies), then manually configure `.claude/mcp.json`. This setup barrier means the binary is effectively optional, and most users rely on the skill alone without code intelligence or mechanical review checks.

**Build:** Package progressive-decomposition as a Claude Code plugin (`.plugin` format) that includes:
- The `/decompose` skill (SKILL.md + reference files + templates + process guide)
- Pre-built `decompose` binaries under `bin/` for target platforms (darwin-arm64, darwin-x64, linux-x64 at minimum)
- MCP server configuration bundled in the plugin manifest, so the `decompose` MCP tools register automatically on install

The user installs the plugin and gets the full stack: skill, templates, binary, and MCP tools. No build step, no CGO dependency, no manual config.

**Binary invocation model.** Two options exist and may coexist:

1. **MCP server (current model).** The binary runs as a stdio MCP server. Plugin-bundled MCP config registers the tools automatically. The skill invokes tools through the MCP protocol with structured JSON responses. This is the richer interface -- tool definitions are discoverable, responses are typed, and Claude Code's tool infrastructure handles retries and error formatting.

2. **Direct Bash invocation.** The `bin/` executable feature allows the skill to call `decompose review auth-system` directly from Bash and parse the text output. This is simpler for one-shot commands (review, coherence check, status) where the overhead of a running MCP server adds no value. It also works as a fallback if MCP registration fails.

The recommended approach: ship both. MCP for the persistent tools used during decomposition (build_graph, query_symbols, get_dependencies, assess_impact, get_clusters), direct Bash for the one-shot commands used at stage boundaries and during review (run_review, run_coherence, status). This avoids keeping a long-running MCP server alive for commands that run once and exit.

**Platform-specific binaries.** The binary includes CGO dependencies (tree-sitter grammars, KuzuDB). Cross-compilation with CGO is non-trivial. The build matrix needs:
- darwin-arm64 (Apple Silicon Macs -- primary user base)
- darwin-x64 (Intel Macs -- declining but still present)
- linux-x64 (CI environments, remote Claude Code sessions, Cowork VMs)

A GitHub Actions workflow with platform-specific runners (or cross-compilation via Zig CC) can produce these. The plugin build step compiles all three, places them under `bin/darwin-arm64/decompose`, `bin/darwin-x64/decompose`, `bin/linux-x64/decompose`, and the plugin manifest selects the correct one at install time. (The exact mechanism for platform selection in plugins needs investigation -- it may require separate plugin builds per platform, or the plugin format may support a platform map.)

**Binary size.** Tree-sitter grammars and KuzuDB add non-trivial weight. Static linking produces a binary in the 20-40MB range depending on which grammars are included. This is acceptable for a plugin but worth monitoring. If size becomes a concern, grammar selection could be configurable at build time (only include Go + TypeScript + Python by default, add others on request).

**What this unlocks beyond distribution:**
- The binary stops being optional and becomes the default path. Code intelligence and mechanical review become standard rather than power-user features.
- Plugin updates deliver binary improvements without requiring users to rebuild.
- The headless verification agent (2A) can invoke `bin/decompose` directly, making it a self-contained verification pipeline without MCP coordination.
- Item 1B (native-tool-only review for remote agents) becomes less critical, since the plugin binary provides the full review capability wherever the plugin is installed.

**Complexity:** Medium-High. The plugin packaging itself is straightforward (skill files + binary + manifest). The complexity is in the cross-platform build pipeline (CGO cross-compilation), platform selection in the plugin format (needs investigation), and ensuring the MCP config registers correctly on install. Once the build pipeline exists, subsequent releases are automated.

**Prerequisites:** Investigate the Claude Code plugin format specification -- specifically how `bin/` executables are resolved per platform, whether MCP configs can be bundled in the manifest, and whether there are size limits on plugin packages. The `create-cowork-plugin` skill may have relevant guidance.

---

## What to Build: Summary

| ID | What to Build in PD | CC Capability It Leverages | Type | Complexity |
|----|---------------------|---------------------------|------|------------|
| 1A | Review-loop pattern in SKILL.md | Cron/`/loop` | SKILL.md addendum | Low |
| 1B | Native-tool-only review prompt template | Remote triggers/`/schedule` | Asset + template | Medium |
| 2A | Decomposition verification agent | Verification agent spawning | `.claude/agents/` definition | Medium |
| 2B | Convergence tracking in skill | Skill improvement hooks | SKILL.md addendum | Low |
| 2C | `run_coherence` cross-stage verification | Decompose binary (tree-sitter, markdown parsing) | Binary subcommand + MCP tool | Medium-High |
| 3A | Session state summary hook | Memory extraction | `.claude/hooks/` + SKILL.md | Medium |
| 3B | Stage 0 as team memory alignment | Team memory (`TEAMMEM`) | Design intent (future) | Low |
| 4A | Cluster-guided parallel exploration | Explore subagent | SKILL.md conditional workflow | Low-Medium |
| 4B | Plan agent for architecture drafting | Plan subagent | SKILL.md addendum | Low |
| 5A | Token-aware stage pacing guidance | Token budget tracking | SKILL.md addendum | Low |
| 6A | Idempotent stage output guidance | Unattended retry | SKILL.md guidance | Low |
| 7A | Plugin packaging with pre-built binaries | Plugin `bin/` executables (GA) | Plugin package + CI pipeline | Medium-High |

---

## Build Sequence

### Now: SKILL.md Additions (items 1A, 2B, 4A, 4B, 5A, 6A)

Six items are SKILL.md addenda that compose directly with Claude Code capabilities already working in the experimental build. No new files, no binary changes. These can be written and tested in a single session against a real decomposition.

### Next: Custom Agent, Hook, and Binary Command (items 2A, 2C, 3A)

Three items require new files:
- `.claude/agents/decompose-verifier.md` -- the adversarial plan verification agent (2A)
- `run_coherence` subcommand + MCP tool in the decompose binary (2C)
- `.claude/hooks/decompose-session-state.sh` (or equivalent) -- the session state summary hook (3A)

These need design, implementation, and testing against real decompositions. The verification agent (2A) needs calibration -- too aggressive and it blocks every stage, too permissive and it adds no value over self-review. `run_coherence` (2C) needs a markdown parsing strategy for extracting entity names and file lists from stage files; heuristic extraction based on template structure is sufficient. The recommended verification order is: `run_coherence` first (deterministic structural checks), then 2A (LLM-based semantic checks), then `run_review` (plan-vs-codebase checks).

### Later: Plugin Packaging, Review Template + Team Memory (items 7A, 1B, 3B)

Three items are contingent or require infrastructure work:
- Plugin packaging (7A) requires investigating the plugin format spec, building a cross-platform CI pipeline, and validating MCP config bundling. This is the highest-impact item in the "Later" group because it makes the binary the default path for all users, but it depends on the binary itself being stable (items 2C and the existing run_review need to be solid first). Once shipped, it reduces the urgency of 1B since the binary's review checks travel with the plugin.
- The native-tool-only review prompt (1B) is worth building when remote triggers stabilize, since it also serves as a fallback for environments without the decompose binary. If 7A ships first, 1B becomes a fallback for remote-only environments where plugins aren't installed.
- The Stage 0 / team memory alignment (3B) is a design intent to converge when `TEAMMEM` ships natively -- no immediate code needed

---

## Broken Flags Worth Watching

Three broken Claude Code features would be high-value if reconstructed. FEATURES.md documents their reconstruction paths (single missing files each):

| Flag | Missing File | If Restored, Enables |
|------|-------------|---------------------|
| `WORKFLOW_SCRIPTS` | `src/commands/workflows/index.js` | Native workflow orchestration that could drive the decompose pipeline stages programmatically |
| `MONITOR_TOOL` | `src/tools/MonitorTool/MonitorTool.js` | Implementation monitoring tool that could watch milestone progress |
| `TEMPLATES` | `src/cli/handlers/templateJobs.js` | Template-driven job dispatch that could automate stage template population |

These are noted for future cross-reference. If any are reconstructed in a future free-code snapshot, re-evaluate against the opportunities above.

---

## Appendix: PD Gap to CC Capability Mapping

| Progressive Decomposition Gap | CC Capability | PD Build Item |
|------------------------------|--------------|---------------|
| Manual review cadence during implementation | Cron/`/loop` | 1A: Review-loop pattern |
| No cross-session review monitoring | Remote triggers | 1B: Native-tool review template |
| Same-agent self-review tension | Verification agent | 2A: Decomposition verifier agent |
| Cross-stage structural drift (entities, milestones, file lists) | Decompose binary | 2C: `run_coherence` command |
| Feedback loop lacks convergence heuristic | Skill improvement hooks | 2B: Convergence tracking |
| No cross-session state persistence | Memory extraction | 3A: Session state hook |
| Stage 0 duplicates team memory concept | `TEAMMEM` | 3B: Design alignment |
| Sequential Stage 1 discovery bottleneck | Explore/Plan subagents | 4A, 4B: Parallel exploration |
| Unexpected compaction mid-stage | Token budget tracking | 5A: Stage pacing guidance |
| API failures interrupting long sessions | Unattended retry | 6A: Idempotent output guidance |
| Binary requires manual build + CGO + MCP config | Plugin `bin/` executables | 7A: Plugin distribution |
