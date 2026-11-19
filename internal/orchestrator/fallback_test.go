package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallback_BasicTemplate_Stage0(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:      "test-project",
		OutputDir: tmpDir,
	}

	fb := NewFallbackExecutor(CapBasic)
	ctx := context.Background()

	// Stage 0 has no inputs.
	result, err := fb.Execute(ctx, cfg, StageDevelopmentStandards, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StageDevelopmentStandards, result.Stage)
	require.Len(t, result.FilePaths, 1)

	// Check file exists.
	content, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(content)

	assert.Contains(t, text, "Stage 0")
	assert.Contains(t, text, "development-standards")
	assert.Contains(t, text, "TODO")
	assert.Contains(t, text, "basic mode")

	// Check section.
	require.Len(t, result.Sections, 1)
	assert.Equal(t, "development-standards", result.Sections[0].Name)
	assert.Equal(t, "template", result.Sections[0].Agent)
}

func TestFallback_BasicTemplate_DesignPack(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:      "test-project",
		OutputDir: tmpDir,
	}

	fb := NewFallbackExecutor(CapBasic)
	ctx := context.Background()

	// Stage 1 with Stage 0 as input.
	inputs := []StageResult{
		{Stage: StageDevelopmentStandards, Sections: []Section{{Name: "dev-standards", Content: "test"}}},
	}

	result, err := fb.Execute(ctx, cfg, StageDesignPack, inputs)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StageDesignPack, result.Stage)

	content, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(content)

	// Should have all 13 section headers from Stage1MergePlan.
	expectedSections := []string{
		"assumptions", "platform-baseline", "data-model", "architecture",
		"features", "integrations", "security", "adrs", "pdrs", "prd",
		"data-lifecycle", "testing", "implementation-plan",
	}
	for _, sec := range expectedSections {
		assert.Contains(t, text, "## "+sec, "missing section header: %s", sec)
	}

	assert.Equal(t, len(expectedSections), len(result.Sections))

	// Each section should have a TODO marker.
	for _, sec := range result.Sections {
		assert.Contains(t, sec.Content, "TODO")
	}
}

func TestFallback_MCPOnly_DesignPack(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:      "test-project",
		OutputDir: tmpDir,
	}

	fb := NewFallbackExecutor(CapMCPOnly)
	ctx := context.Background()

	inputs := []StageResult{
		{Stage: StageDevelopmentStandards, Sections: []Section{{Name: "dev-standards", Content: "test content"}}},
	}

	result, err := fb.Execute(ctx, cfg, StageDesignPack, inputs)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StageDesignPack, result.Stage)

	content, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(content)

	assert.Contains(t, text, "MCP-only mode")
	assert.Contains(t, text, "MCP tools")

	// All 13 section headers.
	for _, sec := range Stage1MergePlan.SectionOrder {
		assert.Contains(t, text, "## "+sec)
	}

	// Agent should be "mcp-local".
	for _, sec := range result.Sections {
		assert.Equal(t, "mcp-local", sec.Agent)
	}
}

func TestFallback_MCPOnly_WithContext(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:      "test-project",
		OutputDir: tmpDir,
	}

	fb := NewFallbackExecutor(CapMCPOnly)
	ctx := context.Background()

	// Stage 2 with Stage 0 and Stage 1 as inputs.
	inputs := []StageResult{
		{Stage: StageDevelopmentStandards, Sections: []Section{{Name: "dev-standards", Content: "prior context here"}}},
		{Stage: StageDesignPack, Sections: []Section{{Name: "design", Content: "design pack content"}}},
	}

	result, err := fb.Execute(ctx, cfg, StageImplementationSkeletons, inputs)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, StageImplementationSkeletons, result.Stage)

	content, err := os.ReadFile(result.FilePaths[0])
	require.NoError(t, err)
	text := string(content)

	// Should include prior context.
	assert.Contains(t, text, "Context from prior stages")
	assert.Contains(t, text, "prior context here")

	// Stage 2 has 3 sections.
	assert.Equal(t, 3, len(result.Sections))
}

func TestFallback_UnsupportedLevel(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:      "test-project",
		OutputDir: tmpDir,
	}

	fb := NewFallbackExecutor(CapA2AMCP)
	ctx := context.Background()

	_, err := fb.Execute(ctx, cfg, StageDevelopmentStandards, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported capability level")
}

func TestFallback_OutputFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:      "test-project",
		OutputDir: tmpDir,
	}

	fb := NewFallbackExecutor(CapBasic)
	ctx := context.Background()

	result, err := fb.Execute(ctx, cfg, StageDevelopmentStandards, nil)
	require.NoError(t, err)

	// Check the output file is at the correct path.
	expectedPath := filepath.Join(tmpDir, "stage-0-development-standards.md")
	assert.Equal(t, expectedPath, result.FilePaths[0])

	// Check it's valid markdown: no unclosed code blocks.
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	text := string(content)

	// Count code block markers — should be even (0 is even).
	codeBlocks := strings.Count(text, "```")
	assert.Equal(t, 0, codeBlocks%2, "unclosed code blocks in output")

	// Headers should start with #.
	assert.True(t, strings.HasPrefix(text, "# "))
}

// TestFallback_ExistingStageFile checks that the fallback executor refuses to
// replace an existing stage file, and overwrites it only when Force is set.
func TestFallback_ExistingStageFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		Name:      "test-project",
		OutputDir: tmpDir,
	}
	outPath := filepath.Join(tmpDir, "stage-1-design-pack.md")
	require.NoError(t, os.WriteFile(outPath, []byte("# Finished design pack"), 0o644))

	fb := NewFallbackExecutor(CapBasic)
	ctx := context.Background()

	_, err := fb.Execute(ctx, cfg, StageDesignPack, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Contains(t, err.Error(), "--force")

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, "# Finished design pack", string(content))

	cfg.Force = true
	result, err := fb.Execute(ctx, cfg, StageDesignPack, nil)
	require.NoError(t, err)
	assert.Equal(t, StageDesignPack, result.Stage)

	content, err = os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "basic mode")
}
