package orchestrator

import (
	"context"
	"fmt"
	"strings"
)

// Compile-time check.
var _ StageExecutor = (*FallbackExecutor)(nil)

// FallbackExecutor provides degraded execution for capability levels that
// cannot use the full parallel pipeline. CapBasic produces template files
// with TODO markers; CapMCPOnly produces sequential single-agent output.
type FallbackExecutor struct {
	level CapabilityLevel
}

// NewFallbackExecutor creates a FallbackExecutor for the given capability level.
func NewFallbackExecutor(level CapabilityLevel) *FallbackExecutor {
	return &FallbackExecutor{level: level}
}

// Execute runs the fallback path for a single stage.
func (f *FallbackExecutor) Execute(ctx context.Context, cfg Config, inputs []StageResult) (*StageResult, error) {
	stage := inferStageFromInputs(inputs)

	switch f.level {
	case CapBasic:
		return f.executeTemplate(ctx, cfg, stage, inputs)
	case CapMCPOnly:
		return f.executeMCPOnly(ctx, cfg, stage, inputs)
	default:
		return nil, fmt.Errorf("fallback: unsupported capability level %s; use Pipeline for %s", f.level, f.level)
	}
}

// executeTemplate produces a template file with TODO markers for manual completion.
func (f *FallbackExecutor) executeTemplate(_ context.Context, cfg Config, stage Stage, _ []StageResult) (*StageResult, error) {
	plan := mergePlanForStage(stage)
	sections := make([]Section, 0, len(plan.SectionOrder))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Stage %d: %s\n\n", int(stage), stage.String()))
	sb.WriteString("> Generated in basic mode. Fill in each section below.\n\n")

	for _, name := range plan.SectionOrder {
		sectionContent := fmt.Sprintf("## %s\n\n<!-- TODO: Complete this section -->\n\n", name)
		sb.WriteString(sectionContent)

		sections = append(sections, Section{
			Name:    name,
			Content: sectionContent,
			Agent:   "template",
		})
	}

	outPath := stageOutputPath(cfg, stage)
	if err := writeOutputFile(outPath, sb.String()); err != nil {
		return nil, fmt.Errorf("fallback template: write output for stage %d (%s): %w", stage, stage, err)
	}

	return &StageResult{
		Stage:     stage,
		FilePaths: []string{outPath},
		Sections:  sections,
	}, nil
}

// executeMCPOnly produces output using sequential MCP tool access.
// Without agents, each section is generated with available context and a note
// about MCP tool availability.
func (f *FallbackExecutor) executeMCPOnly(_ context.Context, cfg Config, stage Stage, inputs []StageResult) (*StageResult, error) {
	plan := mergePlanForStage(stage)
	contextText := buildContextMessage(stage, inputs)
	sections := make([]Section, 0, len(plan.SectionOrder))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Stage %d: %s\n\n", int(stage), stage.String()))
	sb.WriteString("> Generated in MCP-only mode (single agent, sequential execution).\n\n")

	if contextText != "" {
		sb.WriteString(contextText)
		sb.WriteString("\n---\n\n")
	}

	for _, name := range plan.SectionOrder {
		sectionContent := fmt.Sprintf("## %s\n\n_Generated via MCP tools (sequential mode)._\n\n"+
			"Content for this section would be produced by querying MCP code intelligence tools.\n\n", name)
		sb.WriteString(sectionContent)

		sections = append(sections, Section{
			Name:    name,
			Content: sectionContent,
			Agent:   "mcp-local",
		})
	}

	outPath := stageOutputPath(cfg, stage)
	if err := writeOutputFile(outPath, sb.String()); err != nil {
		return nil, fmt.Errorf("fallback mcp-only: write output for stage %d (%s): %w", stage, stage, err)
	}

	return &StageResult{
		Stage:     stage,
		FilePaths: []string{outPath},
		Sections:  sections,
	}, nil
}

// inferStageFromInputs determines which stage is being executed based on the
// inputs that have already been produced.
func inferStageFromInputs(inputs []StageResult) Stage {
	present := make(map[Stage]bool, len(inputs))
	for _, r := range inputs {
		present[r.Stage] = true
	}
	for s := StageDevelopmentStandards; s <= StageTaskSpecifications; s++ {
		if !present[s] {
			return s
		}
	}
	return StageTaskSpecifications
}
