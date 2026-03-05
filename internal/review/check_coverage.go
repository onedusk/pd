package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sourceExtensions are file extensions considered as source files for coverage scanning.
var sourceExtensions = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
	".py": true, ".rs": true, ".java": true, ".kt": true, ".swift": true,
	".yml": true, ".yaml": true, ".json": true, ".toml": true,
}

// CheckCoverageGaps scans the codebase for files semantically related to the
// decomposition scope but absent from the plan. Intentionally high recall /
// moderate precision — the interpretive session triages false positives.
func CheckCoverageGaps(ctx context.Context, cfg ReviewConfig, entries []FileEntry) []ReviewFinding {
	var findings []ReviewFinding
	counter := 0

	// Build set of planned file paths.
	planned := make(map[string]bool)
	for _, e := range entries {
		planned[e.Path] = true
	}

	// Determine scope boundary from planned file paths.
	scopeDirs := determineScopeDirs(entries)
	if len(scopeDirs) == 0 {
		return nil
	}

	// Collect MODIFY targets for impact analysis.
	var modifyTargets []string
	for _, e := range entries {
		for _, action := range e.Actions {
			if action == "MODIFY" {
				modifyTargets = append(modifyTargets, e.Path)
				break
			}
		}
	}

	// Walk filesystem within scope to find unplanned files.
	unplanned := collectUnplannedFiles(cfg.ProjectRoot, scopeDirs, planned)

	// Check for missing test files.
	findings = append(findings, checkMissingTests(unplanned, planned, entries, &counter)...)

	// If graph is available, use cluster and impact analysis.
	if cfg.Graph != nil && cfg.Graph.Available() {
		findings = append(findings, checkViaGraph(ctx, cfg, unplanned, planned, modifyTargets, &counter)...)
	} else {
		// Fallback: check for unplanned files that might import MODIFY targets.
		findings = append(findings, checkViaFilesystem(cfg, unplanned, modifyTargets, &counter)...)
	}

	return findings
}

// determineScopeDirs extracts unique top-level directories from planned file paths.
func determineScopeDirs(entries []FileEntry) []string {
	dirs := make(map[string]bool)
	for _, e := range entries {
		parts := strings.Split(e.Path, "/")
		if len(parts) >= 2 {
			// Use first two path components as scope (e.g., "internal/graph").
			dirs[strings.Join(parts[:2], "/")] = true
		} else if len(parts) == 1 {
			dirs["."] = true
		}
	}

	result := make([]string, 0, len(dirs))
	for d := range dirs {
		result = append(result, d)
	}
	return result
}

// collectUnplannedFiles walks directories within scope and returns source files
// not in the plan.
func collectUnplannedFiles(projectRoot string, scopeDirs []string, planned map[string]bool) []string {
	var unplanned []string
	seen := make(map[string]bool)

	for _, dir := range scopeDirs {
		absDir := filepath.Join(projectRoot, dir)
		filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Skip hidden directories and vendor/node_modules.
				name := info.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}

			ext := filepath.Ext(path)
			if !sourceExtensions[ext] {
				return nil
			}

			rel, err := filepath.Rel(projectRoot, path)
			if err != nil {
				return nil
			}

			if !planned[rel] && !seen[rel] {
				seen[rel] = true
				unplanned = append(unplanned, rel)
			}
			return nil
		})
	}

	return unplanned
}

// checkMissingTests identifies test files for planned files that exist in the
// codebase but are not included in the plan.
func checkMissingTests(unplanned []string, planned map[string]bool, entries []FileEntry, counter *int) []ReviewFinding {
	var findings []ReviewFinding

	// Build set of planned files that have a corresponding test task.
	hasTestTask := make(map[string]bool)
	for _, e := range entries {
		if isTestFile(e.Path) {
			// Find the source file this test corresponds to.
			src := testToSourcePath(e.Path)
			hasTestTask[src] = true
		}
	}

	for _, file := range unplanned {
		if !isTestFile(file) {
			continue
		}
		srcFile := testToSourcePath(file)
		if planned[srcFile] && !hasTestTask[srcFile] {
			*counter++
			findings = append(findings, ReviewFinding{
				ID:             fmt.Sprintf("R-5.%02d", *counter),
				Check:          5,
				Classification: ClassOmission,
				FilePath:       file,
				Description:    fmt.Sprintf("Test file for planned file `%s` exists but has no corresponding task", srcFile),
				Suggestion:     "Add a test update task or verify tests don't need changes",
			})
		}
	}

	return findings
}

// checkViaGraph uses cluster and impact analysis to find related unplanned files.
func checkViaGraph(ctx context.Context, cfg ReviewConfig, unplanned []string, planned map[string]bool, modifyTargets []string, counter *int) []ReviewFinding {
	var findings []ReviewFinding

	// Use AssessImpact to find transitively affected files.
	if len(modifyTargets) > 0 {
		impact, err := cfg.Graph.AssessImpact(ctx, modifyTargets)
		if err == nil && impact != nil {
			for _, affected := range impact.DirectlyAffected {
				if !planned[affected] {
					*counter++
					findings = append(findings, ReviewFinding{
						ID:             fmt.Sprintf("R-5.%02d", *counter),
						Check:          5,
						Classification: ClassOmission,
						FilePath:       affected,
						Description:    "File is directly affected by planned MODIFY targets but not in plan",
						Suggestion:     "Evaluate whether this file needs updates due to planned changes",
					})
				}
			}
		}
	}

	// Use GetClusters to find files in the same clusters as planned files.
	clusters, err := cfg.Graph.GetClusters(ctx)
	if err == nil {
		for _, cluster := range clusters {
			// Check if any cluster member is planned.
			hasPlanned := false
			for _, member := range cluster.Members {
				if planned[member] {
					hasPlanned = true
					break
				}
			}
			if !hasPlanned {
				continue
			}

			// Flag unplanned members of this cluster.
			for _, member := range cluster.Members {
				if !planned[member] && !isTestFile(member) {
					// Only flag if cohesion is high enough to be meaningful.
					if cluster.CohesionScore >= 0.5 {
						*counter++
						findings = append(findings, ReviewFinding{
							ID:             fmt.Sprintf("R-5.%02d", *counter),
							Check:          5,
							Classification: ClassOmission,
							FilePath:       member,
							Description:    fmt.Sprintf("File is in the same cluster (%s, cohesion %.2f) as planned files but not in plan", cluster.Name, cluster.CohesionScore),
							Suggestion:     "Evaluate whether this file needs updates given its tight coupling to planned files",
						})
					}
				}
			}
		}
	}

	return findings
}

// checkViaFilesystem falls back to scanning for import references when graph is unavailable.
func checkViaFilesystem(cfg ReviewConfig, unplanned []string, modifyTargets []string, counter *int) []ReviewFinding {
	var findings []ReviewFinding

	if len(modifyTargets) == 0 {
		return nil
	}

	// Build set of filenames/package names to search for.
	searchTerms := make(map[string]string) // search term -> original MODIFY target path
	for _, target := range modifyTargets {
		base := filepath.Base(target)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		searchTerms[name] = target
		// Also add the directory as a package reference.
		dir := filepath.Dir(target)
		if dir != "." {
			searchTerms[filepath.Base(dir)] = target
		}
	}

	// Scan a limited number of unplanned files for references.
	limit := 100
	for i, file := range unplanned {
		if i >= limit {
			break
		}
		absPath := filepath.Join(cfg.ProjectRoot, file)
		content, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		contentStr := string(content)
		for term, target := range searchTerms {
			if strings.Contains(contentStr, term) {
				*counter++
				findings = append(findings, ReviewFinding{
					ID:             fmt.Sprintf("R-5.%02d", *counter),
					Check:          5,
					Classification: ClassOmission,
					FilePath:       file,
					Description:    fmt.Sprintf("File references `%s` (MODIFY target `%s`) but is not in plan", term, target),
					Suggestion:     "Evaluate whether this file needs updates due to planned changes",
				})
				break // One finding per file.
			}
		}
	}

	return findings
}

// isTestFile returns true if the path looks like a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.tsx") ||
		strings.HasSuffix(base, ".spec.js") ||
		strings.HasSuffix(base, "test_") || // Python test_foo.py
		strings.HasPrefix(base, "test_")
}

// testToSourcePath converts a test file path to its corresponding source file path.
func testToSourcePath(path string) string {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// Go: foo_test.go -> foo.go
	if strings.HasSuffix(base, "_test.go") {
		return filepath.Join(dir, strings.TrimSuffix(base, "_test.go")+".go")
	}
	// TS/JS: foo.test.ts -> foo.ts, foo.spec.ts -> foo.ts
	for _, suffix := range []string{".test.ts", ".test.tsx", ".test.js", ".spec.ts", ".spec.tsx", ".spec.js"} {
		if strings.HasSuffix(base, suffix) {
			ext := strings.TrimPrefix(suffix, ".test")
			ext = strings.TrimPrefix(ext, ".spec")
			return filepath.Join(dir, strings.TrimSuffix(base, suffix)+ext)
		}
	}
	// Python: test_foo.py -> foo.py
	if strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") {
		return filepath.Join(dir, strings.TrimPrefix(base, "test_"))
	}

	return path
}
