package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRunReview_SkillLayout checks that review accepts a decomposition laid
// out the way the /decompose skill writes it: Stage 4 as tasks_mNN.md files
// with no stage-4-task-specifications.md.
func TestRunReview_SkillLayout(t *testing.T) {
	root := t.TempDir()
	decompDir := filepath.Join(root, "docs", "decompose", "demo")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))

	files := map[string]string{
		"stage-1-design-pack.md":              "# Design\n",
		"stage-2-implementation-skeletons.md": "# Skeletons\n",
		"stage-3-task-index.md":               "## Target Directory Tree\n\n```\nmain.go    CREATE (M1)\n```\n",
		"tasks_m01.md":                        "- [ ] **T-01.01 — Add main**\n  - **File:** `main.go` (CREATE)\n  - **Depends on:** None\n",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(decompDir, name), []byte(content), 0o644))
	}

	require.NoError(t, runReview(context.Background(), root, "demo", cliFlags{}))
	require.FileExists(t, filepath.Join(decompDir, "review-findings.md"))
}
