package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StageExecutor executes a single pipeline stage given configuration and
// prerequisite stage results.
type StageExecutor interface {
	Execute(ctx context.Context, cfg Config, inputs []StageResult) (*StageResult, error)
}

// Router maps pipeline stages to their registered executors and handles
// prerequisite resolution.
type Router struct {
	cfg       Config
	executors map[Stage]StageExecutor
}

// NewRouter creates a Router with the given configuration and an empty
// executor registry.
func NewRouter(cfg Config) *Router {
	return &Router{
		cfg:       cfg,
		executors: make(map[Stage]StageExecutor),
	}
}

// RegisterExecutor associates an executor with a pipeline stage.
func (r *Router) RegisterExecutor(stage Stage, exec StageExecutor) {
	r.executors[stage] = exec
}

// Route resolves prerequisites for the given stage, reads their output files,
// and delegates to the registered StageExecutor.
func (r *Router) Route(ctx context.Context, stage Stage) (*StageResult, error) {
	exec, ok := r.executors[stage]
	if !ok {
		return nil, fmt.Errorf("router: no executor registered for stage %d (%s)", stage, stage)
	}

	inputs, err := r.resolvePrerequisites(stage)
	if err != nil {
		return nil, fmt.Errorf("router: prerequisite check failed for stage %d (%s): %w", stage, stage, err)
	}

	return exec.Execute(ctx, r.cfg, inputs)
}

// RouteRange executes stages sequentially from `from` to `to` (inclusive),
// feeding each stage's output forward as an additional input for subsequent
// stages.
func (r *Router) RouteRange(ctx context.Context, from, to Stage) ([]StageResult, error) {
	if from > to {
		return nil, fmt.Errorf("router: invalid range: from (%d) > to (%d)", from, to)
	}

	var results []StageResult

	for stage := from; stage <= to; stage++ {
		result, err := r.Route(ctx, stage)
		if err != nil {
			return results, fmt.Errorf("router: stage %d (%s) failed: %w", stage, stage, err)
		}
		results = append(results, *result)
	}

	return results, nil
}

// prerequisiteRules defines which stages are required or optional before each
// stage can execute.
type prerequisiteRule struct {
	stage    Stage
	required bool // if false, the prerequisite is optional (warn on missing)
}

// prerequisites returns the prerequisite rules for the given stage.
func prerequisites(stage Stage) []prerequisiteRule {
	switch stage {
	case StageDevelopmentStandards:
		// Stage 0: no prerequisites.
		return nil
	case StageDesignPack:
		// Stage 1: Stage 0 recommended but not required.
		return []prerequisiteRule{
			{stage: StageDevelopmentStandards, required: false},
		}
	case StageImplementationSkeletons:
		// Stage 2: Stage 1 MUST exist.
		return []prerequisiteRule{
			{stage: StageDesignPack, required: true},
		}
	case StageTaskIndex:
		// Stage 3: Stage 1 and Stage 2 MUST exist.
		return []prerequisiteRule{
			{stage: StageDesignPack, required: true},
			{stage: StageImplementationSkeletons, required: true},
		}
	case StageTaskSpecifications:
		// Stage 4: Stage 3 MUST exist.
		return []prerequisiteRule{
			{stage: StageTaskIndex, required: true},
		}
	default:
		return nil
	}
}

// stageFileName returns the expected output filename for a stage.
func stageFileName(stage Stage) string {
	return fmt.Sprintf("stage-%d-%s.md", int(stage), stage.String())
}

// resolvePrerequisites reads the output files for all prior stages and returns
// them as StageResult values. Required prerequisites that are missing cause an
// error; optional ones are silently skipped. Reading all prior stages (not just
// declared prerequisites) ensures downstream executors can correctly infer the
// current stage index from the input set.
func (r *Router) resolvePrerequisites(stage Stage) ([]StageResult, error) {
	if stage == StageDevelopmentStandards {
		return nil, nil
	}

	// Build a set of required stages for fast lookup.
	rules := prerequisites(stage)
	required := make(map[Stage]bool, len(rules))
	for _, rule := range rules {
		if rule.required {
			required[rule.stage] = true
		}
	}

	var inputs []StageResult

	for s := StageDevelopmentStandards; s < stage; s++ {
		result, err := r.readStageOutput(s)
		if err != nil {
			if required[s] {
				return nil, fmt.Errorf("required prerequisite stage %d (%s) not satisfied: %w",
					s, s, err)
			}
			// Non-required prior stage: skip silently.
			continue
		}
		inputs = append(inputs, *result)
	}

	return inputs, nil
}

// readStageOutput reads the output file(s) for a completed stage and returns a
// StageResult. For stages 0–3 a single file is expected; for stage 4 the
// output is a set of task specification files matching "tasks_m*.md".
func (r *Router) readStageOutput(stage Stage) (*StageResult, error) {
	if stage == StageTaskSpecifications {
		return r.readTaskSpecFiles()
	}

	filename := stageFileName(stage)
	p := filepath.Join(r.cfg.OutputDir, filename)

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("reading stage output %s: %w", p, err)
	}

	return &StageResult{
		Stage:     stage,
		FilePaths: []string{p},
		Sections: []Section{
			{
				Name:    stage.String(),
				Content: string(data),
			},
		},
	}, nil
}

// readTaskSpecFiles reads all task specification files matching
// "tasks_m*.md" in the output directory.
func (r *Router) readTaskSpecFiles() (*StageResult, error) {
	pattern := filepath.Join(r.cfg.OutputDir, "tasks_m*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing task spec files %s: %w", pattern, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no task specification files found matching %s", pattern)
	}

	result := &StageResult{
		Stage:     StageTaskSpecifications,
		FilePaths: matches,
	}

	for _, p := range matches {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading task spec file %s: %w", p, err)
		}

		// Derive section name from filename without extension and directory.
		name := strings.TrimSuffix(filepath.Base(p), ".md")

		result.Sections = append(result.Sections, Section{
			Name:    name,
			Content: string(data),
		})
	}

	return result, nil
}
