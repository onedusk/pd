package status

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dusk-indust/decompose/internal/orchestrator"
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
		s := orchestrator.Stage(stage)
		filename := fmt.Sprintf("stage-%d-%s.md", stage, s.String())
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err == nil {
			completed = append(completed, stage)
		}
	}
	return completed
}

// NextStage returns the next stage to run based on completed stages.
// Returns -1 if all stages are complete.
func NextStage(completed []int) int {
	if len(completed) == 0 {
		return 0
	}
	max := completed[0]
	for _, s := range completed[1:] {
		if s > max {
			max = s
		}
	}
	next := max + 1
	if next > 4 {
		return -1
	}
	return next
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
				filePath = filepath.Join(outputDir, fmt.Sprintf("stage-%d-%s.md", i, slug))
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
