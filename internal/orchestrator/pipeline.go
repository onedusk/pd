package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// Compile-time interface checks.
var (
	_ Orchestrator  = (*Pipeline)(nil)
	_ StageExecutor = (*Pipeline)(nil)
)

// Pipeline implements both Orchestrator and StageExecutor. It coordinates the
// full decomposition pipeline by delegating stage routing to a Router,
// parallel agent dispatch to a FanOut, and progress reporting to a
// ProgressReporter.
type Pipeline struct {
	cfg      Config
	client   a2a.Client
	router   *Router
	progress *ProgressReporter
	fanout   *FanOut
}

// NewPipeline creates a Pipeline wired with a Router, ProgressReporter, and
// FanOut. The pipeline registers itself as the StageExecutor for all five
// stages.
func NewPipeline(cfg Config, client a2a.Client) *Pipeline {
	progress := NewProgressReporter()
	fanout := NewFanOut(client, progress.Emit)
	router := NewRouter(cfg)

	p := &Pipeline{
		cfg:      cfg,
		client:   client,
		router:   router,
		progress: progress,
		fanout:   fanout,
	}

	// Register this pipeline as the executor for every stage.
	for stage := StageDevelopmentStandards; stage <= StageTaskSpecifications; stage++ {
		router.RegisterExecutor(stage, p)
	}

	return p
}

// ---------------------------------------------------------------------------
// Orchestrator interface
// ---------------------------------------------------------------------------

// RunStage executes a single pipeline stage. It emits a stage header via the
// progress reporter and delegates to the router, which calls back into
// Pipeline.Execute.
func (p *Pipeline) RunStage(ctx context.Context, stage Stage) (*StageResult, error) {
	p.progress.Emit(ProgressEvent{
		Stage:   stage,
		Section: FormatStageHeader(p.cfg.Name, stage),
		Status:  ProgressWorking,
	})

	result, err := p.router.Route(ctx, stage)
	if err != nil {
		p.progress.Emit(ProgressEvent{
			Stage:   stage,
			Section: stage.String(),
			Status:  ProgressFailed,
			Message: err.Error(),
		})
		return nil, err
	}

	p.progress.Emit(ProgressEvent{
		Stage:   stage,
		Section: stage.String(),
		Status:  ProgressComplete,
	})

	return result, nil
}

// RunPipeline executes stages from..to inclusive by delegating to the router.
func (p *Pipeline) RunPipeline(ctx context.Context, from, to Stage) ([]StageResult, error) {
	return p.router.RouteRange(ctx, from, to)
}

// Progress returns a channel that emits progress events.
func (p *Pipeline) Progress() <-chan ProgressEvent {
	return p.progress.Subscribe()
}

// Close shuts down the progress reporter. Callers should invoke this when the
// pipeline is no longer needed.
func (p *Pipeline) Close() {
	p.progress.Close()
}

// ---------------------------------------------------------------------------
// StageExecutor interface
// ---------------------------------------------------------------------------

// Execute is the StageExecutor callback invoked by the Router. It selects
// between fan-out (full/a2a) and fallback (basic/mcp-only) execution modes
// based on the configuration capability level.
func (p *Pipeline) Execute(ctx context.Context, cfg Config, inputs []StageResult) (*StageResult, error) {
	// Determine the current stage from the router's perspective.
	// The stage is derived from the number of completed inputs + prerequisite
	// depth, but in practice the Router passes the correct cfg and we infer
	// the stage from the input count. A more reliable approach: we inspect
	// what stage is NOT yet in the inputs.
	stage := p.inferStage(inputs)

	switch {
	case cfg.Capability >= CapA2AMCP && !cfg.SingleAgent:
		return p.executeFullMode(ctx, cfg, stage, inputs)
	default:
		return p.executeBasicMode(ctx, cfg, stage, inputs)
	}
}

// ---------------------------------------------------------------------------
// Full mode (fan-out with agents)
// ---------------------------------------------------------------------------

func (p *Pipeline) executeFullMode(ctx context.Context, cfg Config, stage Stage, inputs []StageResult) (*StageResult, error) {
	plan := mergePlanForStage(stage)

	// Build the context message from predecessor inputs.
	contextText := buildContextMessage(stage, inputs)

	// Assign sections to agents via round-robin.
	tasks := assignSectionsToAgents(plan, cfg.AgentEndpoints, stage, contextText)

	// Fan out to agents.
	agentResults, err := p.fanout.Run(ctx, stage, tasks)
	if err != nil {
		return nil, fmt.Errorf("pipeline: fan-out for stage %d (%s) failed: %w", stage, stage, err)
	}

	// Convert AgentResults to Sections.
	sections := agentResultsToSections(agentResults)

	// Merge sections according to the plan.
	merger := NewMerger(plan)
	merged, err := merger.Merge(sections)
	if err != nil {
		return nil, fmt.Errorf("pipeline: merge for stage %d (%s) failed: %w", stage, stage, err)
	}

	// Check coherence (log issues, do not block).
	issues, cohErr := CheckCoherence(sections)
	if cohErr != nil {
		log.Printf("WARNING: coherence check error for stage %d (%s): %v", stage, stage, cohErr)
	}
	for _, issue := range issues {
		log.Printf("WARNING: coherence issue in stage %d (%s): %s", stage, stage, issue.Description)
	}

	// Write output file.
	outPath := stageOutputPath(cfg, stage)
	if err := writeOutputFile(outPath, merged); err != nil {
		return nil, fmt.Errorf("pipeline: write output for stage %d (%s): %w", stage, stage, err)
	}

	return &StageResult{
		Stage:     stage,
		FilePaths: []string{outPath},
		Sections:  sections,
	}, nil
}

// ---------------------------------------------------------------------------
// Basic / fallback mode (single section, local execution)
// ---------------------------------------------------------------------------

func (p *Pipeline) executeBasicMode(ctx context.Context, cfg Config, stage Stage, inputs []StageResult) (*StageResult, error) {
	// In basic mode, produce a single section from the combined inputs.
	content := buildContextMessage(stage, inputs)
	if content == "" {
		content = fmt.Sprintf("# Stage %d: %s\n\n(placeholder - run with full capability for agent-generated content)\n",
			int(stage), stage.String())
	}

	section := Section{
		Name:    stage.String(),
		Content: content,
		Agent:   "local",
	}

	outPath := stageOutputPath(cfg, stage)
	if err := writeOutputFile(outPath, content); err != nil {
		return nil, fmt.Errorf("pipeline: write output for stage %d (%s): %w", stage, stage, err)
	}

	return &StageResult{
		Stage:     stage,
		FilePaths: []string{outPath},
		Sections:  []Section{section},
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// inferStage determines which stage is being executed based on the inputs
// that have already been produced. The stage is the smallest stage index
// not present in the inputs.
func (p *Pipeline) inferStage(inputs []StageResult) Stage {
	present := make(map[Stage]bool, len(inputs))
	for _, r := range inputs {
		present[r.Stage] = true
	}
	for s := StageDevelopmentStandards; s <= StageTaskSpecifications; s++ {
		if !present[s] {
			return s
		}
	}
	// All stages present: default to the last one.
	return StageTaskSpecifications
}

// mergePlanForStage returns the MergePlan for the given stage. Stages without
// a multi-section plan return a single-section plan using the stage name.
func mergePlanForStage(stage Stage) MergePlan {
	switch stage {
	case StageDesignPack:
		return Stage1MergePlan
	case StageImplementationSkeletons:
		return Stage2MergePlan
	case StageTaskIndex:
		return Stage3MergePlan
	default:
		// Stages 0 and 4 are single-section.
		return MergePlan{
			Strategy:     MergeConcatenate,
			SectionOrder: []string{stage.String()},
		}
	}
}

// stageOutputPath returns the output file path for a stage:
// <OutputDir>/stage-{N}-{name}.md
func stageOutputPath(cfg Config, stage Stage) string {
	return filepath.Join(cfg.OutputDir, fmt.Sprintf("stage-%d-%s.md", int(stage), stage.String()))
}

// assignSectionsToAgents creates AgentTasks by round-robin assignment of
// merge plan sections to the available agent endpoints.
func assignSectionsToAgents(plan MergePlan, endpoints []string, stage Stage, contextText string) []AgentTask {
	if len(endpoints) == 0 {
		return nil
	}

	tasks := make([]AgentTask, 0, len(plan.SectionOrder))
	for i, section := range plan.SectionOrder {
		endpoint := endpoints[i%len(endpoints)]

		prompt := fmt.Sprintf("Generate the %q section for stage %d (%s).\n\n%s",
			section, int(stage), stage.String(), contextText)

		tasks = append(tasks, AgentTask{
			AgentEndpoint: endpoint,
			Section:       section,
			Message: a2a.Message{
				Role:  a2a.RoleUser,
				Parts: []a2a.Part{a2a.TextPart(prompt)},
			},
		})
	}

	return tasks
}

// buildContextMessage constructs a prompt preamble from predecessor stage
// outputs so that downstream agents have full context.
func buildContextMessage(stage Stage, inputs []StageResult) string {
	if len(inputs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Context from prior stages\n\n")
	for _, input := range inputs {
		for _, sec := range input.Sections {
			fmt.Fprintf(&b, "### %s / %s\n\n%s\n\n", input.Stage.String(), sec.Name, sec.Content)
		}
	}
	return b.String()
}

// agentResultsToSections converts fan-out AgentResults into Sections by
// extracting the text content from each artifact.
func agentResultsToSections(results []AgentResult) []Section {
	sections := make([]Section, 0, len(results))
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		content := extractTextFromArtifacts(r.Artifacts)
		sections = append(sections, Section{
			Name:    r.Section,
			Content: content,
			Agent:   agentFromTask(r.Task),
		})
	}
	return sections
}

// extractTextFromArtifacts concatenates text parts from all artifacts.
func extractTextFromArtifacts(artifacts []a2a.Artifact) string {
	var parts []string
	for _, art := range artifacts {
		for _, p := range art.Parts {
			if p.Text != "" {
				parts = append(parts, p.Text)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// agentFromTask returns the agent name from a completed task, or "unknown".
func agentFromTask(t *a2a.Task) string {
	if t == nil {
		return "unknown"
	}
	// Use the task ID as a proxy for the agent identity.
	return t.ID
}

// writeOutputFile writes content to the given path, creating directories as
// needed.
func writeOutputFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
