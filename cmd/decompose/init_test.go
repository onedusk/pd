package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testGuardCommand = ".claude/hooks/decompose-tool-guard.sh"

// readJSONFile decodes a JSON file into a generic map.
func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

// preToolUseEntries returns the hooks.PreToolUse list of a settings file.
func preToolUseEntries(t *testing.T, settingsPath string) []any {
	t.Helper()
	hooks, ok := readJSONFile(t, settingsPath)["hooks"].(map[string]any)
	require.True(t, ok, "settings must have a hooks object")
	entries, ok := hooks["PreToolUse"].([]any)
	require.True(t, ok, "hooks must have a PreToolUse list")
	return entries
}

// countGuardEntries counts the PreToolUse entries that run the decompose hook.
func countGuardEntries(entries []any) int {
	n := 0
	for _, e := range entries {
		data, _ := json.Marshal(e)
		if strings.Contains(string(data), testGuardCommand) {
			n++
		}
	}
	return n
}

// TestInit_FreshInstall runs init on an empty project and checks the MCP
// entry, the hook script, and the hook settings are all written.
func TestInit_FreshInstall(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, runInit(root, false))

	mcp := readJSONFile(t, filepath.Join(root, ".mcp.json"))
	servers, ok := mcp["mcpServers"].(map[string]any)
	require.True(t, ok)
	entry, ok := servers["decompose"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"--project-root", root, "--serve-mcp"}, entry["args"])

	info, err := os.Stat(filepath.Join(root, ".claude", "hooks", "decompose-tool-guard.sh"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "hook script must be executable")

	entries := preToolUseEntries(t, filepath.Join(root, ".claude", "settings.json"))
	require.Len(t, entries, 1)
	assert.Equal(t, 1, countGuardEntries(entries))
}

// TestMergeSettings_KeepsUnrelatedHooks covers a settings file that already
// has a "hooks" key for another event: the decompose hook must still be
// installed, and nothing the user wrote may change.
func TestMergeSettings_KeepsUnrelatedHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{
  "permissions": {"allow": ["Bash(ls:*)"]},
  "hooks": {
    "PostToolUse": [
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "fmt.sh"}]}
    ]
  }
}`), 0o644))
	before := readJSONFile(t, path)

	require.NoError(t, mergeSettings(path, false))

	after := readJSONFile(t, path)
	assert.Equal(t, before["permissions"], after["permissions"])
	assert.Equal(t, before["hooks"].(map[string]any)["PostToolUse"], after["hooks"].(map[string]any)["PostToolUse"])
	entries := preToolUseEntries(t, path)
	require.Len(t, entries, 1)
	assert.Equal(t, 1, countGuardEntries(entries))
}

// TestMergeSettings_KeepsExistingPreToolUseHooks covers a settings file that
// already has the user's own PreToolUse hook: the decompose entry is added
// after it, and --force replaces only the decompose entry.
func TestMergeSettings_KeepsExistingPreToolUseHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "my-guard.sh"}]}
    ]
  }
}`), 0o644))
	userEntry := preToolUseEntries(t, path)[0]

	require.NoError(t, mergeSettings(path, false))
	entries := preToolUseEntries(t, path)
	require.Len(t, entries, 2)
	assert.Equal(t, userEntry, entries[0])
	assert.Equal(t, 1, countGuardEntries(entries))

	require.NoError(t, mergeSettings(path, true))
	entries = preToolUseEntries(t, path)
	require.Len(t, entries, 2)
	assert.Equal(t, userEntry, entries[0])
	assert.Equal(t, 1, countGuardEntries(entries))
}

// TestMergeSettings_ForceKeepsSharedEntry covers an entry where the user put
// their own hook next to the decompose hook: --force must not drop it.
func TestMergeSettings_ForceKeepsSharedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	original := []byte(`{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Read", "hooks": [
        {"type": "command", "command": ".claude/hooks/decompose-tool-guard.sh"},
        {"type": "command", "command": "my-guard.sh"}
      ]}
    ]
  }
}`)
	require.NoError(t, os.WriteFile(path, original, 0o644))

	require.NoError(t, mergeSettings(path, true))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(data))
}

// TestMergeSettings_RejectsUnexpectedHooksShape checks that a "hooks" value
// init cannot merge into is reported and left alone, not replaced.
func TestMergeSettings_RejectsUnexpectedHooksShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	original := []byte(`{"hooks": {"PreToolUse": {"matcher": "Bash"}}}`)
	require.NoError(t, os.WriteFile(path, original, 0o644))

	require.Error(t, mergeSettings(path, true))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(data))
}

// TestInit_RerunIsNoOp runs init twice and checks the second run leaves
// .mcp.json and the hook settings byte-for-byte unchanged.
func TestInit_RerunIsNoOp(t *testing.T) {
	root := t.TempDir()
	mcpPath := filepath.Join(root, ".mcp.json")
	settingsPath := filepath.Join(root, ".claude", "settings.json")

	require.NoError(t, runInit(root, false))
	mcpBefore, err := os.ReadFile(mcpPath)
	require.NoError(t, err)
	settingsBefore, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	require.NoError(t, runInit(root, false))
	mcpAfter, err := os.ReadFile(mcpPath)
	require.NoError(t, err)
	settingsAfter, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Equal(t, string(mcpBefore), string(mcpAfter))
	assert.Equal(t, string(settingsBefore), string(settingsAfter))

	// --force rewrites the files but must not add a second hook entry.
	require.NoError(t, runInit(root, true))
	entries := preToolUseEntries(t, settingsPath)
	require.Len(t, entries, 1)
	assert.Equal(t, 1, countGuardEntries(entries))
}

// TestMergeMCPConfig_KeepsOtherTopLevelKeys checks that writing the decompose
// server into an existing .mcp.json keeps other servers and every top-level
// key other than mcpServers, with and without --force.
func TestMergeMCPConfig_KeepsOtherTopLevelKeys(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".mcp.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
  "$schema": "https://example.com/mcp.schema.json",
  "inputs": [{"id": "token", "type": "promptString"}],
  "mcpServers": {
    "other": {"command": "other-server", "args": ["--stdio"]}
  }
}`), 0o644))
	before := readJSONFile(t, path)

	for _, force := range []bool{false, true} {
		require.NoError(t, mergeMCPConfig(path, root, force))

		after := readJSONFile(t, path)
		assert.Equal(t, before["$schema"], after["$schema"])
		assert.Equal(t, before["inputs"], after["inputs"])
		servers, ok := after["mcpServers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, before["mcpServers"].(map[string]any)["other"], servers["other"])
		assert.Contains(t, servers, "decompose")
	}
}
