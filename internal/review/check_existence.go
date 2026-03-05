package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// CheckFileExistence compares the Stage 3 directory tree against the actual filesystem.
// For each planned file: CREATE should not exist, MODIFY should exist, DELETE should exist.
func CheckFileExistence(_ context.Context, cfg ReviewConfig, entries []FileEntry) []ReviewFinding {
	var findings []ReviewFinding
	counter := 0

	for _, entry := range entries {
		absPath := filepath.Join(cfg.ProjectRoot, entry.Path)
		_, err := os.Stat(absPath)
		exists := err == nil

		// Check each action on this file.
		for milestone, action := range entry.Actions {
			switch action {
			case "CREATE":
				if exists {
					counter++
					findings = append(findings, ReviewFinding{
						ID:             fmt.Sprintf("R-1.%02d", counter),
						Check:          1,
						Classification: ClassMismatch,
						FilePath:       entry.Path,
						Milestone:      milestone,
						Description:    "File already exists but plan specifies CREATE",
						Suggestion:     "Change action to MODIFY, or verify the plan intends to overwrite this file",
					})
				}
			case "MODIFY":
				if !exists {
					counter++
					findings = append(findings, ReviewFinding{
						ID:             fmt.Sprintf("R-1.%02d", counter),
						Check:          1,
						Classification: ClassMismatch,
						FilePath:       entry.Path,
						Milestone:      milestone,
						Description:    "File does not exist but plan specifies MODIFY",
						Suggestion:     "Change action to CREATE, or check that a dependency creates it first",
					})
				}
			case "DELETE":
				if !exists {
					counter++
					findings = append(findings, ReviewFinding{
						ID:             fmt.Sprintf("R-1.%02d", counter),
						Check:          1,
						Classification: ClassStale,
						FilePath:       entry.Path,
						Milestone:      milestone,
						Description:    "File does not exist but plan specifies DELETE",
						Suggestion:     "Remove this DELETE task — file was already removed or never created",
					})
				}
			}
		}
	}

	return findings
}
