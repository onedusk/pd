package review

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// directoryTreeRe locates the directory tree section in Stage 3 markdown.
// Reuses the same pattern from orchestrator/verification.go.
var directoryTreeRe = regexp.MustCompile(`(?i)directory\s+tree|target\s+directory|file\s+tree`)

// fileActionMilestoneRe matches ACTION (Mn[, Mn...]) annotations.
var fileActionMilestoneRe = regexp.MustCompile(`(CREATE|MODIFY|DELETE)\s*\(([^)]+)\)`)

// treeCharsRe matches box-drawing characters used in directory trees.
var treeCharsRe = regexp.MustCompile(`[├└│─►┌┐┘┬┤┴┼╴╵╶╷]+`)

// fileExtRe matches lines that contain a filename with an extension.
var fileExtRe = regexp.MustCompile(`(\S+\.\w{1,10})`)

// dirTrailingSlashRe detects directory entries (ending with /).
var dirTrailingSlashRe = regexp.MustCompile(`^\s*(\S+)/\s*$`)

// ParseDirectoryTree extracts FileEntry records from Stage 3 markdown content.
// It handles both tree-drawing character format and plain indentation format.
func ParseDirectoryTree(stage3Content string) ([]FileEntry, error) {
	// Find the directory tree section.
	loc := directoryTreeRe.FindStringIndex(stage3Content)
	if loc == nil {
		return nil, fmt.Errorf("no directory tree section found in Stage 3 content")
	}

	// Extract content from the section start to the next major heading or end.
	section := stage3Content[loc[0]:]

	// Look for a code fence block within the section.
	lines := strings.Split(section, "\n")
	var treeLines []string
	inFence := false
	fenceFound := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				break // end of code fence
			}
			inFence = true
			fenceFound = true
			continue
		}
		if inFence {
			treeLines = append(treeLines, line)
		}
	}

	// If no code fence, extract lines until next heading or section separator.
	if !fenceFound {
		pastHeader := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !pastHeader {
				// Skip the heading line itself.
				if directoryTreeRe.MatchString(line) {
					pastHeader = true
				}
				continue
			}
			// Stop at next heading or horizontal rule.
			if strings.HasPrefix(trimmed, "#") || trimmed == "---" {
				break
			}
			treeLines = append(treeLines, line)
		}
	}

	if len(treeLines) == 0 {
		return nil, fmt.Errorf("directory tree section is empty")
	}

	return parseTreeLines(treeLines)
}

// parseTreeLines processes the raw tree lines into FileEntry records.
func parseTreeLines(lines []string) ([]FileEntry, error) {
	entries := make(map[string]*FileEntry) // path -> entry
	var orderedPaths []string

	// dirStack tracks the current directory path at each indentation level.
	type dirLevel struct {
		depth int
		name  string
	}
	var dirStack []dirLevel

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Strip tree-drawing characters and normalize to spaces.
		normalized := treeCharsRe.ReplaceAllString(line, " ")

		// Calculate indentation depth (number of leading spaces after normalization).
		stripped := strings.TrimLeft(normalized, " ")
		if stripped == "" {
			continue
		}
		depth := len(normalized) - len(stripped)

		// Skip lines that are just comments or totals.
		lower := strings.ToLower(stripped)
		if strings.HasPrefix(lower, "total") || strings.HasPrefix(lower, "**total") {
			continue
		}

		// Check if this line has file action annotations.
		actions := fileActionMilestoneRe.FindAllStringSubmatch(stripped, -1)

		// Extract the name (first non-space token before any action annotation).
		name := stripped
		if idx := fileActionMilestoneRe.FindStringIndex(stripped); idx != nil {
			name = strings.TrimSpace(stripped[:idx[0]])
		}
		// Remove backticks if present.
		name = strings.Trim(name, "`")
		name = strings.TrimSpace(name)

		if name == "" {
			continue
		}

		// Pop directory stack to current depth.
		for len(dirStack) > 0 && dirStack[len(dirStack)-1].depth >= depth {
			dirStack = dirStack[:len(dirStack)-1]
		}

		// Determine if this is a directory or file.
		isDir := false
		if strings.HasSuffix(name, "/") {
			isDir = true
			name = strings.TrimSuffix(name, "/")
		} else if len(actions) == 0 && !fileExtRe.MatchString(name) {
			// No action annotations and no file extension — treat as directory.
			isDir = true
		}

		if isDir {
			dirStack = append(dirStack, dirLevel{depth: depth, name: name})
			continue
		}

		// Build full path from directory stack.
		var parts []string
		for _, d := range dirStack {
			parts = append(parts, d.name)
		}
		parts = append(parts, name)
		fullPath := strings.Join(parts, "/")

		// Parse action/milestone annotations.
		entry, exists := entries[fullPath]
		if !exists {
			entry = &FileEntry{
				Path:    fullPath,
				Actions: make(map[string]string),
			}
			entries[fullPath] = entry
			orderedPaths = append(orderedPaths, fullPath)
		}

		for _, match := range actions {
			action := match[1]                                          // CREATE, MODIFY, DELETE
			milestones := strings.Split(strings.TrimSpace(match[2]), ",") // "M1, M7"
			for _, ms := range milestones {
				ms = strings.TrimSpace(ms)
				if ms == "" {
					continue
				}
				entry.Actions[ms] = action
				// Track milestone order.
				found := false
				for _, existing := range entry.Milestones {
					if existing == ms {
						found = true
						break
					}
				}
				if !found {
					entry.Milestones = append(entry.Milestones, ms)
				}
			}
		}
	}

	// Build result in order of appearance.
	result := make([]FileEntry, 0, len(orderedPaths))
	for _, p := range orderedPaths {
		result = append(result, *entries[p])
	}

	return result, nil
}

// LoadAndParseStage3 reads the Stage 3 file from the decomposition directory
// and parses its directory tree.
func LoadAndParseStage3(decompDir string) ([]FileEntry, string, error) {
	stage3Path := filepath.Join(decompDir, "stage-3-task-index.md")
	content, err := os.ReadFile(stage3Path)
	if err != nil {
		return nil, "", fmt.Errorf("read stage 3 file: %w", err)
	}
	entries, err := ParseDirectoryTree(string(content))
	if err != nil {
		return nil, "", err
	}
	return entries, string(content), nil
}
