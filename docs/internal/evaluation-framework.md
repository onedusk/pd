# Evaluation Framework

Notes on operationalizing the assessment of progressive decomposition -- turning "does this work?" into something measurable.

---

## The Problem

After iterating on the methodology and testing it across projects, results have been described informally: "content quality 9/10," "tool compliance 4/10," "mixed results," "this seems to work." These assessments are real but unstable -- they depend on what the evaluator is paying attention to, and they shift between sessions. The goal is to define what "works" means concretely enough that evaluation is repeatable.

This is an operationalization problem: taking an abstract construct (methodology effectiveness) and decomposing it into measurable dimensions with explicit criteria. The same challenge shows up in psychometrics, survey research, and LLM evaluation.

## Relevant Frameworks

**Likert-anchored decomposition** (Davis, 1989 -- Technology Acceptance Model). Instead of measuring a vague construct directly ("how useful is this?"), break it into constituent dimensions and measure each through specific items. The key insight: each item targets a specific facet, and you triangulate from multiple angles. Applied here, "methodology effectiveness" decomposes into several independent dimensions rather than a single score.

**Rubric-based evaluation.** Define explicit scoring criteria with concrete anchors at each level. Instead of "rate quality 1-5," write anchors like "1 = output missing required sections; 3 = all sections present but some reference stale or incorrect data; 5 = all sections present, all references verified, no manual correction needed." The anchors eliminate ambiguity about what a score means. This is standard in NLP evaluation (MT-Bench, etc.).

**Comparative/pairwise evaluation** (Thurstone, 1920s -- law of comparative judgment). Rather than absolute ratings, compare two outputs: "Which decomposition produced task specs that led to fewer review cycles?" This sidesteps calibration drift (different evaluators interpreting scales differently). Useful for comparing methodology versions -- e.g., with code intelligence vs. without.

**Task-grounded proxies.** Instead of measuring the concept, measure a behavioral outcome the concept predicts. "Effectiveness" becomes: did the implementation pass acceptance criteria on the first attempt? How many review cycles per task? Did the final /review come back clean? This is criterion validity -- checking whether the measure predicts real-world outcomes.

**LLM-as-judge considerations** (Zheng et al., 2023). If using Claude to evaluate decomposition quality: position bias is real (swap order and average), providing reference answers improves agreement with human judgment, and chain-of-thought evaluation prompts (reason before scoring) significantly improve consistency.

## Constructs and Dimensions

Four dimensions that map to what has been informally assessed so far:

### 1. Decomposition Completeness

Does the decomposition produce all expected outputs with all required content?

- 1 = One or more stages missing or empty. Templates not followed.
- 3 = All stages present. Most required sections filled. Some sections thin or generic (not grounded in the actual codebase).
- 5 = All stages present and complete. Every section addresses the specific project. Templates fully populated with no placeholder or generic content.

### 2. Cross-Stage Coherence

Do later stages accurately reference earlier stages?

- 1 = Stage 4 task specs reference files, types, or signatures not present in Stage 2 skeletons. Milestone ordering contradicts dependency graph in Stage 3.
- 3 = All file references resolve. Some signatures have drifted between stages (e.g., Stage 4 assumes a function parameter that Stage 2 doesn't define). Milestone dependencies are correct but task ordering within milestones is suboptimal.
- 5 = Every file path, type name, and function signature in Stage 4 traces back to a Stage 2 skeleton with no discrepancies. Milestone and task ordering matches the dependency DAG exactly. ADR/PDR references are complete.

### 3. Execution Fidelity

When Claude implements from the task specs, does the implementation match?

- 1 = Implementation diverges from task spec. Files created that aren't in the plan. Acceptance criteria not addressed.
- 3 = Implementation follows the task spec's file list and general approach. Some acceptance criteria require interpretation. One or two files need manual correction.
- 5 = Implementation matches task spec exactly. All acceptance criteria pass. /review after implementation finds no issues or only minor style/formatting items.

Behavioral proxies for this dimension (measurable without subjective scoring):
- Number of review cycles per task (lower = better)
- Percentage of acceptance criteria passing on first implementation attempt
- Whether /review after full implementation comes back clean

### 4. Course-Correction Cost

When Claude veers from the plan, how much effort is required to get back on track?

- 1 = Veer requires re-running one or more stages. Task specs need rewriting. Significant manual intervention.
- 3 = Veer caught by /review. Fix is localized to the current task. One additional review cycle resolves it.
- 5 = No meaningful veering. Claude follows the specs. Deviations are minor and caught by acceptance criteria checks.

Behavioral proxy: time/tokens spent on correction vs. time/tokens spent on implementation.

## Evaluation Template

For LLM-as-judge or structured self-assessment after a decomposition run:

```
Context: [project name, codebase size, decomposition name]
Stage evaluated: [which stage or full pipeline]

For each dimension, explain your reasoning first, then score 1-5.

1. Decomposition Completeness
   Criteria: Are all stages present? Are all required sections filled
   with project-specific content (not generic)?
   Reasoning: ...
   Score: ...

2. Cross-Stage Coherence
   Criteria: Do Stage 4 task specs trace back to Stage 2 skeletons
   with no discrepancies in file paths, types, or signatures?
   Reasoning: ...
   Score: ...

3. Execution Fidelity
   Criteria: Did the implementation match the task specs? How many
   review cycles were needed?
   Reasoning: ...
   Score: ...

4. Course-Correction Cost
   Criteria: When deviations occurred, how much effort was needed
   to return to the plan?
   Reasoning: ...
   Score: ...
```

## Comparative Use

When evaluating methodology changes (e.g., with vs. without code intelligence, before vs. after SKILL.md revision), pairwise comparison is more reliable than absolute scoring:

"Given the same project and the same initial prompt, which decomposition run produced task specs that led to fewer review cycles during implementation?"

This avoids the calibration problem of absolute scores shifting over time or between evaluators.

## What This Does Not Cover

This framework evaluates the decomposition methodology. It does not evaluate the quality of the idea, the correctness of the high-level plan, or whether the right project was chosen. Those are upstream decisions that the methodology takes as input.

It also does not evaluate Claude's general coding ability. A task spec could be perfect and Claude could still write buggy code. Execution fidelity measures whether Claude followed the plan, not whether the plan was good or the code was correct. Code correctness is what /review and tests are for.

## Status

This framework is descriptive, not prescriptive. It records what has been informally assessed and gives it structure. Whether to formalize evaluation runs against it is an open decision -- useful if comparing methodology versions or sharing with others, optional for solo iteration where informal assessment is working.
