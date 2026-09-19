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
