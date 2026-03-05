// Package review implements the review phase for progressive decomposition.
// It runs 5 mechanical checks comparing decomposition plan outputs (Stage 3
// directory tree + Stage 4 task specs) against the actual codebase, producing
// structured findings for interpretive triage by a separate agent session.
package review

import (
	"context"
	"fmt"
	"time"

	"github.com/onedusk/pd/internal/graph"
)

// Classification indicates how a review finding should be handled.
type Classification string

const (
	ClassMismatch Classification = "MISMATCH" // Plan contradicts codebase — must fix
	ClassOmission Classification = "OMISSION" // Plan misses something — evaluate impact
	ClassStale    Classification = "STALE"    // Plan references something changed — update plan
	ClassOK       Classification = "OK"       // No issue found
)

// ReviewFinding is a single issue found during the review phase.
type ReviewFinding struct {
	ID             string         `json:"id"`             // "R-1.03" format
	Check          int            `json:"check"`          // 1-5
	Classification Classification `json:"classification"` // MISMATCH, OMISSION, STALE, OK
	FilePath       string         `json:"filePath"`       // always populated — findings are self-locating
	TaskID         string         `json:"taskId,omitempty"`
	Milestone      string         `json:"milestone,omitempty"`
	Description    string         `json:"description"`
	Suggestion     string         `json:"suggestion"`
}

// String returns a self-locating representation:
// "R-1.03 [MISMATCH] `path/to/file.go`: File already exists, task specifies CREATE"
func (f ReviewFinding) String() string {
	s := fmt.Sprintf("%s [%s] `%s`: %s", f.ID, f.Classification, f.FilePath, f.Description)
	if f.Suggestion != "" {
		s += " — " + f.Suggestion
	}
	return s
}

// CheckSummary tallies findings for one check.
type CheckSummary struct {
	Check      int    `json:"check"`
	Name       string `json:"name"`
	Total      int    `json:"total"`
	Mismatches int    `json:"mismatches"`
	Omissions  int    `json:"omissions"`
	Stale      int    `json:"stale"`
}

// ReviewReport is the structured output of a full review pass.
type ReviewReport struct {
	Name          string          `json:"name"`
	Timestamp     time.Time       `json:"timestamp"`
	CommitHash    string          `json:"commitHash"`
	GraphIndexed  bool            `json:"graphIndexed"`
	Checks        []CheckSummary  `json:"checks"`
	Findings      []ReviewFinding `json:"findings"`
	HasMismatches bool            `json:"hasMismatches"`
}

// FileEntry represents one file from the Stage 3 directory tree.
type FileEntry struct {
	Path       string            `json:"path"`       // repo-relative path
	Actions    map[string]string `json:"actions"`     // milestone -> action (CREATE/MODIFY/DELETE)
	Milestones []string          `json:"milestones"`  // ordered list of milestones that touch this file
}

// TaskEntry represents one task from Stage 4.
type TaskEntry struct {
	ID         string   `json:"id"`         // "T-01.03"
	Milestone  string   `json:"milestone"`  // "M1"
	File       string   `json:"file"`       // file path
	Action     string   `json:"action"`     // CREATE, MODIFY, DELETE
	DependsOn  []string `json:"dependsOn"`  // task IDs
	SymbolRefs []string `json:"symbolRefs"` // symbol names from outline
	Outline    string   `json:"outline"`    // raw outline text
}

// GraphProvider abstracts graph access for the review engine.
// When graph is available, wraps graph.Store. When not, Available() returns false
// and callers fall back to filesystem-based checks.
type GraphProvider interface {
	Available() bool
	QuerySymbols(ctx context.Context, query string, limit int) ([]graph.SymbolNode, error)
	GetDependencies(ctx context.Context, nodeID string, direction graph.Direction, maxDepth int) ([]graph.DependencyChain, error)
	GetClusters(ctx context.Context) ([]graph.ClusterNode, error)
	AssessImpact(ctx context.Context, changedFiles []string) (*graph.ImpactResult, error)
}

// ReviewConfig holds inputs for running a review.
type ReviewConfig struct {
	ProjectRoot string        // absolute path to the target project
	DecompName  string        // decomposition name (kebab-case)
	DecompDir   string        // docs/decompose/<name>/
	Graph       GraphProvider // nil = no graph available
}

// storeGraphProvider wraps a graph.Store to satisfy GraphProvider.
type storeGraphProvider struct {
	store graph.Store
}

// NewStoreGraphProvider wraps a graph.Store as a GraphProvider.
func NewStoreGraphProvider(s graph.Store) GraphProvider {
	if s == nil {
		return nil
	}
	return &storeGraphProvider{store: s}
}

func (p *storeGraphProvider) Available() bool { return true }

func (p *storeGraphProvider) QuerySymbols(ctx context.Context, query string, limit int) ([]graph.SymbolNode, error) {
	return p.store.QuerySymbols(ctx, query, limit)
}

func (p *storeGraphProvider) GetDependencies(ctx context.Context, nodeID string, direction graph.Direction, maxDepth int) ([]graph.DependencyChain, error) {
	return p.store.GetDependencies(ctx, nodeID, direction, maxDepth)
}

func (p *storeGraphProvider) GetClusters(ctx context.Context) ([]graph.ClusterNode, error) {
	return p.store.GetClusters(ctx)
}

func (p *storeGraphProvider) AssessImpact(ctx context.Context, changedFiles []string) (*graph.ImpactResult, error) {
	return p.store.AssessImpact(ctx, changedFiles)
}

// checkNames maps check numbers to human-readable names.
var checkNames = map[int]string{
	1: "File existence",
	2: "Symbol verification",
	3: "Dependency completeness",
	4: "Cross-milestone consistency",
	5: "Coverage gap scan",
}

// RunReview runs all 5 mechanical checks and returns the ReviewReport.
func RunReview(ctx context.Context, cfg ReviewConfig) (*ReviewReport, error) {
	// Parse Stage 3 directory tree.
	entries, stage3Content, err := loadStage3(cfg)
	if err != nil {
		return nil, fmt.Errorf("parse stage 3: %w", err)
	}

	// Parse Stage 4 task specs.
	tasks, err := loadStage4(cfg)
	if err != nil {
		return nil, fmt.Errorf("parse stage 4: %w", err)
	}

	var allFindings []ReviewFinding

	// Check 1: File existence.
	allFindings = append(allFindings, CheckFileExistence(ctx, cfg, entries)...)

	// Check 2: Symbol verification.
	allFindings = append(allFindings, CheckSymbols(ctx, cfg, tasks)...)

	// Check 3: Dependency completeness.
	allFindings = append(allFindings, CheckDependencyCompleteness(ctx, cfg, entries, tasks)...)

	// Check 4: Cross-milestone consistency.
	allFindings = append(allFindings, CheckCrossMilestoneConsistency(ctx, cfg, entries, tasks, stage3Content)...)

	// Check 5: Coverage gap scan.
	allFindings = append(allFindings, CheckCoverageGaps(ctx, cfg, entries)...)

	// Build check summaries.
	checks := make([]CheckSummary, 5)
	for i := 1; i <= 5; i++ {
		cs := CheckSummary{Check: i, Name: checkNames[i]}
		for _, f := range allFindings {
			if f.Check != i {
				continue
			}
			cs.Total++
			switch f.Classification {
			case ClassMismatch:
				cs.Mismatches++
			case ClassOmission:
				cs.Omissions++
			case ClassStale:
				cs.Stale++
			}
		}
		checks[i-1] = cs
	}

	hasMismatches := false
	for _, f := range allFindings {
		if f.Classification == ClassMismatch {
			hasMismatches = true
			break
		}
	}

	return &ReviewReport{
		Name:          cfg.DecompName,
		Timestamp:     time.Now(),
		GraphIndexed:  cfg.Graph != nil && cfg.Graph.Available(),
		Checks:        checks,
		Findings:      allFindings,
		HasMismatches: hasMismatches,
	}, nil
}

// loadStage3 reads and parses the Stage 3 file, returning file entries and raw content.
func loadStage3(cfg ReviewConfig) ([]FileEntry, string, error) {
	return LoadAndParseStage3(cfg.DecompDir)
}

// loadStage4 reads and parses all Stage 4 task spec files.
func loadStage4(cfg ReviewConfig) ([]TaskEntry, error) {
	return LoadAndParseStage4(cfg.DecompDir)
}
