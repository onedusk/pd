package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onedusk/pd/internal/orchestrator"
)

// TestImplementMilestone_SendsPromptOnStdin checks that the prompt reaches
// claude on stdin and not as a command-line argument. Linux caps one argument
// at 128 KB, and the prompt (Stage 0 + Stage 2 + task spec) can be larger.
func TestImplementMilestone_SendsPromptOnStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell")
	}

	// A stand-in "claude" that records its arguments and stdin.
	binDir := t.TempDir()
	outDir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_CLAUDE_OUT/args\"\ncat > \"$FAKE_CLAUDE_OUT/stdin\"\n"
	claudePath := filepath.Join(binDir, "claude")
	require.NoError(t, os.WriteFile(claudePath, []byte(script), 0o755))
	t.Setenv("FAKE_CLAUDE_OUT", outDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// The real claude must never be reached from a test.
	resolved, err := exec.LookPath("claude")
	require.NoError(t, err)
	require.Equal(t, claudePath, resolved)

	root := t.TempDir()
	decompDir := filepath.Join(root, "docs", "decompose", "demo")
	require.NoError(t, os.MkdirAll(decompDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(decompDir, "stage-2-implementation-skeletons.md"), []byte("type Demo struct{}\n"), 0o644))

	// A task spec larger than the 128 KB single-argument cap.
	spec := "- [ ] **T-01.01 Add main**\n" + strings.Repeat("outline line\n", 16*1024)
	require.Greater(t, len(spec), 128*1024)
	specPath := filepath.Join(decompDir, "tasks_m01.md")
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o644))

	ia := NewImplementerAgent(root, decompDir)
	milestone := &orchestrator.MilestoneNode{ID: "M1", Name: "Foundation", TaskSpecPath: specPath}

	_, err = ia.ImplementMilestone(context.Background(), milestone)
	require.NoError(t, err)

	stdin, err := os.ReadFile(filepath.Join(outDir, "stdin"))
	require.NoError(t, err)
	want := ia.composePrompt(spec)
	assert.Equal(t, len(want), len(stdin))
	assert.True(t, want == string(stdin), "stdin must carry the composed prompt")

	argsData, err := os.ReadFile(filepath.Join(outDir, "args"))
	require.NoError(t, err)
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	assert.Equal(t, []string{"--print", "--max-turns", "50"}, args)
}
