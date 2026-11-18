package orchestrator

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIReviewStrategy_Approve(t *testing.T) {
	input := bytes.NewBufferString("a\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}
	artifacts := []ImplementationArtifact{
		{Path: "cmd/main.go", Action: "CREATE"},
	}

	decision, err := strategy.RequestReview(context.Background(), milestone, artifacts)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)
	assert.Contains(t, output.String(), "M1")
	assert.Contains(t, output.String(), "cmd/main.go")
}

func TestCLIReviewStrategy_Reject(t *testing.T) {
	input := bytes.NewBufferString("r\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}

	decision, err := strategy.RequestReview(context.Background(), milestone, nil)
	require.NoError(t, err)
	assert.Equal(t, ReviewRejected, decision)
}

func TestCLIReviewStrategy_ViewThenApprove(t *testing.T) {
	input := bytes.NewBufferString("v\na\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M2", Name: "Core"}
	artifacts := []ImplementationArtifact{
		{Path: "pkg/core.go", Action: "CREATE", DiffOrBody: "+package core\n+func Init() {}"},
	}

	decision, err := strategy.RequestReview(context.Background(), milestone, artifacts)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)
	assert.Contains(t, output.String(), "+package core")
}

func TestCLIReviewStrategy_InvalidThenApprove(t *testing.T) {
	input := bytes.NewBufferString("x\napprove\n")
	output := &bytes.Buffer{}

	strategy := NewCLIReviewStrategy(input, output)
	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}

	decision, err := strategy.RequestReview(context.Background(), milestone, nil)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)
	assert.Contains(t, output.String(), "Unknown option")
}

func TestFileReviewStrategy_Approve(t *testing.T) {
	dir := t.TempDir()
	strategy := &FileReviewStrategy{
		reportDir:    dir,
		pollInterval: 50 * time.Millisecond,
	}

	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}
	artifacts := []ImplementationArtifact{
		{Path: "cmd/main.go", Action: "CREATE"},
	}

	// Create the approval file after a brief delay.
	go func() {
		time.Sleep(150 * time.Millisecond)
		approvedPath := filepath.Join(dir, "milestone-m1-review.approved")
		_ = os.WriteFile(approvedPath, []byte("ok"), 0o644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	decision, err := strategy.RequestReview(ctx, milestone, artifacts)
	require.NoError(t, err)
	assert.Equal(t, ReviewApproved, decision)

	// Verify the review report was written.
	reportPath := filepath.Join(dir, "milestone-m1-review.md")
	data, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "M1")
	assert.Contains(t, string(data), "cmd/main.go")
}

func TestFileReviewStrategy_Reject(t *testing.T) {
	dir := t.TempDir()
	strategy := &FileReviewStrategy{
		reportDir:    dir,
		pollInterval: 50 * time.Millisecond,
	}

	milestone := &MilestoneNode{ID: "M2", Name: "Core"}

	go func() {
		time.Sleep(150 * time.Millisecond)
		rejectedPath := filepath.Join(dir, "milestone-m2-review.rejected")
		_ = os.WriteFile(rejectedPath, []byte("not good"), 0o644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	decision, err := strategy.RequestReview(ctx, milestone, nil)
	require.NoError(t, err)
	assert.Equal(t, ReviewRejected, decision)
}

func TestFileReviewStrategy_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	strategy := &FileReviewStrategy{
		reportDir:    dir,
		pollInterval: 50 * time.Millisecond,
	}

	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	decision, err := strategy.RequestReview(ctx, milestone, nil)
	require.Error(t, err)
	assert.Equal(t, ReviewRejected, decision)
}

// gitIn runs git in dir and returns its trimmed output.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// initTempRepo creates a git repository with one commit and a local bare
// "origin" remote, and returns the repository path. Git is cut off from the
// user's global and system config so signing or hooks set there cannot
// change the result.
func initTempRepo(t *testing.T) string {
	t.Helper()
	emptyCfg := filepath.Join(t.TempDir(), "gitconfig")
	require.NoError(t, os.WriteFile(emptyCfg, nil, 0o644))
	t.Setenv("GIT_CONFIG_GLOBAL", emptyCfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gitIn(t, root, "init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("# demo\n"), 0o644))
	gitIn(t, root, "add", "README.md")
	gitIn(t, root, "commit", "-q", "-m", "init")

	bare := t.TempDir()
	gitIn(t, bare, "init", "-q", "--bare")
	gitIn(t, root, "remote", "add", "origin", bare)
	return root
}

// fakeGH puts a stand-in "gh" script first on PATH. The script records the
// folder it ran in, fails "pr create", and prints MERGED for anything else.
// It returns the file the folder is recorded in.
func fakeGH(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}
	binDir := t.TempDir()
	pwdFile := filepath.Join(t.TempDir(), "pwd")
	script := "#!/bin/sh\npwd -P > \"$FAKE_GH_PWD\"\nif [ \"$1 $2\" = \"pr create\" ]; then exit 1; fi\necho MERGED\n"
	ghPath := filepath.Join(binDir, "gh")
	require.NoError(t, os.WriteFile(ghPath, []byte(script), 0o755))
	t.Setenv("FAKE_GH_PWD", pwdFile)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// The real gh must never be reached from a test.
	resolved, err := exec.LookPath("gh")
	require.NoError(t, err)
	require.Equal(t, ghPath, resolved)
	return pwdFile
}

// TestPRReviewStrategy_RunsInProjectRoot checks that PR mode runs git and gh
// in the project root it was given, not in the folder the process started in.
func TestPRReviewStrategy_RunsInProjectRoot(t *testing.T) {
	root := initTempRepo(t)
	pwdFile := fakeGH(t)
	// Start somewhere that is not a git repository, so a command that
	// ignores the project root fails instead of touching another repo.
	t.Chdir(t.TempDir())

	require.NoError(t, os.WriteFile(filepath.Join(root, "new.go"), []byte("package demo\n"), 0o644))

	strategy := NewPRReviewStrategy(root, "main", "origin")
	milestone := &MilestoneNode{ID: "M1", Name: "Foundation"}
	artifacts := []ImplementationArtifact{{Path: "new.go", Action: "CREATE"}}

	_, err := strategy.RequestReview(context.Background(), milestone, artifacts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create PR")

	assert.Equal(t, "implement M1: Foundation", gitIn(t, root, "log", "-1", "--format=%s", "implement/m1"))
	assert.Equal(t, "new.go", gitIn(t, root, "show", "--name-only", "--format=", "implement/m1"))
	assert.NotEmpty(t, gitIn(t, root, "ls-remote", "origin", "refs/heads/implement/m1"))

	ranIn, err := os.ReadFile(pwdFile)
	require.NoError(t, err)
	assert.Equal(t, root, strings.TrimSpace(string(ranIn)))
}

// TestGetPRState_RunsInProjectRoot checks that the PR status poll runs gh in
// the project root.
func TestGetPRState_RunsInProjectRoot(t *testing.T) {
	pwdFile := fakeGH(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Chdir(t.TempDir())

	state, err := getPRState(context.Background(), root, "implement/m1")
	require.NoError(t, err)
	assert.Equal(t, "MERGED", state)

	ranIn, err := os.ReadFile(pwdFile)
	require.NoError(t, err)
	assert.Equal(t, root, strings.TrimSpace(string(ranIn)))
}
