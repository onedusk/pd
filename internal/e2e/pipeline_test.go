//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/dusk-indust/decompose/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopA2AClient implements a2a.Client with all methods returning
// a2a.ErrNotImplemented. It is used in e2e tests that exercise the
// FallbackExecutor path (CapBasic), which never dispatches to agents.
type noopA2AClient struct{}

func (n *noopA2AClient) DiscoverAgent(ctx context.Context, baseURL string) (*a2a.AgentCard, error) {
	return nil, a2a.ErrNotImplemented
}

func (n *noopA2AClient) SendMessage(ctx context.Context, endpoint string, req a2a.SendMessageRequest) (*a2a.Task, error) {
	return nil, a2a.ErrNotImplemented
}

func (n *noopA2AClient) GetTask(ctx context.Context, endpoint string, req a2a.GetTaskRequest) (*a2a.Task, error) {
	return nil, a2a.ErrNotImplemented
}

func (n *noopA2AClient) ListTasks(ctx context.Context, endpoint string, req a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	return nil, a2a.ErrNotImplemented
}

func (n *noopA2AClient) CancelTask(ctx context.Context, endpoint string, req a2a.CancelTaskRequest) (*a2a.Task, error) {
	return nil, a2a.ErrNotImplemented
}

func (n *noopA2AClient) SubscribeToTask(ctx context.Context, endpoint string, taskID string) (<-chan a2a.StreamEvent, error) {
	return nil, a2a.ErrNotImplemented
}

// TestPipeline_E2E_CapBasic runs the full decomposition pipeline (stages 0-4)
// in CapBasic mode and verifies that each stage produces the expected output
// files with the correct section structure.
func TestPipeline_E2E_CapBasic(t *testing.T) {
	outputDir := t.TempDir()

	cfg := orchestrator.Config{
		Name:        "e2e-test",
		ProjectRoot: filepath.Join("..", "..", "testdata", "fixtures", "go_project"),
		OutputDir:   outputDir,
		Capability:  orchestrator.CapBasic,
	}

	pipeline := orchestrator.NewPipeline(cfg, &noopA2AClient{})

	// Drain progress events in the background so the pipeline does not block.
	progressCh := pipeline.Progress()
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range progressCh {
			// discard
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run all five stages.
	results, err := pipeline.RunPipeline(ctx, orchestrator.StageDevelopmentStandards, orchestrator.StageTaskSpecifications)
	require.NoError(t, err)
	require.Len(t, results, 5, "pipeline should return results for all 5 stages")

	// Close the pipeline and wait for the drain goroutine to finish.
	pipeline.Close()
	<-drainDone

	// --- Verify all stage output files exist and are non-empty ---

	stageFiles := []string{
		"stage-0-development-standards.md",
		"stage-1-design-pack.md",
		"stage-2-implementation-skeletons.md",
		"stage-3-task-index.md",
		"stage-4-task-specifications.md",
	}

	for _, name := range stageFiles {
		path := filepath.Join(outputDir, name)
		info, err := os.Stat(path)
		require.NoError(t, err, "stage output file %s should exist", name)
		assert.Greater(t, info.Size(), int64(0), "stage output file %s should not be empty", name)
	}

	// --- Verify Stage 1 section headers ---

	stage1Data, err := os.ReadFile(filepath.Join(outputDir, "stage-1-design-pack.md"))
	require.NoError(t, err)
	stage1Content := string(stage1Data)

	for _, section := range []string{"assumptions", "platform-baseline", "data-model", "architecture"} {
		assert.Contains(t, stage1Content, section,
			"stage 1 output should contain section %q", section)
	}

	// --- Verify Stage 2 section headers ---

	stage2Data, err := os.ReadFile(filepath.Join(outputDir, "stage-2-implementation-skeletons.md"))
	require.NoError(t, err)
	stage2Content := string(stage2Data)

	for _, section := range []string{"data-model-code", "interface-contracts", "documentation"} {
		assert.Contains(t, stage2Content, section,
			"stage 2 output should contain section %q", section)
	}

	// --- Verify Stage 3 section headers ---

	stage3Data, err := os.ReadFile(filepath.Join(outputDir, "stage-3-task-index.md"))
	require.NoError(t, err)
	stage3Content := string(stage3Data)

	for _, section := range []string{"progress", "dependencies", "directory-tree"} {
		assert.Contains(t, stage3Content, section,
			"stage 3 output should contain section %q", section)
	}
}

// TestPipeline_E2E_SingleStage runs only stage 0 and verifies that it produces
// its output file while no subsequent stage files are created.
func TestPipeline_E2E_SingleStage(t *testing.T) {
	outputDir := t.TempDir()

	cfg := orchestrator.Config{
		Name:        "e2e-test",
		ProjectRoot: filepath.Join("..", "..", "testdata", "fixtures", "go_project"),
		OutputDir:   outputDir,
		Capability:  orchestrator.CapBasic,
	}

	pipeline := orchestrator.NewPipeline(cfg, &noopA2AClient{})

	// Drain progress events in the background.
	progressCh := pipeline.Progress()
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range progressCh {
			// discard
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run only stage 0.
	result, err := pipeline.RunStage(ctx, orchestrator.StageDevelopmentStandards)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, orchestrator.StageDevelopmentStandards, result.Stage)

	// Close the pipeline and wait for the drain goroutine to finish.
	pipeline.Close()
	<-drainDone

	// Stage 0 output should exist.
	stage0Path := filepath.Join(outputDir, "stage-0-development-standards.md")
	info, err := os.Stat(stage0Path)
	require.NoError(t, err, "stage 0 output file should exist")
	assert.Greater(t, info.Size(), int64(0), "stage 0 output file should not be empty")

	// Stage 1 output should NOT exist.
	stage1Path := filepath.Join(outputDir, "stage-1-design-pack.md")
	_, err = os.Stat(stage1Path)
	assert.True(t, os.IsNotExist(err), "stage 1 output file should not exist after running only stage 0")
}
