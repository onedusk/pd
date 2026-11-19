package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A decompose.yml written for an earlier version may still carry keys that
// are no longer fields. It must load, and the keys still in use must be read.
func TestLoadIgnoresRemovedKeys(t *testing.T) {
	dir := t.TempDir()
	yml := "outputDir: out\n" +
		"languages: [go]\n" +
		"excludeDirs: [vendor]\n" +
		"templatePath: tpl\n" +
		"verbose: true\n" +
		"graphExcludes: [dist]\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "decompose.yml"), []byte(yml), 0o644))

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "out", cfg.OutputDir)
	assert.True(t, cfg.Verbose)
	assert.False(t, cfg.SingleAgent)
}
