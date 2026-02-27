package mcptools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dusk-indust/decompose/internal/orchestrator"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DecomposeService handles MCP tool calls for the decompose server mode.
// It wraps an Orchestrator to execute pipeline stages and query status.
type DecomposeService struct {
	pipeline orchestrator.Orchestrator
	cfg      orchestrator.Config
}

// NewDecomposeService creates a DecomposeService with the given pipeline and config.
func NewDecomposeService(pipeline orchestrator.Orchestrator, cfg orchestrator.Config) *DecomposeService {
	return &DecomposeService{
		pipeline: pipeline,
		cfg:      cfg,
	}
}

// RunStage executes a single pipeline stage and returns the files written.
func (s *DecomposeService) RunStage(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input RunStageInput,
) (*mcp.CallToolResult, RunStageOutput, error) {
	if input.Stage < 0 || input.Stage > 4 {
		return nil, RunStageOutput{
			Stage:   input.Stage,
			Status:  "failed",
			Message: fmt.Sprintf("stage must be 0-4, got %d", input.Stage),
		}, fmt.Errorf("invalid stage number: %d", input.Stage)
	}

	stage := orchestrator.Stage(input.Stage)
	result, err := s.pipeline.RunStage(ctx, stage)
	if err != nil {
		return nil, RunStageOutput{
			Stage:   input.Stage,
			Status:  "failed",
			Message: err.Error(),
		}, nil
	}

	return nil, RunStageOutput{
		FilesWritten: result.FilePaths,
		Stage:        input.Stage,
		Status:       "completed",
	}, nil
}

// GetStatus reports which stages are complete for a named decomposition.
func (s *DecomposeService) GetStatus(
	_ context.Context,
	_ *mcp.CallToolRequest,
	input GetStatusInput,
) (*mcp.CallToolResult, GetStatusOutput, error) {
	name := input.Name
	if name == "" {
		name = s.cfg.Name
	}

	outputDir := filepath.Join(s.cfg.ProjectRoot, "docs", "decompose", name)
	completed := scanCompletedStages(outputDir)
	next := nextStage(completed)

	return nil, GetStatusOutput{
		Name:            name,
		CompletedStages: completed,
		NextStage:       next,
		CapabilityLevel: s.cfg.Capability.String(),
	}, nil
}

// ListDecompositions scans the docs/decompose directory for all decompositions.
func (s *DecomposeService) ListDecompositions(
	_ context.Context,
	_ *mcp.CallToolRequest,
	input ListDecompositionsInput,
) (*mcp.CallToolResult, ListDecompositionsOutput, error) {
	projectRoot := input.ProjectRoot
	if projectRoot == "" {
		projectRoot = s.cfg.ProjectRoot
	}

	decomposeDir := filepath.Join(projectRoot, "docs", "decompose")
	entries, err := os.ReadDir(decomposeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ListDecompositionsOutput{}, nil
		}
		return nil, ListDecompositionsOutput{}, fmt.Errorf("read decompose dir: %w", err)
	}

	var summaries []DecompositionSummary
	hasStage0 := false

	for _, entry := range entries {
		if !entry.IsDir() {
			// Check for shared stage-0 file at the top level.
			if strings.HasPrefix(entry.Name(), "stage-0-") {
				hasStage0 = true
			}
			continue
		}

		name := entry.Name()
		subDir := filepath.Join(decomposeDir, name)
		completed := scanCompletedStages(subDir)

		summaries = append(summaries, DecompositionSummary{
			Name:            name,
			CompletedStages: completed,
			NextStage:       nextStage(completed),
		})
	}

	return nil, ListDecompositionsOutput{
		Decompositions: summaries,
		HasStage0:      hasStage0,
	}, nil
}

// scanCompletedStages checks which stage output files exist in a directory.
func scanCompletedStages(dir string) []int {
	var completed []int
	for stage := 0; stage <= 4; stage++ {
		s := orchestrator.Stage(stage)
		filename := fmt.Sprintf("stage-%d-%s.md", stage, s.String())
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			completed = append(completed, stage)
		}
	}
	return completed
}

// nextStage returns the next stage to run based on completed stages.
func nextStage(completed []int) int {
	if len(completed) == 0 {
		return 0
	}
	max := completed[0]
	for _, s := range completed[1:] {
		if s > max {
			max = s
		}
	}
	next := max + 1
	if next > 4 {
		next = -1 // all stages complete
	}
	return next
}
