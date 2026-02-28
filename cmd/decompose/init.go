package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dusk-indust/decompose/internal/skilldata"
)

// mcpConfig represents the structure of a .mcp.json file.
type mcpConfig struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

// buildMCPEntry creates the MCP server configuration using the absolute path
// of the currently running binary and the target project root.
func buildMCPEntry(projectRoot string) json.RawMessage {
	exe, err := os.Executable()
	if err != nil {
		exe = "decompose" // fallback to PATH lookup
	} else {
		exe, _ = filepath.EvalSymlinks(exe)
	}

	entry := map[string]interface{}{
		"type":    "stdio",
		"command": exe,
		"args":    []string{"--project-root", projectRoot, "--serve-mcp"},
	}
	data, _ := json.Marshal(entry)
	return json.RawMessage(data)
}

// runInit installs the decompose skill files and MCP configuration into the
// target project directory.
func runInit(projectRoot string, force bool) error {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolving project root: %w", err)
	}

	skillDir := filepath.Join(abs, ".claude", "skills", "decompose")
	mcpPath := filepath.Join(abs, ".mcp.json")

	// --- Copy embedded skill files ---

	root := "skill/decompose"
	err = fs.WalkDir(skilldata.SkillFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute the relative path from the embed root.
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		dest := filepath.Join(skillDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		// Check if file already exists.
		if !force {
			if _, err := os.Stat(dest); err == nil {
				fmt.Printf("  skipped %s (exists, use --force to overwrite)\n", dotRelative(abs, dest))
				return nil
			}
		}

		data, err := skilldata.SkillFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dest, err)
		}

		fmt.Printf("  created %s\n", dotRelative(abs, dest))
		return nil
	})
	if err != nil {
		return fmt.Errorf("copying skill files: %w", err)
	}

	// --- Install hook scripts ---

	hooksDir := filepath.Join(abs, ".claude", "hooks")
	err = fs.WalkDir(skilldata.HooksFS, "hooks", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return os.MkdirAll(hooksDir, 0o755)
		}

		dest := filepath.Join(hooksDir, d.Name())

		if !force {
			if _, statErr := os.Stat(dest); statErr == nil {
				fmt.Printf("  skipped %s (exists, use --force to overwrite)\n", dotRelative(abs, dest))
				return nil
			}
		}

		data, readErr := skilldata.HooksFS.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading embedded %s: %w", path, readErr)
		}

		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			return mkErr
		}
		if writeErr := os.WriteFile(dest, data, 0o755); writeErr != nil {
			return fmt.Errorf("writing %s: %w", dest, writeErr)
		}

		fmt.Printf("  created %s\n", dotRelative(abs, dest))
		return nil
	})
	if err != nil {
		return fmt.Errorf("copying hook files: %w", err)
	}

	// Check for hook dependencies.
	if _, err := exec.LookPath("jq"); err != nil {
		fmt.Fprintln(os.Stderr, "  note: the augmentation hook requires 'jq' (not found in PATH)")
		fmt.Fprintln(os.Stderr, "        install with: brew install jq (macOS) or apt install jq (Linux)")
	}

	// --- Create/merge .mcp.json ---

	if err := mergeMCPConfig(mcpPath, abs, force); err != nil {
		return err
	}

	// --- Create/merge .claude/settings.json with hook config ---

	settingsPath := filepath.Join(abs, ".claude", "settings.json")
	if err := mergeSettings(settingsPath, force); err != nil {
		return err
	}

	// --- Append decompose block to CLAUDE.md ---

	claudeMDPath := filepath.Join(abs, "CLAUDE.md")
	if err := mergeClaudeMD(claudeMDPath, abs); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update CLAUDE.md: %v\n", err)
	}

	// --- Add .decompose/ to .gitignore ---

	gitignorePath := filepath.Join(abs, ".gitignore")
	addToGitignore(gitignorePath, ".decompose/")

	fmt.Println("\nSetup complete. The /decompose skill and MCP server are ready.")
	return nil
}

// mergeMCPConfig creates or merges the decompose entry into .mcp.json.
func mergeMCPConfig(mcpPath, projectRoot string, force bool) error {
	var cfg mcpConfig

	data, err := os.ReadFile(mcpPath)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parsing %s: %w", mcpPath, err)
		}
	}

	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]json.RawMessage)
	}

	if _, exists := cfg.MCPServers["decompose"]; exists && !force {
		fmt.Printf("  skipped .mcp.json decompose entry (exists, use --force to overwrite)\n")
		return nil
	}

	cfg.MCPServers["decompose"] = buildMCPEntry(projectRoot)

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling .mcp.json: %w", err)
	}

	if err := os.WriteFile(mcpPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", mcpPath, err)
	}

	action := "created"
	if data != nil {
		action = "updated"
	}
	fmt.Printf("  %s .mcp.json with decompose MCP server\n", action)
	return nil
}

// settingsConfig represents the structure of .claude/settings.json.
type settingsConfig struct {
	Hooks map[string]json.RawMessage `json:"hooks,omitempty"`
	Rest  map[string]json.RawMessage `json:"-"` // preserve unknown keys
}

// mergeSettings creates or merges the hook configuration into .claude/settings.json.
func mergeSettings(settingsPath string, force bool) error {
	hookConfig := json.RawMessage(`[
    {
      "matcher": "Read|Write|Edit|Glob|Grep|Bash",
      "hooks": [
        {
          "type": "command",
          "command": ".claude/hooks/decompose-tool-guard.sh",
          "timeout": 8
        }
      ]
    }
  ]`)

	// Read existing file.
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
			return fmt.Errorf("parsing %s: %w", settingsPath, jsonErr)
		}
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}

	// Check if hooks already exist.
	if _, exists := raw["hooks"]; exists && !force {
		fmt.Printf("  skipped %s hooks (exists, use --force to overwrite)\n", dotRelative(filepath.Dir(filepath.Dir(settingsPath)), settingsPath))
		return nil
	}

	// Merge hooks key.
	var hooks map[string]json.RawMessage
	if existing, ok := raw["hooks"]; ok {
		_ = json.Unmarshal(existing, &hooks)
	}
	if hooks == nil {
		hooks = make(map[string]json.RawMessage)
	}
	hooks["PreToolUse"] = hookConfig

	hooksJSON, _ := json.Marshal(hooks)
	raw["hooks"] = hooksJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	fmt.Printf("  created %s with hook config\n", dotRelative(filepath.Dir(filepath.Dir(settingsPath)), settingsPath))
	return nil
}

const claudeMDMarkerStart = "<!-- decompose:start -->"
const claudeMDMarkerEnd = "<!-- decompose:end -->"

const claudeMDBlock = `<!-- decompose:start -->
## Decompose Code Intelligence

This project has a decompose MCP server with code intelligence tools powered by tree-sitter and a graph database. For code understanding tasks, these tools provide richer context than manual file operations:

- ` + "`mcp__decompose__build_graph`" + ` — index the codebase (run once per session, persists to .decompose/graph/)
- ` + "`mcp__decompose__query_symbols`" + ` — find functions, types, interfaces by name
- ` + "`mcp__decompose__get_dependencies`" + ` — trace upstream/downstream dependencies
- ` + "`mcp__decompose__assess_impact`" + ` — compute blast radius of file changes
- ` + "`mcp__decompose__get_clusters`" + ` — discover tightly-coupled file groups

For the /decompose skill specifically:
- ` + "`mcp__decompose__get_stage_context`" + ` — load templates and prerequisite content
- ` + "`mcp__decompose__write_stage`" + ` — write stage files with validation and coherence checking
- ` + "`mcp__decompose__get_status`" + ` — check decomposition progress
<!-- decompose:end -->`

// mergeClaudeMD appends or replaces the decompose block in CLAUDE.md.
func mergeClaudeMD(claudeMDPath, projectRoot string) error {
	data, err := os.ReadFile(claudeMDPath)
	content := ""
	if err == nil {
		content = string(data)
	}

	// Check if block already exists — replace it.
	if strings.Contains(content, claudeMDMarkerStart) {
		startIdx := strings.Index(content, claudeMDMarkerStart)
		endIdx := strings.Index(content, claudeMDMarkerEnd)
		if endIdx > startIdx {
			content = content[:startIdx] + claudeMDBlock + content[endIdx+len(claudeMDMarkerEnd):]
		}
	} else {
		// Append block.
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if content != "" {
			content += "\n"
		}
		content += claudeMDBlock + "\n"
	}

	if err := os.WriteFile(claudeMDPath, []byte(content), 0o644); err != nil {
		return err
	}

	fmt.Printf("  updated %s with decompose block\n", dotRelative(projectRoot, claudeMDPath))
	return nil
}

// addToGitignore adds a pattern to .gitignore if not already present.
func addToGitignore(gitignorePath, pattern string) {
	data, err := os.ReadFile(gitignorePath)
	content := ""
	if err == nil {
		content = string(data)
	}

	if strings.Contains(content, pattern) {
		return
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += pattern + "\n"

	if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
	}
}

// dotRelative returns a display path relative to the project root, prefixed
// with "./".
func dotRelative(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return "./" + rel
}
