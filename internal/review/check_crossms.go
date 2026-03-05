package review

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/onedusk/pd/internal/orchestrator"
)

// CheckCrossMilestoneConsistency verifies that files touched by multiple milestones
// have consistent assumptions across those milestones. Uses programmatic heuristics
// only — semantic conflicts are deferred to the A2A interpretive pass.
func CheckCrossMilestoneConsistency(_ context.Context, cfg ReviewConfig, entries []FileEntry, tasks []TaskEntry, stage3Content string) []ReviewFinding {
	var findings []ReviewFinding
	counter := 0

	// Build milestone ordering from Stage 3.
	milestoneOrder := buildMilestoneOrder(stage3Content)

	// Check 4a: Find files with multiple milestones and check for conflicting actions.
	for _, entry := range entries {
		if len(entry.Milestones) < 2 {
			continue
		}

		findings = append(findings, checkConflictingActions(entry, milestoneOrder, &counter)...)
	}

	// Check 4b: Find overlapping symbol modifications across tasks targeting the same file.
	findings = append(findings, checkOverlappingSymbols(tasks, &counter)...)

	return findings
}

// buildMilestoneOrder returns a map of milestone ID to its topological order.
// Lower numbers come first in the dependency chain.
func buildMilestoneOrder(stage3Content string) map[string]int {
	order := make(map[string]int)

	milestones, err := orchestrator.ParseMilestones(stage3Content)
	if err != nil || len(milestones) == 0 {
		return order
	}

	// Use the scheduler to get topological ordering.
	scheduler, err := orchestrator.NewScheduler(milestones)
	if err != nil {
		// Fallback: use milestone IDs as natural order (M1 < M2 < ...).
		for i, m := range milestones {
			order[m.ID] = i
		}
		return order
	}

	// Walk the scheduler to get execution order.
	idx := 0
	for {
		ready := scheduler.Ready()
		if len(ready) == 0 {
			break
		}
		for _, m := range ready {
			order[m.ID] = idx
			idx++
			scheduler.MarkRunning(m.ID)
			scheduler.MarkCompleted(m.ID)
		}
	}

	return order
}

// checkConflictingActions detects action conflicts on a multi-milestone file.
func checkConflictingActions(entry FileEntry, milestoneOrder map[string]int, counter *int) []ReviewFinding {
	var findings []ReviewFinding

	// Sort milestones by execution order.
	type msAction struct {
		milestone string
		action    string
		order     int
	}
	var sorted []msAction
	for ms, action := range entry.Actions {
		o, ok := milestoneOrder[ms]
		if !ok {
			o = 999 // Unknown order — put at end.
		}
		sorted = append(sorted, msAction{milestone: ms, action: action, order: o})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].order < sorted[j].order })

	// Check for CREATE + DELETE conflict.
	hasCreate := false
	hasDelete := false
	for _, ma := range sorted {
		if ma.action == "CREATE" {
			hasCreate = true
		}
		if ma.action == "DELETE" {
			hasDelete = true
		}
	}
	if hasCreate && hasDelete {
		*counter++
		findings = append(findings, ReviewFinding{
			ID:             fmt.Sprintf("R-4.%02d", *counter),
			Check:          4,
			Classification: ClassMismatch,
			FilePath:       entry.Path,
			Description:    "File has both CREATE and DELETE across milestones",
			Suggestion:     "Verify intent — if the file is temporary, document why; otherwise resolve the conflict",
		})
	}

	// Check for MODIFY before CREATE in execution order.
	firstCreate := -1
	for i, ma := range sorted {
		if ma.action == "CREATE" {
			firstCreate = i
			break
		}
	}

	if firstCreate > 0 {
		// There are actions before the first CREATE.
		for i := 0; i < firstCreate; i++ {
			if sorted[i].action == "MODIFY" {
				*counter++
				findings = append(findings, ReviewFinding{
					ID:             fmt.Sprintf("R-4.%02d", *counter),
					Check:          4,
					Classification: ClassMismatch,
					FilePath:       entry.Path,
					Milestone:      sorted[i].milestone,
					Description:    fmt.Sprintf("Milestone %s MODIFYs this file before %s CREATEs it", sorted[i].milestone, sorted[firstCreate].milestone),
					Suggestion:     "Reorder milestones or change the earlier action to CREATE",
				})
			}
		}
	}

	return findings
}

// checkOverlappingSymbols detects when multiple MODIFY tasks reference the same
// symbol in the same file across different milestones.
func checkOverlappingSymbols(tasks []TaskEntry, counter *int) []ReviewFinding {
	var findings []ReviewFinding

	// Group MODIFY tasks by file.
	byFile := make(map[string][]TaskEntry)
	for _, task := range tasks {
		if task.Action == "MODIFY" && task.File != "" {
			byFile[task.File] = append(byFile[task.File], task)
		}
	}

	for file, fileTasks := range byFile {
		if len(fileTasks) < 2 {
			continue
		}

		// Check for overlapping symbol references across tasks in different milestones.
		type symbolSource struct {
			taskID    string
			milestone string
		}
		symbolMap := make(map[string][]symbolSource)

		for _, task := range fileTasks {
			for _, sym := range task.SymbolRefs {
				symbolMap[sym] = append(symbolMap[sym], symbolSource{
					taskID:    task.ID,
					milestone: task.Milestone,
				})
			}
		}

		for sym, sources := range symbolMap {
			if len(sources) < 2 {
				continue
			}

			// Check if sources span different milestones.
			milestones := make(map[string]bool)
			for _, s := range sources {
				milestones[s.milestone] = true
			}
			if len(milestones) < 2 {
				continue
			}

			var taskIDs []string
			var msList []string
			for _, s := range sources {
				taskIDs = append(taskIDs, s.taskID)
			}
			for ms := range milestones {
				msList = append(msList, ms)
			}
			sort.Strings(msList)

			*counter++
			findings = append(findings, ReviewFinding{
				ID:             fmt.Sprintf("R-4.%02d", *counter),
				Check:          4,
				Classification: ClassOmission,
				FilePath:       file,
				Description:    fmt.Sprintf("Symbol `%s` modified by tasks %s across milestones %s", sym, strings.Join(taskIDs, ", "), strings.Join(msList, ", ")),
				Suggestion:     "Add a dependency edge between these tasks or merge them to avoid conflicts",
			})
		}
	}

	return findings
}
