package review

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Task spec parsing regexes.
var (
	// taskHeadingRe matches task headings like "**T-01.03 — Title**" or "**T-01.03 -- Title**".
	taskHeadingRe = regexp.MustCompile(`\*\*T-(\d+)\.(\d+)\s*[—–-]+\s*(.+?)\*\*`)

	// taskFileRe matches the File line: **File:** `path` (ACTION) or **File:** `path` (ACTION), `path2` (ACTION)
	taskFileRe = regexp.MustCompile("(?i)\\*\\*File:\\*\\*\\s*`([^`]+)`\\s*\\((CREATE|MODIFY|DELETE)\\)")

	// taskDependsRe matches the Depends on line.
	taskDependsRe = regexp.MustCompile(`(?i)\*\*Depends?\s+on:\*\*\s*(.+)`)

	// taskIDExtractRe extracts task IDs from a depends-on value.
	taskIDExtractRe = regexp.MustCompile(`T-\d+\.\d+`)

	// outlineStartRe matches the start of an Outline section.
	outlineStartRe = regexp.MustCompile(`(?i)\*\*Outline:\*\*`)

	// acceptanceStartRe matches the start of an Acceptance section.
	acceptanceStartRe = regexp.MustCompile(`(?i)\*\*Acceptance:?\*\*`)

	// backtickSymbolRe extracts backtick-delimited identifiers.
	backtickSymbolRe = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*(?:\\.[A-Za-z_][A-Za-z0-9_]*)*)`")

	// pascalCaseRe matches PascalCase identifiers (3+ chars, not common prose words).
	pascalCaseRe = regexp.MustCompile(`\b([A-Z][a-z]+(?:[A-Z][a-z]+)+)\b`)
)

// commonProseWords are PascalCase words that appear in natural English prose
// and should not be treated as symbol references.
var commonProseWords = map[string]bool{
	"None": true, "True": true, "False": true, "This": true,
	"That": true, "Each": true, "Every": true, "Some": true,
	"Also": true, "Both": true, "After": true, "Before": true,
	"Because": true, "Since": true, "Until": true, "While": true,
	"Where": true, "Which": true, "Other": true, "Another": true,
}

// ParseTaskSpecs extracts TaskEntry records from one or more Stage 4 task files.
// taskFiles maps milestone identifier (e.g., "M1") to file content.
func ParseTaskSpecs(taskFiles map[string]string) ([]TaskEntry, error) {
	var allTasks []TaskEntry

	for milestone, content := range taskFiles {
		tasks, err := parseOneTaskFile(milestone, content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", milestone, err)
		}
		allTasks = append(allTasks, tasks...)
	}

	return allTasks, nil
}

// parseOneTaskFile parses a single tasks_mNN.md file.
func parseOneTaskFile(milestone, content string) ([]TaskEntry, error) {
	// Split content into task sections by finding task headings.
	headingLocs := taskHeadingRe.FindAllStringSubmatchIndex(content, -1)
	if len(headingLocs) == 0 {
		return nil, nil // No tasks found — not an error, file might be a placeholder.
	}

	var tasks []TaskEntry

	for i, loc := range headingLocs {
		// Extract the task section.
		sectionStart := loc[0]
		sectionEnd := len(content)
		if i+1 < len(headingLocs) {
			sectionEnd = headingLocs[i+1][0]
		}
		section := content[sectionStart:sectionEnd]

		// Extract task ID.
		mm := content[loc[2]:loc[3]]
		ss := content[loc[4]:loc[5]]
		taskID := fmt.Sprintf("T-%s.%s", mm, ss)

		// Extract file path and action.
		filePath := ""
		action := ""
		if m := taskFileRe.FindStringSubmatch(section); m != nil {
			filePath = m[1]
			action = strings.ToUpper(m[2])
		}

		// Extract depends-on.
		var dependsOn []string
		if m := taskDependsRe.FindStringSubmatch(section); m != nil {
			depValue := strings.TrimSpace(m[1])
			if !strings.EqualFold(depValue, "none") && depValue != "—" && depValue != "-" {
				ids := taskIDExtractRe.FindAllString(depValue, -1)
				dependsOn = append(dependsOn, ids...)
			}
		}

		// Extract outline text.
		outline := extractOutline(section)

		// Extract symbol references from outline.
		symbolRefs := ExtractSymbolRefs(outline)

		tasks = append(tasks, TaskEntry{
			ID:         taskID,
			Milestone:  milestone,
			File:       filePath,
			Action:     action,
			DependsOn:  dependsOn,
			SymbolRefs: symbolRefs,
			Outline:    outline,
		})
	}

	return tasks, nil
}

// extractOutline extracts the outline text from a task section.
func extractOutline(section string) string {
	lines := strings.Split(section, "\n")
	var outlineLines []string
	inOutline := false

	for _, line := range lines {
		if outlineStartRe.MatchString(line) {
			inOutline = true
			// Check if there's content after the marker on the same line.
			idx := outlineStartRe.FindStringIndex(line)
			rest := strings.TrimSpace(line[idx[1]:])
			if rest != "" {
				outlineLines = append(outlineLines, rest)
			}
			continue
		}
		if inOutline {
			// Stop at Acceptance section or next task field.
			if acceptanceStartRe.MatchString(line) {
				break
			}
			if strings.HasPrefix(strings.TrimSpace(line), "**") && !outlineStartRe.MatchString(line) {
				break
			}
			outlineLines = append(outlineLines, line)
		}
	}

	return strings.Join(outlineLines, "\n")
}

// ExtractSymbolRefs extracts likely symbol names from a task outline.
// Looks for backtick-delimited identifiers and PascalCase names.
func ExtractSymbolRefs(outline string) []string {
	seen := make(map[string]bool)
	var refs []string

	// Extract backtick-delimited identifiers.
	for _, m := range backtickSymbolRe.FindAllStringSubmatch(outline, -1) {
		name := m[1]
		// Skip short names (likely variable names, not significant symbols).
		if len(name) < 3 {
			continue
		}
		// Skip common keywords.
		if isKeyword(name) {
			continue
		}
		if !seen[name] {
			seen[name] = true
			refs = append(refs, name)
		}
	}

	// Extract PascalCase identifiers.
	for _, m := range pascalCaseRe.FindAllStringSubmatch(outline, -1) {
		name := m[1]
		if commonProseWords[name] {
			continue
		}
		if !seen[name] {
			seen[name] = true
			refs = append(refs, name)
		}
	}

	return refs
}

// isKeyword returns true if the name is a common language keyword.
func isKeyword(name string) bool {
	keywords := map[string]bool{
		"func": true, "function": true, "class": true, "type": true,
		"struct": true, "interface": true, "enum": true, "const": true,
		"var": true, "let": true, "import": true, "export": true,
		"return": true, "error": true, "nil": true, "null": true,
		"true": true, "false": true, "string": true, "int": true,
		"bool": true, "byte": true, "float": true, "float64": true,
		"package": true, "module": true, "def": true, "self": true,
		"pub": true, "mod": true, "use": true, "crate": true,
	}
	return keywords[strings.ToLower(name)]
}

// LoadAndParseStage4 reads all Stage 4 task files from the decomposition directory.
func LoadAndParseStage4(decompDir string) ([]TaskEntry, error) {
	pattern := filepath.Join(decompDir, "tasks_m*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob task files: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no Stage 4 task files found at %s", pattern)
	}

	taskFiles := make(map[string]string)
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read task file %s: %w", path, err)
		}
		// Extract milestone number from filename: tasks_m01.md -> M1
		base := filepath.Base(path)
		milestone := milestoneFromFilename(base)
		taskFiles[milestone] = string(content)
	}

	return ParseTaskSpecs(taskFiles)
}

// milestoneFromFilename extracts milestone ID from a task filename.
// "tasks_m01.md" -> "M1", "tasks_m12.md" -> "M12"
func milestoneFromFilename(filename string) string {
	re := regexp.MustCompile(`tasks_m(\d+)\.md`)
	m := re.FindStringSubmatch(filename)
	if m == nil {
		return filename
	}
	// Strip leading zeros.
	num := strings.TrimLeft(m[1], "0")
	if num == "" {
		num = "0"
	}
	return "M" + num
}
