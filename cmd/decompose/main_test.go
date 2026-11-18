package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_SingleStage_WritesRequestedStageAndHonorsForce runs
// "decompose <name> 2" against a decomposition that has Stage 1 but no Stage 0
// in its folder. The first run must write Stage 2 (not Stage 0), a second run
// must refuse to replace it, and --force must allow the overwrite.
func TestRun_SingleStage_WritesRequestedStageAndHonorsForce(t *testing.T) {
	root := t.TempDir()
	decompDir := filepath.Join(root, "docs", "decompose", "demo")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))
	stage1 := filepath.Join(decompDir, "stage-1-design-pack.md")
	require.NoError(t, os.WriteFile(stage1, []byte("# Finished design pack\n"), 0o644))
	stage2 := filepath.Join(decompDir, "stage-2-implementation-skeletons.md")

	require.NoError(t, run([]string{"--project-root", root, "--single-agent", "demo", "2"}))
	require.FileExists(t, stage2)
	assert.NoFileExists(t, filepath.Join(decompDir, "stage-0-development-standards.md"))

	// Mark the file as finished work, then run the same stage again.
	require.NoError(t, os.WriteFile(stage2, []byte("# Finished skeletons\n"), 0o644))
	err := run([]string{"--project-root", root, "--single-agent", "demo", "2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")
	data, readErr := os.ReadFile(stage2)
	require.NoError(t, readErr)
	assert.Equal(t, "# Finished skeletons\n", string(data))

	require.NoError(t, run([]string{"--project-root", root, "--force", "--single-agent", "demo", "2"}))
	data, readErr = os.ReadFile(stage2)
	require.NoError(t, readErr)
	assert.NotEqual(t, "# Finished skeletons\n", string(data))
}

// TestRun_HelpShowsMaxConcurrentDefaultOfOne checks that implement sessions
// run one at a time unless the user asks for more: they share one working
// tree, so the --max-concurrent default must be 1.
func TestRun_HelpShowsMaxConcurrentDefaultOfOne(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stderr
	os.Stderr = w
	runErr := run([]string{"--help"})
	os.Stderr = orig
	require.NoError(t, w.Close())
	require.NoError(t, runErr)

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Regexp(t, `-max-concurrent int\n[^\n]*\(default 1\)`, string(out))
}
