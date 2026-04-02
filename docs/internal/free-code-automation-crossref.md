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

## What to Build: Summary

| ID | What to Build in PD | CC Capability It Leverages | Type | Complexity |
|----|---------------------|---------------------------|------|------------|
| 1A | Review-loop pattern in SKILL.md | Cron/`/loop` | SKILL.md addendum | Low |
| 1B | Native-tool-only review prompt template | Remote triggers/`/schedule` | Asset + template | Medium |
| 2A | Decomposition verification agent | Verification agent spawning | `.claude/agents/` definition | Medium |
| 2B | Convergence tracking in skill | Skill improvement hooks | SKILL.md addendum | Low |
| 3A | Session state summary hook | Memory extraction | `.claude/hooks/` + SKILL.md | Medium |
| 3B | Stage 0 as team memory alignment | Team memory (`TEAMMEM`) | Design intent (future) | Low |
| 4A | Cluster-guided parallel exploration | Explore subagent | SKILL.md conditional workflow | Low-Medium |
| 4B | Plan agent for architecture drafting | Plan subagent | SKILL.md addendum | Low |
| 5A | Token-aware stage pacing guidance | Token budget tracking | SKILL.md addendum | Low |
| 6A | Idempotent stage output guidance | Unattended retry | SKILL.md guidance | Low |

---

## Build Sequence

### Now: SKILL.md Additions (items 1A, 2B, 4A, 4B, 5A, 6A)

Six items are SKILL.md addenda that compose directly with Claude Code capabilities already working in the experimental build. No new files, no binary changes. These can be written and tested in a single session against a real decomposition.

### Next: Custom Agent + Hook (items 2A, 3A)

Two items require new files:
- `.claude/agents/decompose-verifier.md` -- the adversarial plan verification agent
- `.claude/hooks/decompose-session-state.sh` (or equivalent) -- the session state summary hook

These need design, implementation, and testing against real decompositions. The verification agent in particular needs calibration -- too aggressive and it blocks every stage, too permissive and it adds no value over self-review.

### Later: Review Template + Team Memory (items 1B, 3B)

Two items are contingent:
- The native-tool-only review prompt (1B) is worth building when remote triggers stabilize, since it also serves as a fallback for environments without the decompose binary
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
| Feedback loop lacks convergence heuristic | Skill improvement hooks | 2B: Convergence tracking |
| No cross-session state persistence | Memory extraction | 3A: Session state hook |
| Stage 0 duplicates team memory concept | `TEAMMEM` | 3B: Design alignment |
| Sequential Stage 1 discovery bottleneck | Explore/Plan subagents | 4A, 4B: Parallel exploration |
| Unexpected compaction mid-stage | Token budget tracking | 5A: Stage pacing guidance |
| API failures interrupting long sessions | Unattended retry | 6A: Idempotent output guidance |
