package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// symbolLookupLimit is the row limit for the substring query behind an exact
// symbol lookup. It matches the cap build_graph uses when it persists symbols,
// so the exact name is not cut off by other names that contain it.
const symbolLookupLimit = 100000

// CheckSymbols verifies that symbols referenced in MODIFY task outlines exist
// in the codebase with expected locations.
func CheckSymbols(ctx context.Context, cfg ReviewConfig, tasks []TaskEntry) []ReviewFinding {
	var findings []ReviewFinding
	counter := 0

	for _, task := range tasks {
		if task.Action != "MODIFY" {
			continue
		}
		if len(task.SymbolRefs) == 0 {
			continue
		}

		for _, sym := range task.SymbolRefs {
			finding := checkOneSymbol(ctx, cfg, task, sym, &counter)
			if finding != nil {
				findings = append(findings, *finding)
			}
		}
	}

	return findings
}

// checkOneSymbol checks a single symbol reference against the codebase.
func checkOneSymbol(ctx context.Context, cfg ReviewConfig, task TaskEntry, symbolName string, counter *int) *ReviewFinding {
	if cfg.Graph != nil && cfg.Graph.Available() {
		return checkSymbolViaGraph(ctx, cfg, task, symbolName, counter)
	}
	return checkSymbolViaFile(cfg, task, symbolName, counter)
}

// checkSymbolViaGraph uses the code intelligence graph to verify a symbol.
func checkSymbolViaGraph(ctx context.Context, cfg ReviewConfig, task TaskEntry, symbolName string, counter *int) *ReviewFinding {
	results, err := cfg.Graph.QuerySymbols(ctx, symbolName, symbolLookupLimit)
	if err != nil {
		return nil // Graph error is non-fatal; skip this check.
	}

	// QuerySymbols matches substrings; keep the files that define the exact name.
	var otherFiles []string
	for _, sym := range results {
		if sym.Name != symbolName {
			continue
		}
		if sym.FilePath == task.File {
			return nil // Found in expected location.
		}
		otherFiles = append(otherFiles, sym.FilePath)
	}

	// Symbol exists in a different file. Report the first by path so the
	// finding does not change between runs.
	if len(otherFiles) > 0 {
		sort.Strings(otherFiles)
		*counter++
		return &ReviewFinding{
			ID:             fmt.Sprintf("R-2.%02d", *counter),
			Check:          2,
			Classification: ClassStale,
			FilePath:       task.File,
			TaskID:         task.ID,
			Milestone:      task.Milestone,
			Description:    fmt.Sprintf("Symbol `%s` not found in `%s` but exists in `%s`", symbolName, task.File, otherFiles[0]),
			Suggestion:     fmt.Sprintf("Update task to reference `%s` instead", otherFiles[0]),
		}
	}

	// Symbol not found anywhere.
	*counter++
	return &ReviewFinding{
		ID:             fmt.Sprintf("R-2.%02d", *counter),
		Check:          2,
		Classification: ClassMismatch,
		FilePath:       task.File,
		TaskID:         task.ID,
		Milestone:      task.Milestone,
		Description:    fmt.Sprintf("Symbol `%s` referenced in task outline does not exist in codebase", symbolName),
		Suggestion:     "Verify the symbol name or update the task outline",
	}
}

// checkSymbolViaFile falls back to reading the target file and searching for the symbol.
func checkSymbolViaFile(cfg ReviewConfig, task TaskEntry, symbolName string, counter *int) *ReviewFinding {
	absPath := filepath.Join(cfg.ProjectRoot, task.File)
	content, err := os.ReadFile(absPath)
	if err != nil {
		// File doesn't exist — Check 1 would have caught this already.
		return nil
	}

	if strings.Contains(string(content), symbolName) {
		return nil // Found in expected file.
	}

	// Symbol not found in the file. We can't cheaply search the whole codebase
	// without the graph, so report as a potential mismatch.
	*counter++
	return &ReviewFinding{
		ID:             fmt.Sprintf("R-2.%02d", *counter),
		Check:          2,
		Classification: ClassMismatch,
		FilePath:       task.File,
		TaskID:         task.ID,
		Milestone:      task.Milestone,
		Description:    fmt.Sprintf("Symbol `%s` not found in `%s` (searched via file read, graph unavailable)", symbolName, task.File),
		Suggestion:     "Verify the symbol name exists in the file or run with graph for better detection",
	}
}
