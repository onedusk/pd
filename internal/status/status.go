package status

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/onedusk/pd/internal/orchestrator"
)

// StageInfo describes the completion state of a single stage.
type StageInfo struct {
	Stage    int
	Name     string // human-readable name (e.g. "Development Standards")
	Slug     string // file slug (e.g. "development-standards")
	Complete bool
	FilePath string // absolute path when complete, empty otherwise
}

// DecompositionStatus holds the status of one named decomposition.
type DecompositionStatus struct {
	Name      string
	Stages    []StageInfo
	NextStage int // -1 if all complete
}

var stageLabels = [5]string{
	"Development Standards",
	"Design Pack",
	"Implementation Skeletons",
	"Task Index",
	"Task Specifications",
}

// ScanCompletedStages checks which stage output files exist in a directory.
// Returns the stage numbers (0-4) that have output files.
func ScanCompletedStages(dir string) []int {
	var completed []int
	for stage := 0; stage <= 4; stage++ {
		if stageFilePath(dir, stage) != "" {
			completed = append(completed, stage)
		}
	}
	return completed
}

// stageFilePath returns the path of a stage's output file in dir, or "" if
// there is none. Stage 4 is either the single stage-4-task-specifications.md
// written by the pipeline or the per-milestone tasks_mNN.md files written by
// the /decompose skill; for the latter the first task file is returned.
func stageFilePath(dir string, stage int) string {
	path := filepath.Join(dir, fmt.Sprintf("stage-%d-%s.md", stage, orchestrator.Stage(stage).String()))
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if stage == int(orchestrator.StageTaskSpecifications) {
		if matches, _ := filepath.Glob(filepath.Join(dir, "tasks_m*.md")); len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

// NextStage returns the next stage to run based on completed stages: the
// earliest missing stage from 1 to 4. Stage 0 is shared across decompositions
// and optional, so it is only returned when nothing is complete.
// Returns -1 if all stages are complete.
func NextStage(completed []int) int {
	if len(completed) == 0 {
		return 0
	}
	done := make(map[int]bool, len(completed))
	for _, s := range completed {
		done[s] = true
	}
	for stage := 1; stage <= 4; stage++ {
		if !done[stage] {
			return stage
		}
	}
	return -1
}

// CheckExists returns an error if the named decomposition has no directory
// under docs/decompose.
func CheckExists(projectRoot, name string) error {
	dir := filepath.Join(projectRoot, "docs", "decompose", name)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("decomposition %q not found (expected %s)", name, dir)
	}
	return nil
}

// GetDecompositionStatus returns detailed status for a single decomposition.
func GetDecompositionStatus(projectRoot, name string) DecompositionStatus {
	outputDir := filepath.Join(projectRoot, "docs", "decompose", name)
	completedSet := make(map[int]bool)
	for _, s := range ScanCompletedStages(outputDir) {
		completedSet[s] = true
	}

	// Stage 0 is at root level.
	stage0Path := filepath.Join(projectRoot, "docs", "decompose",
		fmt.Sprintf("stage-0-%s.md", orchestrator.StageDevelopmentStandards.String()))
	if _, err := os.Stat(stage0Path); err == nil {
		completedSet[0] = true
	}

	stages := make([]StageInfo, 5)
	for i := 0; i < 5; i++ {
		slug := orchestrator.Stage(i).String()
		var filePath string
		if completedSet[i] {
			if i == 0 {
				filePath = stage0Path
			} else {
				filePath = stageFilePath(outputDir, i)
			}
		}
		stages[i] = StageInfo{
			Stage:    i,
			Name:     stageLabels[i],
			Slug:     slug,
			Complete: completedSet[i],
			FilePath: filePath,
		}
	}

	var completed []int
	for s := range completedSet {
		completed = append(completed, s)
	}

	return DecompositionStatus{
		Name:      name,
		Stages:    stages,
		NextStage: NextStage(completed),
	}
}

// ListDecompositions scans the docs/decompose directory for all decompositions.
func ListDecompositions(projectRoot string) ([]DecompositionStatus, bool) {
	decomposeDir := filepath.Join(projectRoot, "docs", "decompose")
	entries, err := os.ReadDir(decomposeDir)
	if err != nil {
		return nil, false
	}

	hasStage0 := false
	var results []DecompositionStatus

	for _, entry := range entries {
		if !entry.IsDir() {
			if strings.HasPrefix(entry.Name(), "stage-0-") {
				hasStage0 = true
			}
			continue
		}
		results = append(results, GetDecompositionStatus(projectRoot, entry.Name()))
	}

	return results, hasStage0
}
