package export

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dusk-indust/decompose/internal/status"
)

// DecompositionExport is the top-level JSON export structure.
type DecompositionExport struct {
	Name       string        `json:"name"`
	ExportedAt string        `json:"exportedAt"`
	Stages     []StageExport `json:"stages"`
	Tasks      []TaskExport  `json:"tasks,omitempty"`
}

// StageExport describes one pipeline stage.
type StageExport struct {
	Stage    int    `json:"stage"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	FilePath string `json:"filePath,omitempty"`
}

// TaskExport describes a single task from Stage 4.
type TaskExport struct {
	ID           string   `json:"id"`
	Milestone    string   `json:"milestone"`
	Title        string   `json:"title"`
	FileActions  []string `json:"fileActions,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Acceptance   []string `json:"acceptanceCriteria,omitempty"`
}

// ExportDecomposition builds a DecompositionExport from the filesystem.
func ExportDecomposition(projectRoot, name string) (*DecompositionExport, error) {
	ds := status.GetDecompositionStatus(projectRoot, name)

	export := &DecompositionExport{
		Name:       name,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for _, si := range ds.Stages {
		s := "pending"
		if si.Complete {
			s = "complete"
		}
		export.Stages = append(export.Stages, StageExport{
			Stage:    si.Stage,
			Name:     si.Name,
			Status:   s,
			FilePath: si.FilePath,
		})
	}

	// Parse task files from Stage 4 output.
	outputDir := filepath.Join(projectRoot, "docs", "decompose", name)
	taskFiles, _ := filepath.Glob(filepath.Join(outputDir, "tasks_m*.md"))
	for _, tf := range taskFiles {
		tasks, err := parseTaskFile(tf)
		if err != nil {
			continue
		}
		export.Tasks = append(export.Tasks, tasks...)
	}

	return export, nil
}

var (
	// Matches: "- [ ] **T-01.01 — Title**" or "### T-01.01 — Title" or "### T-01.01: Title"
	taskIDRegex     = regexp.MustCompile(`(?:^-\s+\[[ x]\]\s+\*\*|^###?\s+)(T-\d+\.\d+)\s*[:\-–—]+\s*(.+?)(?:\*\*)?$`)
	fileActionRegex = regexp.MustCompile(`(?i)^\s*-\s+\*\*(?:File:|)(CREATE|MODIFY|DELETE)\*\*\s+` + "`" + `(.+?)` + "`")
	filePlainRegex  = regexp.MustCompile(`^\s*-\s+\*\*File:\*\*\s+` + "`" + `(.+?)` + "`" + `\s+\((CREATE|MODIFY|DELETE)\)`)
	depRegex        = regexp.MustCompile(`(?i)^\s*-\s+\*\*Depends on:\*\*\s*(.+)`)
	depIDRegex      = regexp.MustCompile(`T-\d+\.\d+`)
	acceptRegex     = regexp.MustCompile(`^\s*-\s+\[[ x]\]\s+(.+)`)
	acceptPlain     = regexp.MustCompile(`(?i)^\s*-\s+\*\*Acceptance:?\*\*\s*(.+)`)
)

func parseTaskFile(path string) ([]TaskExport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Extract milestone from filename (tasks_m01.md → M01).
	base := filepath.Base(path)
	milestone := strings.TrimSuffix(strings.TrimPrefix(base, "tasks_"), ".md")

	var tasks []TaskExport
	var current *TaskExport
	inAcceptance := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// New task header.
		if m := taskIDRegex.FindStringSubmatch(line); m != nil {
			if current != nil {
				tasks = append(tasks, *current)
			}
			current = &TaskExport{
				ID:        m[1],
				Milestone: milestone,
				Title:     strings.TrimSpace(m[2]),
			}
			inAcceptance = false
			continue
		}

		if current == nil {
			continue
		}

		// File actions: "**CREATE** `path`" or "**File:** `path` (CREATE)"
		if m := fileActionRegex.FindStringSubmatch(line); m != nil {
			current.FileActions = append(current.FileActions, fmt.Sprintf("%s %s", strings.ToUpper(m[1]), m[2]))
			continue
		}
		if m := filePlainRegex.FindStringSubmatch(line); m != nil {
			current.FileActions = append(current.FileActions, fmt.Sprintf("%s %s", strings.ToUpper(m[2]), m[1]))
			continue
		}

		// Dependencies: "**Depends on:** T-01.01, T-01.02" or "- Depends on: ..."
		if m := depRegex.FindStringSubmatch(line); m != nil {
			ids := depIDRegex.FindAllString(m[1], -1)
			current.Dependencies = append(current.Dependencies, ids...)
			continue
		}

		// Inline acceptance: "**Acceptance:** criteria text"
		if m := acceptPlain.FindStringSubmatch(line); m != nil {
			current.Acceptance = append(current.Acceptance, strings.TrimSpace(m[1]))
			continue
		}

		// Acceptance criteria section header.
		lower := strings.ToLower(line)
		if strings.Contains(lower, "acceptance") && strings.Contains(lower, "criteria") {
			inAcceptance = true
			continue
		}

		// Acceptance criteria items (checklist form).
		if inAcceptance {
			if m := acceptRegex.FindStringSubmatch(line); m != nil {
				current.Acceptance = append(current.Acceptance, strings.TrimSpace(m[1]))
			} else if strings.TrimSpace(line) == "" {
				// blank line is ok within acceptance
			} else if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
				inAcceptance = false
			}
		}
	}

	if current != nil {
		tasks = append(tasks, *current)
	}

	return tasks, scanner.Err()
}
