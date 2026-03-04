# Empirical Evaluation Framework for Progressive Decomposition

## Motivation

Progressive Decomposition makes several testable claims:

1. **Staged refinement reduces ambiguity** vs single-step decomposition
2. **Stage 2 (Implementation Skeletons) surfaces design issues** that prose specifications miss
3. **Output task specs are executable** with less back-and-forth than alternatives
4. **The pipeline works across stacks/team compositions**

None of these have empirical data behind them. The formal foundations section sets up the theoretical argument; what's missing is evidence showing the gap between pd and alternatives on real projects.

---

## What pd Actually Claims (Constructs to Measure)

The README and process guide assert several things: that staged refinement produces fewer ambiguities than single-step decomposition, that code skeletons (Stage 2) surface design issues that prose specs miss, that the output task specs are executable with less back-and-forth, and that the pipeline works across stacks/team compositions. Each of these is a separate measurable construct.

---

## Evaluation Approaches

### 1. Rubric-Based Evaluation on Output Quality

The most direct approach. Take a set of project ideas of varying complexity, run them through pd's pipeline, and evaluate the Stage 4 output against concrete criteria:

- **Ambiguity rate**: What percentage of tasks require clarification before an implementer can start? Score each task 0 (blocked, needs clarification) / 1 (partially actionable) / 2 (fully actionable as written). This is behavioral and binary-ish, not subjective.
- **Completeness**: Does the task index account for all requirements in the design pack? You can check this mechanically by tracing requirements forward.
- **File-level accuracy**: Do the named files and actions (CREATE/MODIFY/DELETE) actually make sense given the skeleton? Again, verifiable.

### 2. Comparative Evaluation (Strongest Signal)

Take the same project idea, decompose it with pd and with a baseline (e.g., a single-prompt "give me a task list," or GitHub's 3-stage spec kit flow, or just a senior engineer writing tasks freehand). Then either:

- Have implementers (human or AI) attempt to execute both sets of tasks and measure time-to-completion, error rate, and number of clarification questions asked. This is the criterion validity approach — you're measuring downstream outcomes, not opinions.
- Use pairwise LLM-as-judge: "Given this project idea and these two task specifications, which would require less clarification to implement? Explain your reasoning, then choose A or B." Run with position swapping.

### 3. Skeleton-Specific Measurement (Stage 2's Value-Add)

Since Stage 2 is pd's core differentiator, isolate its contribution. Run the pipeline with and without Stage 2, then compare the Stage 4 outputs. The specific hypothesis is that skipping skeletons produces tasks with more type ambiguities (nullable vs. required, missing interface contracts, wrong field types). You can count these mechanically if you have both a skeleton and the resulting task specs.

### 4. What's Hard to Measure Well

The "works for any stack/team" claim is a generalizability claim requiring breadth of evaluation across different project types. The "feedback loops converge" claim from the formal foundations section is interesting but would need tracking how many iterations people actually go through and whether the changes are genuinely additive-only. Both require volume — many projects, many users — which is a cold-start problem for a new tool.

### Primary Metric

**Number of clarification questions before implementation can start** — it's concrete, countable, and directly maps to the core value proposition that staged decomposition reduces ambiguity. It's also the metric most likely to show a clear difference if the methodology works as described, since baseline approaches genuinely do leave more gaps.

### Practical Starting Point

Start with the comparative approach on 3-5 project ideas spanning different complexity levels. Use a mix of human implementers and AI agents as the executors.

---

## Implementation Design

### Architecture Overview

A new `internal/eval/` package and a `decompose eval` CLI subcommand. Builds on three existing subsystems:

1. **Verification system** (`internal/orchestrator/verification.go`) — the `VerificationRule` pattern extended with evaluation-specific rule sets. Reuses `extractEntityNames`, `extractFeatureNames`, `extractTreeFiles` for cross-stage tracing.
2. **Export system** (`internal/export/json.go`) — `TaskExport` already parses task IDs, file actions, dependencies, and acceptance criteria from Stage 4 markdown. All scorers operate on `[]TaskExport`.
3. **Status system** (`internal/status/status.go`) — `GetDecompositionStatus` determines which stages are complete and locates their files.

Four key concepts:
- **Scorer**: Interface for metric-specific evaluation logic. Each scorer receives parsed tasks + stage content, returns scored results.
- **EvalReport**: Structured output (JSON + markdown) of all metric scores for a single decomposition.
- **ComparisonReport**: Paired eval reports from two decompositions with delta analysis.
- **EvalConfig**: Runtime configuration including which metrics to run, comparison mode, and LLM-as-judge enablement.

---

## Milestone 1: Core Types + Rubric Scorers

No dependencies. New `internal/eval/` package with four rule-based scorers.

### `internal/eval/types.go` — Core Types

```go
type MetricName string // "ambiguity", "completeness", "file-accuracy", "clarifications"

type TaskScore struct {
    TaskID, Reason string; Metric MetricName; Score, MaxScore int; Suggestions []string
}

type MetricResult struct {
    Metric MetricName; TotalScore, MaxPossible int; Percentage float64
    TaskScores []TaskScore; Summary string
}

type EvalReport struct {
    Name, DecompositionName, Variant, Summary string
    EvaluatedAt time.Time
    Metrics []MetricResult
    Verification *orchestrator.VerificationReport
    ClarificationCount, TaskCount int
}

type EvalConfig struct {
    ProjectRoot, Name, Variant string
    Metrics []MetricName  // empty = all
    LLMJudge, Verbose bool
}
```

### `internal/eval/scorer.go` — Scorer Interface

```go
type Scorer interface {
    Name() MetricName
    Score(tasks []export.TaskExport, stages StageContent) *MetricResult
}

type StageContent struct {
    Stage0, Stage1, Stage2, Stage3 string
    Stage4 map[string]string // milestone key -> content
}

func RunScorers(cfg EvalConfig, tasks []export.TaskExport, stages StageContent, scorers []Scorer) *EvalReport
func DefaultScorers() []Scorer  // returns all four
```

### `internal/eval/loader.go` — Data Loading

Reuses `export.ExportDecomposition` and `status.GetDecompositionStatus`:

```go
func LoadDecomposition(projectRoot, name string) (*export.DecompositionExport, StageContent, error)
```

### `internal/eval/ambiguity.go` — Ambiguity Scorer

Per-task 0/1/2 scoring:
- **0 (blocked)**: No acceptance criteria AND no file actions
- **1 (partial)**: Has acceptance OR file actions, but not both; or outline lacks concrete identifiers
- **2 (fully actionable)**: Has acceptance + file actions + backtick-quoted identifiers or specific paths

"Concrete" detection: backtick-quoted names, file path patterns, PascalCase type names. All regex-based, no LLM.

### `internal/eval/completeness.go` — Completeness Scorer

Traces Stage 1 requirements → Stage 4 tasks:
1. Extract feature names from Stage 1 (reuse exported `ExtractFeatureNames`)
2. Extract entity names from Stage 1 data model (reuse exported `ExtractEntityNames`)
3. For each requirement, check if any task references it (case-insensitive match in title + file actions)
4. Score = covered / total requirements

### `internal/eval/fileaccuracy.go` — File Accuracy Scorer

Validates file actions against Stage 2/3:
1. Parse file actions from `TaskExport.FileActions` (already parsed as `"CREATE path"`)
2. Parse Stage 3 directory tree (reuse exported `ExtractTreeFiles`)
3. CREATE: file should appear in Stage 3 tree
4. MODIFY: file should be referenced in Stage 2 or Stage 3
5. Orphans: files in Stage 3 tree not covered by any task
6. Score = valid / total actions

### `internal/eval/clarifications.go` — Clarification Scorer (Primary Metric)

A task needs clarification if ANY of:
- No acceptance criteria
- No file actions
- Acceptance criteria aren't binary (heuristic: no action verbs like "compiles", "passes", "returns")
- Dependencies reference non-existent task IDs

`ClarificationCount` = number of tasks needing >= 1 clarification. Each task gets a list of specific questions that would be needed.

### `internal/eval/report.go` — Output Formatting

```go
func (r *EvalReport) JSON() ([]byte, error)    // indented JSON
func (r *EvalReport) Markdown() string          // human-readable with metrics table
```

### Modification: `internal/orchestrator/verification.go`

Export three helpers (keep unexported aliases for backward compat):

```go
func ExtractEntityNames(content string) []string
func ExtractFeatureNames(content string) []string
func ExtractTreeFiles(content string) []string
```

### `internal/eval/scorer_test.go`

Table-driven tests for all four scorers:
- Ambiguity: fully actionable (score 2), blocked (score 0), partial (score 1)
- Completeness: all covered (100%), some missing (< 100%)
- File accuracy: all valid (100%), orphans reported
- Clarifications: well-formed (0), missing acceptance (> 0)

---

## Milestone 2: CLI Command + Comparative Evaluation

Depends on: M1.

### `cmd/decompose/eval.go` — CLI Handler

Following `export.go` pattern:

```
decompose eval <name>                       Score single decomposition
decompose eval compare <nameA> <nameB>      Compare two decompositions
decompose eval skeleton <name>              Stage 2 ablation study
decompose eval aggregate <name1> ...        Cross-project summary
```

Eval-specific flags parsed within `runEval`:
- `--format json|md` (default: md to stdout)
- `--metrics ambiguity,completeness,...` (default: all)
- `--save` — persist results to `eval-<variant>.json`
- `--threshold N` — significance threshold for comparison (default: 5%)

### Modification: `cmd/decompose/main.go`

Add `eval` subcommand dispatch:

```go
if len(positional) > 0 && positional[0] == "eval" {
    return runEval(projectRoot, positional[1:], flags)
}
```

Add `eval` to `printUsage`.

### `internal/eval/compare.go` — Comparison Engine

```go
func Compare(a, b *EvalReport, threshold float64) *ComparisonReport

type ComparisonReport struct {
    EvalA, EvalB *EvalReport
    Deltas []MetricDelta
    Winner, Summary string
}

type MetricDelta struct {
    Metric MetricName; ScoreA, ScoreB, Delta float64; Significant bool
}
```

Winner: count metrics where each side wins. Clarification count weighted 2x. Ties broken by total score.

---

## Milestone 3: Persistence + Aggregation

Depends on: M2.

### `internal/eval/store.go` — Result Persistence

```go
func EvalStorePath(outputDir, variant string) string  // docs/decompose/<name>/eval-<variant>.json
func SaveReport(outputDir string, report *EvalReport) error
func LoadReport(path string) (*EvalReport, error)
```

### `internal/eval/aggregate.go` — Cross-Project Summary

```go
type AggregateReport struct {
    Projects []ProjectEval; Averages MetricAverages; Summary string
}
func Aggregate(reports []*EvalReport) *AggregateReport
```

For the "3-5 project comparative study" — loads saved eval reports, computes mean/median across metrics.

### `internal/eval/skeleton_ablation.go` — Stage 2 Value-Add Measurement

Convention: `<name>` for full pipeline, `<name>-no-skeleton` for ablated run. Both use same file structure so `LoadDecomposition` works on both.

```go
func SkeletonAblation(withSkeleton, withoutSkeleton *EvalReport) *ComparisonReport
```

Adds hypothesis-specific analysis: delta in type-related ambiguity scores, delta in file action coverage, difference in clarification count.

---

## Milestone 4: LLM-as-Judge Extension Point

Depends on: M3. **Stubbed interfaces only** — actual LLM integration deferred.

### `internal/eval/llmjudge.go`

```go
type PairwiseJudgment struct {
    TaskIDA, TaskIDB, Winner, Reasoning, PositionA string
    Confidence float64
}

func PairwiseCompare(taskA, taskB export.TaskExport) *PairwiseJudgment  // stub
```

Pairwise comparison design with position swapping for bias mitigation. The interface is ready for Claude API / OpenAI / local LLM integration.

---

## Dependency Graph

```
M1 (Core types + scorers) → M2 (CLI + comparative) → M3 (Persistence + aggregation) → M4 (LLM-as-judge stub)
```

Linear chain. M1 is the critical path — once scoring works, everything else builds on it.

---

## Files Summary

| File | Action | Milestone |
|------|--------|-----------|
| `internal/eval/types.go` | CREATE | M1 |
| `internal/eval/scorer.go` | CREATE | M1 |
| `internal/eval/loader.go` | CREATE | M1 |
| `internal/eval/ambiguity.go` | CREATE | M1 |
| `internal/eval/completeness.go` | CREATE | M1 |
| `internal/eval/fileaccuracy.go` | CREATE | M1 |
| `internal/eval/clarifications.go` | CREATE | M1 |
| `internal/eval/report.go` | CREATE | M1 |
| `internal/eval/scorer_test.go` | CREATE | M1 |
| `internal/orchestrator/verification.go` | MODIFY (export 3 helpers) | M1 |
| `cmd/decompose/eval.go` | CREATE | M2 |
| `cmd/decompose/main.go` | MODIFY (add eval dispatch + usage) | M2 |
| `internal/eval/compare.go` | CREATE | M2 |
| `internal/eval/compare_test.go` | CREATE | M2 |
| `internal/eval/store.go` | CREATE | M3 |
| `internal/eval/aggregate.go` | CREATE | M3 |
| `internal/eval/skeleton_ablation.go` | CREATE | M3 |
| `internal/eval/llmjudge.go` | CREATE | M4 |

## Key Reuse Points

- `export.TaskExport` (`internal/export/json.go`) — already parses task IDs, file actions, deps, acceptance criteria from Stage 4 markdown. All scorers operate on `[]TaskExport`.
- `export.ExportDecomposition` — loads and parses all task files from filesystem.
- `status.GetDecompositionStatus` — locates stage output files, checks completion.
- `verification.go` helpers — `extractEntityNames`, `extractFeatureNames`, `extractTreeFiles` for cross-stage tracing.
- `cmd/decompose/export.go` — pattern for the eval CLI handler (load → process → format → stdout).

## Design Decisions

1. **Scorer interface over VerificationRule extension**: The verification system's `VerificationRule` returns a single `*VerificationFinding` (pass/fail). Evaluation needs numeric scores per task, aggregate percentages, and detailed reason strings. A dedicated `Scorer` interface is cleaner than overloading findings with scores.

2. **Reuse export.TaskExport as scorer input**: The export system already does the hard work of parsing task IDs, file actions, dependencies, and acceptance criteria from Stage 4 markdown. Scorers operate on `[]export.TaskExport` rather than re-parsing markdown.

3. **Export verification helpers**: `extractEntityNames`, `extractFeatureNames`, and `extractTreeFiles` are useful beyond verification. Exporting them avoids duplication between the eval and verification packages.

4. **Convention-based baseline storage**: Baselines use the same directory structure as pd output (`docs/decompose/<name>-baseline/`). The entire scoring pipeline works identically on both pd and baseline outputs without special-casing.

5. **Clarification count as derived metric**: Rather than a separate measurement system, clarification count is computed from the ambiguity scorer's task-level scores. Tasks scoring 0 (blocked) need at least 1 clarification. This makes the primary metric consistent with the rubric-based scoring.

6. **LLM-as-judge as extension point**: The pairwise comparison design (with position swapping for bias mitigation) is fully specified as an interface, but implementation is deferred. Rule-based metrics provide the v1 foundation; LLM scoring is an additive enhancement.

## Verification Plan

1. **Unit tests**: `go test ./internal/eval/...` — all scorers tested with known-good and known-bad task specs
2. **Single eval**: `decompose eval <name>` on an existing decomposition → prints markdown with metric scores
3. **Compare**: Create two decompositions of same idea (pd + manual baseline), run `decompose eval compare` → see delta analysis
4. **Skeleton ablation**: Run pipeline twice on same idea (with and without Stage 2), run `decompose eval skeleton` → confirm Stage 2 hypothesis shows measurable delta
5. **JSON output**: `decompose eval <name> --format json` → valid JSON, parseable by external tools
6. **Persistence**: `decompose eval <name> --save` → writes `eval-default.json`, subsequent `decompose eval aggregate` picks it up
