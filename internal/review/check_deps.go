package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/onedusk/pd/internal/graph"
)

// importPatterns match common import statements across languages.
var importPatterns = []*regexp.Regexp{
	// Go: import "path" or import ( "path" )
	regexp.MustCompile(`(?m)(?:import\s+(?:\(\s*)?)"([^"]+)"`),
	// TypeScript/JavaScript: import ... from 'path' or require('path')
	regexp.MustCompile(`(?m)(?:from\s+['"]([^'"]+)['"]|require\s*\(\s*['"]([^'"]+)['"]\s*\))`),
	// Python: from path import ... or import path
	regexp.MustCompile(`(?m)(?:from\s+(\S+)\s+import|^import\s+(\S+))`),
	// Rust: use crate::path or mod path
	regexp.MustCompile(`(?m)(?:use\s+(?:crate::)?(\S+)|mod\s+(\S+))`),
}

// CheckDependencyCompleteness verifies that all files importing MODIFY targets
// are accounted for in the plan.
//
// DIRECTION SEMANTICS (validated by TestDirectionSemantics):
//   - DirectionUpstream from B returns files that import B (B's dependents)
//   - This is what Check 3 needs: "who depends on each MODIFY target?"
func CheckDependencyCompleteness(ctx context.Context, cfg ReviewConfig, entries []FileEntry, tasks []TaskEntry) []ReviewFinding {
	var findings []ReviewFinding
	counter := 0

	// Build set of all planned file paths.
	planned := make(map[string]bool)
	for _, e := range entries {
		planned[e.Path] = true
	}

	// Collect MODIFY targets from entries.
	var modifyTargets []string
	for _, e := range entries {
		for _, action := range e.Actions {
			if action == "MODIFY" {
				modifyTargets = append(modifyTargets, e.Path)
				break
			}
		}
	}

	for _, target := range modifyTargets {
		var dependents []string

		if cfg.Graph != nil && cfg.Graph.Available() {
			dependents = findDependentsViaGraph(ctx, cfg.Graph, target)
		} else {
			dependents = findDependentsViaFilesystem(cfg.ProjectRoot, target)
		}

		for _, dep := range dependents {
			if !planned[dep] && dep != target {
				counter++
				findings = append(findings, ReviewFinding{
					ID:             fmt.Sprintf("R-3.%02d", counter),
					Check:          3,
					Classification: ClassOmission,
					FilePath:       dep,
					Description:    fmt.Sprintf("File imports MODIFY target `%s` but is not in the plan", target),
					Suggestion:     "Evaluate whether this file needs updates due to the planned changes",
				})
			}
		}
	}

	return findings
}

// findDependentsViaGraph uses the code intelligence graph to find files
// that import the target file.
func findDependentsViaGraph(ctx context.Context, gp GraphProvider, target string) []string {
	// DirectionUpstream: follows edges where TargetID == target,
	// returning SourceID = files that import target.
	chains, err := gp.GetDependencies(ctx, target, graph.DirectionUpstream, 1)
	if err != nil {
		return nil
	}

	var dependents []string
	for _, chain := range chains {
		if len(chain.Nodes) >= 2 {
			dep := chain.Nodes[len(chain.Nodes)-1]
			dependents = append(dependents, dep)
		}
	}
	return dependents
}

// findDependentsViaFilesystem falls back to scanning source files for import
// statements that reference the target.
func findDependentsViaFilesystem(projectRoot, target string) []string {
	targetBase := filepath.Base(target)
	targetName := strings.TrimSuffix(targetBase, filepath.Ext(targetBase))
	targetDir := filepath.Dir(target)

	var dependents []string
	seen := make(map[string]bool)

	filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !sourceExtensions[ext] {
			return nil
		}

		rel, err := filepath.Rel(projectRoot, path)
		if err != nil || rel == target {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)

		// Check if the file references the target by name or package.
		if strings.Contains(contentStr, targetName) || strings.Contains(contentStr, targetDir) {
			// Verify with import patterns.
			for _, pattern := range importPatterns {
				matches := pattern.FindAllStringSubmatch(contentStr, -1)
				for _, m := range matches {
					for _, group := range m[1:] {
						if group == "" {
							continue
						}
						if strings.Contains(group, targetName) || strings.Contains(group, targetDir) {
							if !seen[rel] {
								seen[rel] = true
								dependents = append(dependents, rel)
							}
						}
					}
				}
			}
		}

		return nil
	})

	return dependents
}
