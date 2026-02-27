package mcptools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dusk-indust/decompose/internal/orchestrator"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOrchestrator is a test double for orchestrator.Orchestrator.
type mockOrchestrator struct {
	runStageResult *orchestrator.StageResult
	runStageErr    error
	progressCh     chan orchestrator.ProgressEvent
}

func newMockOrchestrator() *mockOrchestrator {
	ch := make(chan orchestrator.ProgressEvent)
	close(ch) // immediately closed since we don't need progress
	return &mockOrchestrator{progressCh: ch}
}

func (m *mockOrchestrator) RunStage(_ context.Context, stage orchestrator.Stage) (*orchestrator.StageResult, error) {
	if m.runStageErr != nil {
		return nil, m.runStageErr
	}
	if m.runStageResult != nil {
		return m.runStageResult, nil
	}
	return &orchestrator.StageResult{
		Stage:     stage,
		FilePaths: []string{"/tmp/test-output.md"},
	}, nil
}

func (m *mockOrchestrator) RunPipeline(_ context.Context, from, to orchestrator.Stage) ([]orchestrator.StageResult, error) {
	return nil, nil
}

func (m *mockOrchestrator) Progress() <-chan orchestrator.ProgressEvent {
	return m.progressCh
}

func TestDecomposeService_RunStage(t *testing.T) {
	mock := newMockOrchestrator()
	mock.runStageResult = &orchestrator.StageResult{
		Stage:     orchestrator.StageDevelopmentStandards,
		FilePaths: []string{"/out/stage-0-development-standards.md"},
	}

	cfg := orchestrator.Config{
		Name:        "test",
		ProjectRoot: t.TempDir(),
	}

	svc := NewDecomposeService(mock, cfg)
	ctx := context.Background()

	_, out, err := svc.RunStage(ctx, nil, RunStageInput{Name: "test", Stage: 0})
	require.NoError(t, err)
	assert.Equal(t, "completed", out.Status)
	assert.Equal(t, 0, out.Stage)
	assert.Equal(t, []string{"/out/stage-0-development-standards.md"}, out.FilesWritten)
}

func TestDecomposeService_RunStage_InvalidStage(t *testing.T) {
	mock := newMockOrchestrator()
	cfg := orchestrator.Config{Name: "test", ProjectRoot: t.TempDir()}
	svc := NewDecomposeService(mock, cfg)

	_, _, err := svc.RunStage(context.Background(), nil, RunStageInput{Name: "test", Stage: 5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid stage")
}

func TestDecomposeService_GetStatus_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := orchestrator.Config{Name: "test", ProjectRoot: tmpDir}
	mock := newMockOrchestrator()
	svc := NewDecomposeService(mock, cfg)

	_, out, err := svc.GetStatus(context.Background(), nil, GetStatusInput{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, "test", out.Name)
	assert.Empty(t, out.CompletedStages)
	assert.Equal(t, 0, out.NextStage)
}

func TestDecomposeService_GetStatus_WithStages(t *testing.T) {
	tmpDir := t.TempDir()
	decompDir := filepath.Join(tmpDir, "docs", "decompose", "myproject")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))

	// Create stage 0 and stage 1 files.
	require.NoError(t, os.WriteFile(filepath.Join(decompDir, "stage-0-development-standards.md"), []byte("# stage 0"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(decompDir, "stage-1-design-pack.md"), []byte("# stage 1"), 0o644))

	cfg := orchestrator.Config{Name: "myproject", ProjectRoot: tmpDir}
	mock := newMockOrchestrator()
	svc := NewDecomposeService(mock, cfg)

	_, out, err := svc.GetStatus(context.Background(), nil, GetStatusInput{Name: "myproject"})
	require.NoError(t, err)
	assert.Equal(t, "myproject", out.Name)
	assert.Equal(t, []int{0, 1}, out.CompletedStages)
	assert.Equal(t, 2, out.NextStage)
}

func TestDecomposeService_ListDecompositions_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := orchestrator.Config{ProjectRoot: tmpDir}
	mock := newMockOrchestrator()
	svc := NewDecomposeService(mock, cfg)

	_, out, err := svc.ListDecompositions(context.Background(), nil, ListDecompositionsInput{})
	require.NoError(t, err)
	assert.Empty(t, out.Decompositions)
	assert.False(t, out.HasStage0)
}

func TestDecomposeService_ListDecompositions_WithDecomp(t *testing.T) {
	tmpDir := t.TempDir()
	decompDir := filepath.Join(tmpDir, "docs", "decompose")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))

	// Create a decomposition directory with stage files.
	projDir := filepath.Join(decompDir, "my-project")
	require.NoError(t, os.MkdirAll(projDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "stage-0-development-standards.md"), []byte("s0"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "stage-1-design-pack.md"), []byte("s1"), 0o644))

	// Create a shared stage-0 file at the top level.
	require.NoError(t, os.WriteFile(filepath.Join(decompDir, "stage-0-development-standards.md"), []byte("shared"), 0o644))

	cfg := orchestrator.Config{ProjectRoot: tmpDir}
	mock := newMockOrchestrator()
	svc := NewDecomposeService(mock, cfg)

	_, out, err := svc.ListDecompositions(context.Background(), nil, ListDecompositionsInput{})
	require.NoError(t, err)
	assert.True(t, out.HasStage0)
	require.Len(t, out.Decompositions, 1)
	assert.Equal(t, "my-project", out.Decompositions[0].Name)
	assert.Equal(t, []int{0, 1}, out.Decompositions[0].CompletedStages)
	assert.Equal(t, 2, out.Decompositions[0].NextStage)
}

func TestDecomposeMCPServer_ToolsList(t *testing.T) {
	mock := newMockOrchestrator()
	cfg := orchestrator.Config{Name: "test", ProjectRoot: t.TempDir()}

	server := NewDecomposeMCPServer(mock, cfg)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Run(ctx, serverTransport)

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "dev"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)

	tools, err := session.ListTools(ctx, nil)
	require.NoError(t, err)

	toolNames := make([]string, len(tools.Tools))
	for i, tool := range tools.Tools {
		toolNames[i] = tool.Name
	}

	assert.Contains(t, toolNames, "run_stage")
	assert.Contains(t, toolNames, "get_status")
	assert.Contains(t, toolNames, "list_decompositions")
	assert.Len(t, tools.Tools, 3)
}
