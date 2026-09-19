package status

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextStage(t *testing.T) {
	tests := []struct {
		name      string
		completed []int
		want      int
	}{
		{"nothing complete", nil, 0},
		{"stage 0 only", []int{0}, 1},
		{"stage 0 is optional", []int{1}, 2},
		{"contiguous", []int{0, 1}, 2},
		{"gap returns earliest missing", []int{0, 1, 3}, 2},
		{"gap with unordered input", []int{3, 1, 0}, 2},
		{"stage 1 missing", []int{2}, 1},
		{"all complete without stage 0", []int{1, 2, 3, 4}, -1},
		{"all complete", []int{0, 1, 2, 3, 4}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NextStage(tt.completed))
		})
	}
}

func TestCheckExists(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "decompose", "present"), 0o755))

	assert.NoError(t, CheckExists(root, "present"))

	err := CheckExists(root, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestScanCompletedStages_Stage4Layouts(t *testing.T) {
	write := func(t *testing.T, dir string, names ...string) {
		t.Helper()
		for _, n := range names {
			require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644))
		}
	}

	t.Run("per-milestone task files written by the skill", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "stage-1-design-pack.md", "tasks_m01.md", "tasks_m02.md")
		assert.Equal(t, []int{1, 4}, ScanCompletedStages(dir))
	})

	t.Run("single file written by the pipeline", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "stage-4-task-specifications.md")
		assert.Equal(t, []int{4}, ScanCompletedStages(dir))
	})

	t.Run("no stage 4 output", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "stage-3-task-index.md")
		assert.Equal(t, []int{3}, ScanCompletedStages(dir))
	})
}

func TestGetDecompositionStatus_TaskFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "decompose", "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, n := range []string{
		"stage-1-design-pack.md",
		"stage-2-implementation-skeletons.md",
		"stage-3-task-index.md",
		"tasks_m02.md",
		"tasks_m01.md",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644))
	}

	ds := GetDecompositionStatus(root, "demo")
	assert.True(t, ds.Stages[4].Complete)
	assert.Equal(t, filepath.Join(dir, "tasks_m01.md"), ds.Stages[4].FilePath)
	assert.Equal(t, -1, ds.NextStage)
}
