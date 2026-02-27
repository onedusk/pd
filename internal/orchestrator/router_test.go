package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExecutor is a test double for StageExecutor that returns preconfigured
// results.
type mockExecutor struct {
	result *StageResult
	err    error
	// called tracks how many times Execute was invoked.
	called int
}

func (m *mockExecutor) Execute(ctx context.Context, cfg Config, inputs []StageResult) (*StageResult, error) {
	m.called++
	return m.result, m.err
}

// writeStageFile creates a stage output file in dir with conventional naming.
func writeStageFile(t *testing.T, dir string, stage Stage, content string) {
	t.Helper()
	filename := stageFileName(stage)
	err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
	require.NoError(t, err)
}

func TestRoute_Stage1_WithStage0Present(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "stage-0-development-standards.md"), []byte("# Standards"), 0644)
	cfg := Config{OutputDir: dir}

	router := NewRouter(cfg)

	exec := &mockExecutor{
		result: &StageResult{
			Stage:     StageDesignPack,
			FilePaths: []string{filepath.Join(dir, "stage-1-design-pack.md")},
			Sections: []Section{
				{Name: "design-pack", Content: "# Design Pack"},
			},
		},
	}
	router.RegisterExecutor(StageDesignPack, exec)

	result, err := router.Route(context.Background(), StageDesignPack)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, StageDesignPack, result.Stage)
	assert.Equal(t, 1, exec.called)
}

func TestRoute_Stage1_WithoutStage0_WarnsButProceeds(t *testing.T) {
	// Stage 0 is an optional prerequisite for Stage 1. When the file is
	// absent the router should log a warning but NOT return an error.
	dir := t.TempDir()
	// Intentionally do NOT write the stage-0 file.
	cfg := Config{OutputDir: dir}

	router := NewRouter(cfg)

	exec := &mockExecutor{
		result: &StageResult{
			Stage:     StageDesignPack,
			FilePaths: []string{filepath.Join(dir, "stage-1-design-pack.md")},
			Sections: []Section{
				{Name: "design-pack", Content: "# Design Pack"},
			},
		},
	}
	router.RegisterExecutor(StageDesignPack, exec)

	result, err := router.Route(context.Background(), StageDesignPack)
	require.NoError(t, err, "optional prerequisite absence should not cause an error")
	require.NotNil(t, result)
	assert.Equal(t, StageDesignPack, result.Stage)
	assert.Equal(t, 1, exec.called)
}

func TestRoute_Stage2_WithoutStage1_ReturnsError(t *testing.T) {
	// Stage 1 (design-pack) is a REQUIRED prerequisite for Stage 2
	// (implementation-skeletons). When missing, the router MUST return
	// an error with a clear message about the missing prerequisite.
	dir := t.TempDir()
	cfg := Config{OutputDir: dir}

	router := NewRouter(cfg)

	exec := &mockExecutor{
		result: &StageResult{
			Stage: StageImplementationSkeletons,
		},
	}
	router.RegisterExecutor(StageImplementationSkeletons, exec)

	result, err := router.Route(context.Background(), StageImplementationSkeletons)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "prerequisite")
	assert.Contains(t, err.Error(), "design-pack")
	// The executor must NOT have been called.
	assert.Equal(t, 0, exec.called)
}

func TestRouteRange_Stage1To3_ExecutesInOrder(t *testing.T) {
	dir := t.TempDir()

	// Stage 0 is an optional prerequisite for Stage 1 — write it so the
	// full happy path resolves cleanly.
	writeStageFile(t, dir, StageDevelopmentStandards, "# Standards")

	cfg := Config{OutputDir: dir}
	router := NewRouter(cfg)

	// Each executor writes its own output file so subsequent stages can
	// find their prerequisites on disk.
	for _, stage := range []Stage{StageDesignPack, StageImplementationSkeletons, StageTaskIndex} {
		s := stage // capture
		exec := &mockExecutor{
			result: &StageResult{
				Stage:     s,
				FilePaths: []string{filepath.Join(dir, stageFileName(s))},
				Sections: []Section{
					{Name: s.String(), Content: "# " + s.String()},
				},
			},
		}
		// The executor writes the output file so that subsequent stages
		// resolve their prerequisites.
		origExec := exec
		wrapper := &writingExecutor{
			inner: origExec,
			dir:   dir,
			stage: s,
		}
		router.RegisterExecutor(s, wrapper)
	}

	results, err := router.RouteRange(context.Background(), StageDesignPack, StageTaskIndex)
	require.NoError(t, err)
	require.Len(t, results, 3, "expected 3 results for stages 1, 2, 3")

	assert.Equal(t, StageDesignPack, results[0].Stage)
	assert.Equal(t, StageImplementationSkeletons, results[1].Stage)
	assert.Equal(t, StageTaskIndex, results[2].Stage)
}

// writingExecutor wraps a mockExecutor and writes the stage output file to
// disk so that subsequent stages can resolve their prerequisites.
type writingExecutor struct {
	inner *mockExecutor
	dir   string
	stage Stage
}

func (w *writingExecutor) Execute(ctx context.Context, cfg Config, inputs []StageResult) (*StageResult, error) {
	result, err := w.inner.Execute(ctx, cfg, inputs)
	if err != nil {
		return result, err
	}
	// Write the stage file so that later stages can find it.
	filename := stageFileName(w.stage)
	_ = os.WriteFile(filepath.Join(w.dir, filename), []byte("# "+w.stage.String()), 0644)
	return result, nil
}

func TestRouteRange_FailureAtStage2_StopsAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeStageFile(t, dir, StageDevelopmentStandards, "# Standards")
	cfg := Config{OutputDir: dir}

	router := NewRouter(cfg)

	// Stage 1: succeeds and writes its output file.
	stage1Exec := &mockExecutor{
		result: &StageResult{
			Stage:     StageDesignPack,
			FilePaths: []string{filepath.Join(dir, stageFileName(StageDesignPack))},
			Sections:  []Section{{Name: "design-pack", Content: "# Design Pack"}},
		},
	}
	router.RegisterExecutor(StageDesignPack, &writingExecutor{
		inner: stage1Exec,
		dir:   dir,
		stage: StageDesignPack,
	})

	// Stage 2: fails.
	stage2Exec := &mockExecutor{
		err: errors.New("agent unreachable"),
	}
	router.RegisterExecutor(StageImplementationSkeletons, stage2Exec)

	// Stage 3: should never be reached.
	stage3Exec := &mockExecutor{
		result: &StageResult{
			Stage: StageTaskIndex,
		},
	}
	router.RegisterExecutor(StageTaskIndex, stage3Exec)

	results, err := router.RouteRange(context.Background(), StageDesignPack, StageTaskIndex)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent unreachable")

	// Stage 1 succeeded so its result should be in the slice.
	require.Len(t, results, 1)
	assert.Equal(t, StageDesignPack, results[0].Stage)

	// Stage 3 executor must NOT have been called.
	assert.Equal(t, 0, stage3Exec.called, "stage 3 should not be attempted after stage 2 failure")
}

func TestRoute_NoExecutorRegistered_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{OutputDir: dir}

	router := NewRouter(cfg)
	// Do not register any executor.

	result, err := router.Route(context.Background(), StageDesignPack)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no executor registered")
}
