package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

// TestGetCommitHash_UsesProjectRoot checks that the commit hash comes from the
// repository at the project root, not from the folder the process started in.
func TestGetCommitHash_UsesProjectRoot(t *testing.T) {
	emptyCfg := filepath.Join(t.TempDir(), "gitconfig")
	require.NoError(t, os.WriteFile(emptyCfg, nil, 0o644))
	t.Setenv("GIT_CONFIG_GLOBAL", emptyCfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	root := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# demo\n"), 0o644))
	git("add", "README.md")
	git("commit", "-q", "-m", "init")
	want := git("rev-parse", "HEAD")

	// Start somewhere that is not a git repository.
	t.Chdir(t.TempDir())

	assert.Equal(t, want, getCommitHash(root))
}
