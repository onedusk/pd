package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunImplement_Stage0Optional checks that a decomposition with stages 1-4
// but no shared Stage 0 passes the completeness gate. The Stage 3 file has no
// milestones, so runImplement stops at milestone parsing before spawning any
// implementation sessions.
func TestRunImplement_Stage0Optional(t *testing.T) {
	root := t.TempDir()
	decompDir := filepath.Join(root, "docs", "decompose", "demo")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))
	for _, f := range []string{
		"stage-1-design-pack.md",
		"stage-2-implementation-skeletons.md",
		"stage-3-task-index.md",
		"stage-4-task-specifications.md",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(decompDir, f), []byte("# placeholder\n"), 0o644))
	}

	err := runImplement(context.Background(), root, "demo", cliFlags{SkipReview: true})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "stage 0")
	assert.Contains(t, err.Error(), "parse milestones")
}
