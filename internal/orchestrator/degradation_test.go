package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dusk-indust/decompose/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockA2AClient is a minimal mock for a2a.Client used in degradation tests.
type mockA2AClient struct{}

func (m *mockA2AClient) SendMessage(_ context.Context, _ string, _ a2a.SendMessageRequest) (*a2a.Task, error) {
	return nil, a2a.ErrNotImplemented
}

func (m *mockA2AClient) GetTask(_ context.Context, _ string, _ a2a.GetTaskRequest) (*a2a.Task, error) {
	return nil, a2a.ErrNotImplemented
}

func (m *mockA2AClient) ListTasks(_ context.Context, _ string, _ a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	return nil, a2a.ErrNotImplemented
}

func (m *mockA2AClient) CancelTask(_ context.Context, _ string, _ a2a.CancelTaskRequest) (*a2a.Task, error) {
	return nil, a2a.ErrNotImplemented
}

func (m *mockA2AClient) SubscribeToTask(_ context.Context, _ string, _ string) (<-chan a2a.StreamEvent, error) {
	return nil, a2a.ErrNotImplemented
}

func (m *mockA2AClient) DiscoverAgent(_ context.Context, _ string) (*a2a.AgentCard, error) {
	return nil, a2a.ErrNotImplemented
}

func TestDegradation_CapBasic_Stage0(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:       "deg-test",
		OutputDir:  tmpDir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, &mockA2AClient{})
	defer pipeline.Close()

	ctx := context.Background()
	// Drain progress.
	go func() {
		for range pipeline.Progress() {
		}
	}()

	result, err := pipeline.RunStage(ctx, StageDevelopmentStandards)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StageDevelopmentStandards, result.Stage)
	require.Len(t, result.FilePaths, 1)

	data, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(data)

	assert.Contains(t, text, "Stage 0")
	assert.Contains(t, text, "TODO")
}

func TestDegradation_CapMCPOnly_Stage0(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:       "deg-test",
		OutputDir:  tmpDir,
		Capability: CapMCPOnly,
	}

	pipeline := NewPipeline(cfg, &mockA2AClient{})
	defer pipeline.Close()

	ctx := context.Background()
	go func() {
		for range pipeline.Progress() {
		}
	}()

	result, err := pipeline.RunStage(ctx, StageDevelopmentStandards)
	require.NoError(t, err)
	require.NotNil(t, result)

	data, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(data)

	assert.Contains(t, text, "MCP-only mode")
	assert.Contains(t, text, "MCP tools")
}

func TestDegradation_CapBasic_Stage1(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:       "deg-test",
		OutputDir:  tmpDir,
		Capability: CapBasic,
	}

	// Create stage 0 file as prerequisite.
	stage0Path := filepath.Join(tmpDir, "stage-0-development-standards.md")
	require.NoError(t, os.WriteFile(stage0Path, []byte("# Stage 0\nDev standards content"), 0o644))

	pipeline := NewPipeline(cfg, &mockA2AClient{})
	defer pipeline.Close()

	ctx := context.Background()
	go func() {
		for range pipeline.Progress() {
		}
	}()

	result, err := pipeline.RunStage(ctx, StageDesignPack)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StageDesignPack, result.Stage)

	data, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(data)

	// Verify all 13 design pack section headers are present.
	expectedSections := Stage1MergePlan.SectionOrder
	for _, sec := range expectedSections {
		assert.True(t, strings.Contains(text, "## "+sec), "missing section: %s", sec)
	}
}

func TestDegradation_SingleAgentOverride(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:        "deg-test",
		OutputDir:   tmpDir,
		Capability:  CapA2AMCP,
		SingleAgent: true,
	}

	pipeline := NewPipeline(cfg, &mockA2AClient{})
	defer pipeline.Close()

	ctx := context.Background()
	go func() {
		for range pipeline.Progress() {
		}
	}()

	// Should use basic path despite CapA2AMCP because SingleAgent=true.
	result, err := pipeline.RunStage(ctx, StageDevelopmentStandards)
	require.NoError(t, err)
	require.NotNil(t, result)

	data, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(data)

	assert.Contains(t, text, "TODO")
	assert.Contains(t, text, "basic mode")
}
