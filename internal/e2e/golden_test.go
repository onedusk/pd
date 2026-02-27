//go:build e2e

package e2e

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dusk-indust/decompose/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

// goldenDir returns the path to the testdata/golden directory.
func goldenDir() string {
	return filepath.Join("..", "..", "testdata", "golden")
}

// stageGoldenFiles maps stage output filenames to golden filenames.
var stageGoldenFiles = []struct {
	stage  string
	golden string
}{
	{"stage-0-development-standards.md", "stage0_output.md"},
	{"stage-1-design-pack.md", "stage1_output.md"},
	{"stage-2-implementation-skeletons.md", "stage2_output.md"},
	{"stage-3-task-index.md", "stage3_output.md"},
	{"stage-4-task-specifications.md", "stage4_output.md"},
}

// runPipelineForGolden runs the full pipeline in CapBasic mode and returns the
// output directory.
func runPipelineForGolden(t *testing.T) string {
	t.Helper()

	outputDir := t.TempDir()
	cfg := orchestrator.Config{
		Name:        "golden-test",
		ProjectRoot: filepath.Join("..", "..", "testdata", "fixtures", "go_project"),
		OutputDir:   outputDir,
		Capability:  orchestrator.CapBasic,
	}

	pipeline := orchestrator.NewPipeline(cfg, &noopA2AClient{})
	progressCh := pipeline.Progress()
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range progressCh {
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, err := pipeline.RunPipeline(ctx, orchestrator.StageDevelopmentStandards, orchestrator.StageTaskSpecifications)
	require.NoError(t, err)

	pipeline.Close()
	<-drainDone

	return outputDir
}

// TestGolden compares the pipeline output against golden files. If golden files
// do not exist, the test is skipped with a message to run with -update.
func TestGolden(t *testing.T) {
	outputDir := runPipelineForGolden(t)
	gDir := goldenDir()

	for _, sg := range stageGoldenFiles {
		t.Run(sg.golden, func(t *testing.T) {
			goldenPath := filepath.Join(gDir, sg.golden)
			golden, err := os.ReadFile(goldenPath)
			if os.IsNotExist(err) {
				t.Skipf("golden file %s not found; run with -update to generate", sg.golden)
				return
			}
			require.NoError(t, err)

			actual, err := os.ReadFile(filepath.Join(outputDir, sg.stage))
			require.NoError(t, err)

			assert.Equal(t, string(golden), string(actual),
				"output for %s does not match golden file", sg.stage)
		})
	}
}

// TestUpdateGolden regenerates golden files from the current pipeline output.
// Run with: go test -tags e2e -run TestUpdateGolden ./internal/e2e/ -update
func TestUpdateGolden(t *testing.T) {
	if !*update {
		t.Skip("skipping golden file update; run with -update flag")
	}

	outputDir := runPipelineForGolden(t)
	gDir := goldenDir()

	err := os.MkdirAll(gDir, 0o755)
	require.NoError(t, err)

	for _, sg := range stageGoldenFiles {
		data, err := os.ReadFile(filepath.Join(outputDir, sg.stage))
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(gDir, sg.golden), data, 0o644)
		require.NoError(t, err)

		t.Logf("updated %s", sg.golden)
	}
}
