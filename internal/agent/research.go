package agent

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dusk-indust/decompose/internal/a2a"
)

// skipDirs is the set of directory names to skip when walking a project tree.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"target":       true,
}

// knownConfigFiles lists config file names that indicate platform/tooling choices.
var knownConfigFiles = []string{
	"go.mod",
	"go.sum",
	"package.json",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"Cargo.toml",
	"Cargo.lock",
	"pyproject.toml",
	"requirements.txt",
	"setup.py",
	"setup.cfg",
	"Makefile",
	"Dockerfile",
	"docker-compose.yml",
	"docker-compose.yaml",
	".dockerignore",
	".gitignore",
	"tsconfig.json",
	"vite.config.ts",
	"webpack.config.js",
	"babel.config.js",
	".eslintrc.json",
	".prettierrc",
	"Gemfile",
	"CMakeLists.txt",
	"build.gradle",
	"pom.xml",
	"flake.nix",
	"shell.nix",
}

// extToLanguage maps file extensions to language names.
var extToLanguage = map[string]string{
	".go":    "Go",
	".js":    "JavaScript",
	".jsx":   "JavaScript (JSX)",
	".ts":    "TypeScript",
	".tsx":   "TypeScript (TSX)",
	".py":    "Python",
	".rs":    "Rust",
	".java":  "Java",
	".kt":    "Kotlin",
	".rb":    "Ruby",
	".c":     "C",
	".h":     "C/C++ Header",
	".cpp":   "C++",
	".cc":    "C++",
	".cs":    "C#",
	".swift": "Swift",
	".md":    "Markdown",
	".json":  "JSON",
	".yaml":  "YAML",
	".yml":   "YAML",
	".toml":  "TOML",
	".html":  "HTML",
	".css":   "CSS",
	".scss":  "SCSS",
	".sql":   "SQL",
	".sh":    "Shell",
	".bash":  "Shell",
	".zsh":   "Shell",
	".proto": "Protocol Buffers",
	".nix":   "Nix",
}

// ResearchAgent is a specialist agent that researches platforms, verifies
// versions, and explores codebases. It embeds BaseAgent for A2A protocol
// handling.
type ResearchAgent struct {
	*BaseAgent
}

// NewResearchAgent creates a new ResearchAgent with its agent card and
// process function wired up.
func NewResearchAgent() *ResearchAgent {
	ra := &ResearchAgent{}

	card := a2a.AgentCard{
		Name:        "research-agent",
		Description: "Researches platforms, verifies versions, and explores codebases",
		Version:     "dev",
		Skills: []a2a.AgentSkill{
			{
				ID:          "research-platform",
				Name:        "Research Platform",
				Description: "Read project config files to identify dependencies and produce a platform baseline",
				Tags:        []string{"research", "platform", "dependencies"},
			},
			{
				ID:          "verify-versions",
				Name:        "Verify Versions",
				Description: "Verify dependency versions against latest releases",
				Tags:        []string{"research", "versions"},
			},
			{
				ID:          "explore-codebase",
				Name:        "Explore Codebase",
				Description: "Walk a project directory and produce a structural summary",
				Tags:        []string{"research", "codebase", "structure"},
			},
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/markdown"},
	}

	ra.BaseAgent = NewBaseAgent(card, ra.processMessage)
	return ra
}

// processMessage is the ProcessFunc that dispatches to the appropriate skill
// based on the message text content.
func (ra *ResearchAgent) processMessage(ctx context.Context, task *a2a.Task, msg a2a.Message) ([]a2a.Artifact, error) {
	text := extractText(msg)

	switch {
	case strings.Contains(text, "explore-codebase"):
		return ra.exploreCodebase(ctx, text)
	case strings.Contains(text, "research-platform"):
		return ra.researchPlatform(ctx, text)
	case strings.Contains(text, "verify-versions"):
		return ra.verifyVersions(ctx, text)
	default:
		return nil, fmt.Errorf("unknown skill: message does not contain a recognized skill ID (explore-codebase, research-platform, verify-versions)")
	}
}

// extractText concatenates all text parts from a message.
func extractText(msg a2a.Message) string {
	var parts []string
	for _, p := range msg.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// extractPath attempts to find an absolute path in the message text. It looks
// for the first line that starts with "/" or, failing that, returns the first
// non-empty line that is not the skill ID itself.
func extractPath(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "/") {
			// Strip any trailing skill ID or extra text after the path.
			// A simple heuristic: take the first whitespace-delimited token
			// that looks like a path.
			fields := strings.Fields(trimmed)
			for _, f := range fields {
				if strings.HasPrefix(f, "/") {
					return f
				}
			}
		}
	}
	// Fallback: try the first non-empty line.
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip lines that are just the skill ID.
		if trimmed == "explore-codebase" || trimmed == "research-platform" || trimmed == "verify-versions" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return "."
}

// exploreCodebase walks the project directory and produces a markdown summary.
func (ra *ResearchAgent) exploreCodebase(_ context.Context, text string) ([]a2a.Artifact, error) {
	root := extractPath(text)

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("explore-codebase: cannot access path %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("explore-codebase: path %q is not a directory", root)
	}

	type dirEntry struct {
		relPath string
		isDir   bool
	}

	var entries []dirEntry
	fileCounts := make(map[string]int) // language -> count
	var configFiles []string

	knownConfigSet := make(map[string]bool, len(knownConfigFiles))
	for _, cf := range knownConfigFiles {
		knownConfigSet[cf] = true
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we cannot read
		}

		name := d.Name()

		// Skip hidden directories (except root) and known noisy directories.
		if d.IsDir() && path != root {
			if skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		if rel == "." {
			return nil
		}

		entries = append(entries, dirEntry{relPath: rel, isDir: d.IsDir()})

		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(name))
			if lang, ok := extToLanguage[ext]; ok {
				fileCounts[lang]++
			} else if ext != "" {
				fileCounts[ext]++
			}

			if knownConfigSet[name] {
				configFiles = append(configFiles, rel)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("explore-codebase: walk error: %w", err)
	}

	// Build directory tree.
	var tree strings.Builder
	tree.WriteString("## Directory Tree\n\n```\n")
	tree.WriteString(filepath.Base(root) + "/\n")
	for _, e := range entries {
		depth := strings.Count(e.relPath, string(filepath.Separator))
		indent := strings.Repeat("  ", depth)
		name := filepath.Base(e.relPath)
		if e.isDir {
			tree.WriteString(indent + name + "/\n")
		} else {
			tree.WriteString(indent + name + "\n")
		}
	}
	tree.WriteString("```\n")

	// Build file counts.
	var counts strings.Builder
	counts.WriteString("## File Count by Language\n\n")
	type langCount struct {
		lang  string
		count int
	}
	var lcs []langCount
	for lang, count := range fileCounts {
		lcs = append(lcs, langCount{lang, count})
	}
	sort.Slice(lcs, func(i, j int) bool {
		if lcs[i].count != lcs[j].count {
			return lcs[i].count > lcs[j].count
		}
		return lcs[i].lang < lcs[j].lang
	})
	for _, lc := range lcs {
		counts.WriteString(fmt.Sprintf("- **%s**: %d\n", lc.lang, lc.count))
	}

	// Build config file list.
	var configs strings.Builder
	configs.WriteString("## Detected Config Files\n\n")
	if len(configFiles) == 0 {
		configs.WriteString("_No known config files detected._\n")
	} else {
		sort.Strings(configFiles)
		for _, cf := range configFiles {
			configs.WriteString(fmt.Sprintf("- `%s`\n", cf))
		}
	}

	// Combine into markdown.
	var md strings.Builder
	md.WriteString(fmt.Sprintf("# Codebase Exploration: %s\n\n", root))
	md.WriteString(tree.String())
	md.WriteString("\n")
	md.WriteString(counts.String())
	md.WriteString("\n")
	md.WriteString(configs.String())

	artifact := a2a.Artifact{
		ArtifactID:  a2a.NewTaskID(),
		Name:        "codebase-exploration",
		Description: fmt.Sprintf("Structural summary of %s", root),
		Parts:       []a2a.Part{a2a.TextPart(md.String())},
	}

	return []a2a.Artifact{artifact}, nil
}

// researchPlatform reads project config files and produces a platform baseline.
func (ra *ResearchAgent) researchPlatform(_ context.Context, text string) ([]a2a.Artifact, error) {
	root := extractPath(text)

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("research-platform: cannot access path %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("research-platform: path %q is not a directory", root)
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# Platform & Tooling Baseline: %s\n\n", root))

	found := false

	// Go: go.mod
	goModPath := filepath.Join(root, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		found = true
		md.WriteString("## Go (go.mod)\n\n")
		md.WriteString(parseGoMod(string(data)))
		md.WriteString("\n")
	}

	// Node.js: package.json
	pkgJSONPath := filepath.Join(root, "package.json")
	if data, err := os.ReadFile(pkgJSONPath); err == nil {
		found = true
		md.WriteString("## Node.js (package.json)\n\n")
		md.WriteString("```json\n")
		md.WriteString(string(data))
		md.WriteString("\n```\n\n")
	}

	// Rust: Cargo.toml
	cargoPath := filepath.Join(root, "Cargo.toml")
	if data, err := os.ReadFile(cargoPath); err == nil {
		found = true
		md.WriteString("## Rust (Cargo.toml)\n\n")
		md.WriteString("```toml\n")
		md.WriteString(string(data))
		md.WriteString("\n```\n\n")
	}

	// Python: pyproject.toml
	pyprojectPath := filepath.Join(root, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectPath); err == nil {
		found = true
		md.WriteString("## Python (pyproject.toml)\n\n")
		md.WriteString("```toml\n")
		md.WriteString(string(data))
		md.WriteString("\n```\n\n")
	}

	// Python: requirements.txt
	reqPath := filepath.Join(root, "requirements.txt")
	if data, err := os.ReadFile(reqPath); err == nil {
		found = true
		md.WriteString("## Python (requirements.txt)\n\n")
		md.WriteString("```\n")
		md.WriteString(string(data))
		md.WriteString("\n```\n\n")
	}

	if !found {
		md.WriteString("_No recognized project config files found at the root level._\n")
	}

	artifact := a2a.Artifact{
		ArtifactID:  a2a.NewTaskID(),
		Name:        "platform-baseline",
		Description: fmt.Sprintf("Platform & tooling baseline for %s", root),
		Parts:       []a2a.Part{a2a.TextPart(md.String())},
	}

	return []a2a.Artifact{artifact}, nil
}

// parseGoMod extracts the module path, Go version, and dependencies from a
// go.mod file and formats them as markdown.
func parseGoMod(content string) string {
	var md strings.Builder

	lines := strings.Split(content, "\n")
	var deps []string
	inRequire := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "module ") {
			md.WriteString(fmt.Sprintf("**Module**: `%s`\n\n", strings.TrimPrefix(trimmed, "module ")))
			continue
		}

		if strings.HasPrefix(trimmed, "go ") {
			md.WriteString(fmt.Sprintf("**Go Version**: `%s`\n\n", strings.TrimPrefix(trimmed, "go ")))
			continue
		}

		if trimmed == "require (" {
			inRequire = true
			continue
		}
		if trimmed == ")" {
			inRequire = false
			continue
		}

		if inRequire && trimmed != "" {
			// Strip inline comments.
			dep := trimmed
			if idx := strings.Index(dep, "//"); idx != -1 {
				dep = strings.TrimSpace(dep[:idx])
			}
			if dep != "" {
				deps = append(deps, dep)
			}
		}
	}

	if len(deps) > 0 {
		md.WriteString("**Dependencies**:\n\n")
		for _, dep := range deps {
			md.WriteString(fmt.Sprintf("- `%s`\n", dep))
		}
	}

	return md.String()
}

// verifyVersions is a stub that returns a fallback-mode notice.
func (ra *ResearchAgent) verifyVersions(_ context.Context, _ string) ([]a2a.Artifact, error) {
	md := "# Version Verification\n\n" +
		"**Note**: Web search is not available in fallback mode.\n\n" +
		"Version verification requires access to external package registries " +
		"(e.g., pkg.go.dev, npmjs.com, crates.io) which is not available without " +
		"MCP tool integration. This skill will produce full results once MCP " +
		"tools are configured.\n"

	artifact := a2a.Artifact{
		ArtifactID:  a2a.NewTaskID(),
		Name:        "version-verification",
		Description: "Version verification (fallback mode)",
		Parts:       []a2a.Part{a2a.TextPart(md)},
	}

	return []a2a.Artifact{artifact}, nil
}
