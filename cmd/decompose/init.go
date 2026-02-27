package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dusk-indust/decompose/internal/skilldata"
)

// mcpConfig represents the structure of a .mcp.json file.
type mcpConfig struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

// decomposeMCPEntry is the MCP server configuration for the decompose binary.
var decomposeMCPEntry = json.RawMessage(`{
  "type": "stdio",
  "command": "decompose",
  "args": ["--serve-mcp"]
}`)

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

	// --- Create/merge .mcp.json ---

	if err := mergeMCPConfig(mcpPath, force); err != nil {
		return err
	}

	fmt.Println("\nSetup complete. The /decompose skill and MCP server are ready.")
	return nil
}

// mergeMCPConfig creates or merges the decompose entry into .mcp.json.
func mergeMCPConfig(mcpPath string, force bool) error {
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

	cfg.MCPServers["decompose"] = decomposeMCPEntry

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

// dotRelative returns a display path relative to the project root, prefixed
// with "./".
func dotRelative(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return "./" + rel
}
