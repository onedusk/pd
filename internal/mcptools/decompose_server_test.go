package mcptools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/onedusk/pd/internal/graph"
	"github.com/onedusk/pd/internal/orchestrator"
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

// TestUnifiedMCPServer_ToolsList checks the decompose tools the unified server
// registers when no code intelligence service is attached. run_stage must not
// be among them: it ignored its name input and wrote into the working
// directory of the server process.
func TestUnifiedMCPServer_ToolsList(t *testing.T) {
	mock := newMockOrchestrator()
	cfg := orchestrator.Config{ProjectRoot: t.TempDir()}

	server := NewUnifiedMCPServer(mock, cfg, nil)

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

	assert.NotContains(t, toolNames, "run_stage")
	assert.ElementsMatch(t, []string{
		"get_status",
		"list_decompositions",
		"write_stage",
		"get_stage_context",
		"set_input",
		"run_review",
	}, toolNames)
}

// writeReviewFixture creates a project whose plan modifies main.go and names
// the symbol HandleRequest, which main.go defines.
func writeReviewFixture(t *testing.T) (root, decompDir string) {
	t.Helper()
	root = t.TempDir()
	decompDir = filepath.Join(root, "docs", "decompose", "demo")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc HandleRequest() {}\n"), 0o644))

	files := map[string]string{
		"stage-3-task-index.md": "## Target Directory Tree\n\n```\nmain.go    MODIFY (M1)\n```\n",
		"tasks_m01.md":          "- [ ] **T-01.01 -- Change handler**\n  - **File:** `main.go` (MODIFY)\n  - **Depends on:** None\n  - **Outline:** Change `HandleRequest` to log.\n",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(decompDir, name), []byte(content), 0o644))
	}
	return root, decompDir
}

// TestDecomposeService_RunReview_EmptyGraph covers run_review before
// build_graph: the attached in-memory store has no files, so it must not be
// treated as an index of the codebase.
func TestDecomposeService_RunReview_EmptyGraph(t *testing.T) {
	root, decompDir := writeReviewFixture(t)

	svc := NewDecomposeService(newMockOrchestrator(), orchestrator.Config{ProjectRoot: root})
	svc.SetCodeIntel(NewCodeIntelService(graph.NewMemStore(), nil))

	_, out, err := svc.RunReview(context.Background(), nil, RunReviewInput{Name: "demo"})
	require.NoError(t, err)
	require.Equal(t, "completed", out.Status)
	assert.Equal(t, 0, out.Mismatches)

	findings, err := os.ReadFile(filepath.Join(decompDir, "review-findings.md"))
	require.NoError(t, err)
	assert.Contains(t, string(findings), "**Graph indexed:** no")
	assert.NotContains(t, string(findings), "does not exist in codebase")
}

// TestDecomposeService_RunReview_EmptyGraphUsesDiskGraph checks that an empty
// in-memory store falls through to the graph persisted under .decompose/graph.
func TestDecomposeService_RunReview_EmptyGraphUsesDiskGraph(t *testing.T) {
	root, decompDir := writeReviewFixture(t)
	ctx := context.Background()

	require.NoError(t, os.MkdirAll(filepath.Join(root, ".decompose"), 0o755))
	disk, err := graph.NewKuzuFileStore(filepath.Join(root, ".decompose", "graph"))
	require.NoError(t, err)
	require.NoError(t, disk.InitSchema(ctx))
	require.NoError(t, disk.AddFile(ctx, graph.FileNode{Path: "main.go", Language: graph.LangGo, LOC: 3}))
	require.NoError(t, disk.AddSymbol(ctx, graph.SymbolNode{
		Name: "HandleRequest", Kind: graph.SymbolKindFunction, Exported: true,
		FilePath: "main.go", StartLine: 3, EndLine: 3,
	}))
	require.NoError(t, disk.Close())

	svc := NewDecomposeService(newMockOrchestrator(), orchestrator.Config{ProjectRoot: root})
	svc.SetCodeIntel(NewCodeIntelService(graph.NewMemStore(), nil))

	_, out, err := svc.RunReview(ctx, nil, RunReviewInput{Name: "demo"})
	require.NoError(t, err)
	require.Equal(t, "completed", out.Status)
	assert.Equal(t, 0, out.Mismatches)

	findings, err := os.ReadFile(filepath.Join(decompDir, "review-findings.md"))
	require.NoError(t, err)
	assert.Contains(t, string(findings), "**Graph indexed:** yes")
}

// TestDecomposeService_RunReview_IndexedGraph checks that an in-memory store
// with files in it is still the one run_review uses.
func TestDecomposeService_RunReview_IndexedGraph(t *testing.T) {
	root, decompDir := writeReviewFixture(t)
	ctx := context.Background()

	store := graph.NewMemStore()
	require.NoError(t, store.AddFile(ctx, graph.FileNode{Path: "main.go", Language: graph.LangGo, LOC: 3}))
	require.NoError(t, store.AddSymbol(ctx, graph.SymbolNode{
		Name: "HandleRequest", Kind: graph.SymbolKindFunction, Exported: true,
		FilePath: "main.go", StartLine: 3, EndLine: 3,
	}))

	svc := NewDecomposeService(newMockOrchestrator(), orchestrator.Config{ProjectRoot: root})
	svc.SetCodeIntel(NewCodeIntelService(store, nil))

	_, out, err := svc.RunReview(ctx, nil, RunReviewInput{Name: "demo"})
	require.NoError(t, err)
	require.Equal(t, "completed", out.Status)
	assert.Equal(t, 0, out.Mismatches)

	findings, err := os.ReadFile(filepath.Join(decompDir, "review-findings.md"))
	require.NoError(t, err)
	assert.Contains(t, string(findings), "**Graph indexed:** yes")
}

// TestDecomposeService_GetStageContext_GraphAvailable checks that
// graphAvailable reports whether the attached graph store holds any files,
// not a constant true.
func TestDecomposeService_GetStageContext_GraphAvailable(t *testing.T) {
	ctx := context.Background()

	indexed := graph.NewMemStore()
	require.NoError(t, indexed.AddFile(ctx, graph.FileNode{Path: "main.go", Language: graph.LangGo, LOC: 3}))

	tests := []struct {
		name      string
		codeintel *CodeIntelService
		want      bool
	}{
		{"no code intelligence service", nil, false},
		{"empty graph before build_graph", NewCodeIntelService(graph.NewMemStore(), nil), false},
		{"graph with files", NewCodeIntelService(indexed, nil), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewDecomposeService(newMockOrchestrator(), orchestrator.Config{ProjectRoot: t.TempDir()})
			if tt.codeintel != nil {
				svc.SetCodeIntel(tt.codeintel)
			}

			_, out, err := svc.GetStageContext(ctx, nil, GetStageContextInput{Name: "demo", Stage: 1})
			require.NoError(t, err)
			assert.Equal(t, tt.want, out.GraphAvailable)
		})
	}
}

// TestDecomposeService_RunReview_EmptyDiskGraph checks that a saved graph
// with no files (a build_graph run that matched nothing) is not used as an
// index.
func TestDecomposeService_RunReview_EmptyDiskGraph(t *testing.T) {
	root, decompDir := writeReviewFixture(t)
	ctx := context.Background()

	require.NoError(t, os.MkdirAll(filepath.Join(root, ".decompose"), 0o755))
	disk, err := graph.NewKuzuFileStore(filepath.Join(root, ".decompose", "graph"))
	require.NoError(t, err)
	require.NoError(t, disk.InitSchema(ctx))
	require.NoError(t, disk.Close())

	svc := NewDecomposeService(newMockOrchestrator(), orchestrator.Config{ProjectRoot: root})
	svc.SetCodeIntel(NewCodeIntelService(graph.NewMemStore(), nil))

	_, out, err := svc.RunReview(ctx, nil, RunReviewInput{Name: "demo"})
	require.NoError(t, err)
	require.Equal(t, "completed", out.Status)
	assert.Equal(t, 0, out.Mismatches)

	findings, err := os.ReadFile(filepath.Join(decompDir, "review-findings.md"))
	require.NoError(t, err)
	assert.Contains(t, string(findings), "**Graph indexed:** no")
}
