package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onedusk/pd/internal/a2a"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time check: Pipeline must satisfy the Orchestrator interface.
var _ Orchestrator = (*Pipeline)(nil)

// stubClient returns a mockClient whose SendMessage is never expected to be
// called (basic mode does not fan out to agents). If it is called, the test
// fails immediately.
func stubClient(t *testing.T) *mockClient {
	t.Helper()
	return &mockClient{
		sendMessage: func(_ context.Context, _ string, _ a2a.SendMessageRequest) (*a2a.Task, error) {
			t.Fatal("SendMessage should not be called in basic mode")
			return nil, nil
		},
	}
}

// TestPipeline_InterfaceCompliance is a compile-time-only test that verifies
// Pipeline satisfies both Orchestrator and StageExecutor. The var declarations
// above and in pipeline.go enforce this; this test exists so the intent is
// explicit in the test suite.
func TestPipeline_InterfaceCompliance(t *testing.T) {
	// If this compiles, the interfaces are satisfied.
	var _ Orchestrator = (*Pipeline)(nil)
	var _ StageExecutor = (*Pipeline)(nil)
}

// TestPipeline_BasicMode_RunStage executes a single stage in basic mode and
// verifies that an output file is written to the correct directory.
func TestPipeline_BasicMode_RunStage(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Name:       "test-project",
		OutputDir:  dir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	defer pipeline.Close()

	result, err := pipeline.RunStage(context.Background(), StageDevelopmentStandards)
	require.NoError(t, err)
	require.NotNil(t, result)

	// The result must reference the correct stage.
	assert.Equal(t, StageDevelopmentStandards, result.Stage)

	// At least one output file path should be reported.
	require.NotEmpty(t, result.FilePaths)

	// The output file must exist on disk.
	info, err := os.Stat(result.FilePaths[0])
	require.NoError(t, err, "output file should exist")
	assert.Greater(t, info.Size(), int64(0), "output file should not be empty")

	// The file should live under the configured output directory.
	rel, err := filepath.Rel(dir, result.FilePaths[0])
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(rel), "output should be inside the temp dir")

	// Sections should be populated.
	require.NotEmpty(t, result.Sections)
	assert.Equal(t, "template", result.Sections[0].Agent)
}

// TestPipeline_BasicMode_RunPipeline runs stages 0-1 sequentially in basic
// mode and verifies that each stage produces an output file. We limit to two
// stages because the Router resolves prerequisites by reading disk files, and
// stages 0-1 have the simplest prerequisite chain (Stage 0 has none, Stage 1
// optionally depends on Stage 0).
func TestPipeline_BasicMode_RunPipeline(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Name:       "full-run",
		OutputDir:  dir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	defer pipeline.Close()

	results, err := pipeline.RunPipeline(
		context.Background(),
		StageDevelopmentStandards,
		StageDesignPack,
	)
	require.NoError(t, err)
	require.Len(t, results, 2, "should return results for stages 0 and 1")

	expectedStages := []Stage{
		StageDevelopmentStandards,
		StageDesignPack,
	}

	for i, want := range expectedStages {
		res := results[i]
		assert.Equal(t, want, res.Stage, "stage %d mismatch", i)
		require.NotEmpty(t, res.FilePaths, "stage %d should have file paths", i)

		// Each output file must exist on disk.
		_, err := os.Stat(res.FilePaths[0])
		assert.NoError(t, err, "output file for stage %d should exist", i)
	}

	// Verify that later stages can read the files produced by earlier ones.
	// Stage 1 output file should exist and be non-empty.
	data, err := os.ReadFile(results[1].FilePaths[0])
	require.NoError(t, err)
	assert.NotEmpty(t, data, "stage 1 output should contain content")
}

// TestPipeline_ProgressEvents subscribes to the progress channel, runs a
// stage, and verifies that relevant progress events are received.
func TestPipeline_ProgressEvents(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Name:       "progress-test",
		OutputDir:  dir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	defer pipeline.Close()

	progressCh := pipeline.Progress()
	require.NotNil(t, progressCh, "Progress() should return a non-nil channel")

	// Run a stage so that events are emitted.
	_, err := pipeline.RunStage(context.Background(), StageDevelopmentStandards)
	require.NoError(t, err)

	// Drain events with a short timeout. RunStage emits at least a "working"
	// and a "complete" event for the stage.
	var events []ProgressEvent
	timeout := time.After(2 * time.Second)
drain:
	for {
		select {
		case ev, ok := <-progressCh:
			if !ok {
				break drain
			}
			events = append(events, ev)
		case <-timeout:
			break drain
		}
	}

	require.NotEmpty(t, events, "should have received at least one progress event")

	// All events should reference the correct stage.
	for _, ev := range events {
		assert.Equal(t, StageDevelopmentStandards, ev.Stage)
	}

	// We expect at minimum a "working" and a "complete" status.
	statuses := make(map[ProgressStatus]bool)
	for _, ev := range events {
		statuses[ev.Status] = true
	}
	assert.True(t, statuses[ProgressWorking], "should have a 'working' event")
	assert.True(t, statuses[ProgressComplete], "should have a 'complete' event")
}

// TestPipeline_Close verifies that calling Close() closes the progress channel.
func TestPipeline_Close(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Name:       "close-test",
		OutputDir:  dir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	progressCh := pipeline.Progress()
	require.NotNil(t, progressCh)

	pipeline.Close()

	// After Close(), the channel should be closed. Reading from a closed
	// channel returns the zero value immediately with ok == false.
	select {
	case _, ok := <-progressCh:
		assert.False(t, ok, "progress channel should be closed after Close()")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for progress channel to close")
	}
}

// TestPipeline_RunStage_WritesRequestedStage runs Stage 2 in a directory that
// holds Stage 1 but no Stage 0 (Stage 0 is shared and lives one level up). The
// pipeline must write the Stage 2 file, not a Stage 0 file.
func TestPipeline_RunStage_WritesRequestedStage(t *testing.T) {
	dir := t.TempDir()
	writeStageFile(t, dir, StageDesignPack, "# Finished design pack")
	cfg := Config{
		Name:       "requested-stage",
		OutputDir:  dir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	defer pipeline.Close()

	result, err := pipeline.RunStage(context.Background(), StageImplementationSkeletons)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StageImplementationSkeletons, result.Stage)
	assert.Equal(t, []string{filepath.Join(dir, stageFileName(StageImplementationSkeletons))}, result.FilePaths)
	assert.FileExists(t, filepath.Join(dir, stageFileName(StageImplementationSkeletons)))
	assert.NoFileExists(t, filepath.Join(dir, stageFileName(StageDevelopmentStandards)))
}

// TestPipeline_RunPipeline_RefusesExistingStageFiles runs the full pipeline
// over a finished decomposition. It must fail before writing anything: the
// finished files keep their content and no new file appears.
func TestPipeline_RunPipeline_RefusesExistingStageFiles(t *testing.T) {
	dir := t.TempDir()
	finished := []Stage{StageDesignPack, StageImplementationSkeletons, StageTaskIndex}
	for _, s := range finished {
		writeStageFile(t, dir, s, "# Finished "+s.String())
	}
	cfg := Config{
		Name:       "finished",
		OutputDir:  dir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	defer pipeline.Close()

	_, err := pipeline.RunPipeline(context.Background(), StageDevelopmentStandards, StageTaskSpecifications)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Contains(t, err.Error(), "--force")

	for _, s := range finished {
		data, readErr := os.ReadFile(filepath.Join(dir, stageFileName(s)))
		require.NoError(t, readErr)
		assert.Equal(t, "# Finished "+s.String(), string(data))
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, len(finished), "no new files should be written")
}

// TestPipeline_RunStage_RefusesExistingStageFile checks the single-stage path:
// an existing stage file is left alone and the error names --force.
func TestPipeline_RunStage_RefusesExistingStageFile(t *testing.T) {
	dir := t.TempDir()
	writeStageFile(t, dir, StageDesignPack, "# Finished design pack")
	cfg := Config{
		Name:       "finished",
		OutputDir:  dir,
		Capability: CapBasic,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	defer pipeline.Close()

	_, err := pipeline.RunStage(context.Background(), StageDesignPack)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Contains(t, err.Error(), "--force")

	data, err := os.ReadFile(filepath.Join(dir, stageFileName(StageDesignPack)))
	require.NoError(t, err)
	assert.Equal(t, "# Finished design pack", string(data))
}

// TestPipeline_FullMode_RefusesExistingStageFileBeforeFanOut checks that the
// existing-file check runs before any agent is contacted.
func TestPipeline_FullMode_RefusesExistingStageFileBeforeFanOut(t *testing.T) {
	dir := t.TempDir()
	writeStageFile(t, dir, StageDevelopmentStandards, "# Finished standards")
	cfg := Config{
		Name:           "finished",
		OutputDir:      dir,
		Capability:     CapA2AMCP,
		AgentEndpoints: []string{"http://127.0.0.1:0"},
	}

	var calls atomic.Int32
	client := &mockClient{
		sendMessage: func(_ context.Context, _ string, _ a2a.SendMessageRequest) (*a2a.Task, error) {
			calls.Add(1)
			return completedTask("t1", "development-standards"), nil
		},
	}

	pipeline := NewPipeline(cfg, client)
	defer pipeline.Close()

	_, err := pipeline.RunStage(context.Background(), StageDevelopmentStandards)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Equal(t, int32(0), calls.Load(), "no agent should be contacted")

	data, err := os.ReadFile(filepath.Join(dir, stageFileName(StageDevelopmentStandards)))
	require.NoError(t, err)
	assert.Equal(t, "# Finished standards", string(data))
}

// TestPipeline_RunPipeline_ForceOverwritesExistingStageFiles checks that Force
// lifts the existing-file refusal for a multi-stage run.
func TestPipeline_RunPipeline_ForceOverwritesExistingStageFiles(t *testing.T) {
	dir := t.TempDir()
	writeStageFile(t, dir, StageDesignPack, "# Finished design pack")
	cfg := Config{
		Name:       "forced",
		OutputDir:  dir,
		Capability: CapBasic,
		Force:      true,
	}

	pipeline := NewPipeline(cfg, stubClient(t))
	defer pipeline.Close()

	results, err := pipeline.RunPipeline(context.Background(), StageDevelopmentStandards, StageDesignPack)
	require.NoError(t, err)
	require.Len(t, results, 2)

	data, err := os.ReadFile(filepath.Join(dir, stageFileName(StageDesignPack)))
	require.NoError(t, err)
	assert.Contains(t, string(data), "basic mode")
}
