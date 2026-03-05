package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/onedusk/pd/internal/a2a"
	"github.com/onedusk/pd/internal/graph"
	"github.com/onedusk/pd/internal/review"
	"github.com/onedusk/pd/internal/status"
)

// runReview executes the review phase for a decomposition.
// It runs all 5 mechanical checks and writes review-findings.md.
func runReview(_ context.Context, projectRoot, name string, flags cliFlags) error {
	outputDir := filepath.Join(projectRoot, "docs", "decompose", name)

	// Verify stages 1-4 are complete.
	ds := status.GetDecompositionStatus(projectRoot, name)
	for _, s := range ds.Stages {
		if s.Stage == 0 {
			continue // Stage 0 is optional for review.
		}
		if !s.Complete {
			return fmt.Errorf("stage %d (%s) is not complete; complete all stages before reviewing", s.Stage, s.Name)
		}
	}

	// Attempt to open graph store.
	var gp review.GraphProvider
	graphDir := filepath.Join(projectRoot, ".decompose", "graph")
	if _, err := os.Stat(graphDir); err == nil {
		store, err := graph.NewKuzuFileStore(graphDir)
		if err == nil {
			defer store.Close()
			gp = review.NewStoreGraphProvider(store)
			if flags.Verbose {
				fmt.Fprintln(os.Stderr, "Graph store opened for review checks")
			}
		} else if flags.Verbose {
			fmt.Fprintf(os.Stderr, "warning: could not open graph store: %v\n", err)
		}
	}

	// Get git commit hash.
	commitHash := getCommitHash()

	cfg := review.ReviewConfig{
		ProjectRoot: projectRoot,
		DecompName:  name,
		DecompDir:   outputDir,
		Graph:       gp,
	}

	ctx := context.Background()
	report, err := review.RunReview(ctx, cfg)
	if err != nil {
		return fmt.Errorf("run review: %w", err)
	}

	report.CommitHash = commitHash

	// Write findings file.
	findingsPath := filepath.Join(outputDir, "review-findings.md")
	if err := os.WriteFile(findingsPath, []byte(report.Markdown()), 0o644); err != nil {
		return fmt.Errorf("write findings: %w", err)
	}

	// Print summary to stderr.
	total := len(report.Findings)
	mismatches := report.MismatchCount()
	fmt.Fprintf(os.Stderr, "Review complete: %d findings", total)
	if mismatches > 0 {
		fmt.Fprintf(os.Stderr, " (%d MISMATCHes)", mismatches)
	}
	fmt.Fprintln(os.Stderr)

	for _, cs := range report.Checks {
		if cs.Total > 0 {
			fmt.Fprintf(os.Stderr, "  Check %d (%s): %d findings\n", cs.Check, cs.Name, cs.Total)
		}
	}

	// Print output file path to stdout.
	fmt.Println(findingsPath)

	if mismatches > 0 {
		fmt.Fprintln(os.Stderr, "\nResolve MISMATCHes before implementing. Run `decompose review-interpret` for interpretive triage.")
	}

	return nil
}

// runReviewInterpret handles the A2A interpretive triage delegation.
// Tries A2A agent discovery first; falls back to printing instructions.
func runReviewInterpret(ctx context.Context, projectRoot, name string, flags cliFlags) error {
	outputDir := filepath.Join(projectRoot, "docs", "decompose", name)
	findingsPath := filepath.Join(outputDir, "review-findings.md")

	if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
		return fmt.Errorf("review-findings.md not found; run `decompose review %s` first", name)
	}

	// Try A2A agent endpoints if configured.
	var endpoints []string
	if flags.Agents != "" {
		for _, ep := range strings.Split(flags.Agents, ",") {
			endpoints = append(endpoints, strings.TrimSpace(ep))
		}
	} else {
		// Default: try localhost on common A2A ports.
		endpoints = []string{"http://localhost:8080", "http://localhost:9000"}
	}

	client := a2a.NewHTTPClient(a2a.WithTimeout(5 * time.Second))
	cfg := review.InterpretConfig{
		DecompName: name,
		DecompDir:  outputDir,
		Endpoints:  endpoints,
	}

	result, err := review.SubmitInterpretTask(ctx, client, cfg)
	if err != nil {
		// A2A not available — fall back to printing instructions.
		if flags.Verbose {
			fmt.Fprintf(os.Stderr, "A2A agent not available: %v\n", err)
		}
		fmt.Fprintln(os.Stderr, "No A2A review-interpret agent found. Printing instructions for manual delegation.")
		fmt.Println(reviewInterpretInstructions(name))
		return nil
	}

	fmt.Fprintf(os.Stderr, "Review-interpret task submitted to %s\n", result.Endpoint)
	fmt.Fprintf(os.Stderr, "Task ID: %s\n", result.TaskID)
	fmt.Fprintf(os.Stderr, "Polling for completion...\n")

	// Poll with a longer timeout for the actual work.
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	pollClient := a2a.NewHTTPClient(a2a.WithTimeout(30 * time.Second))
	task, err := review.PollInterpretTask(pollCtx, pollClient, result.Endpoint, result.TaskID, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Polling stopped: %v\n", err)
		fmt.Fprintf(os.Stderr, "Check task status manually: decompose review-interpret %s\n", name)
		return nil
	}

	if task.Status.State == a2a.TaskStateCompleted {
		fmt.Fprintln(os.Stderr, "Review-interpret task completed successfully.")
		// Check if the agent wrote an updated findings artifact.
		for _, artifact := range task.Artifacts {
			if artifact.Name == "review-findings" {
				for _, part := range artifact.Parts {
					if part.Text != "" {
						if err := os.WriteFile(findingsPath, []byte(part.Text), 0o644); err != nil {
							return fmt.Errorf("write updated findings: %w", err)
						}
						fmt.Fprintf(os.Stderr, "Updated %s with interpretive triage results.\n", findingsPath)
					}
				}
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "Review-interpret task ended with state: %s\n", task.Status.State)
		if task.Status.Message != nil {
			for _, part := range task.Status.Message.Parts {
				if part.Text != "" {
					fmt.Fprintln(os.Stderr, part.Text)
				}
			}
		}
	}

	return nil
}

// checkReviewBeforeImplement checks the review state and prints warnings.
// Returns nil even if warnings are printed (non-blocking).
func checkReviewBeforeImplement(projectRoot, name string, skipReview bool) {
	if skipReview {
		return
	}

	outputDir := filepath.Join(projectRoot, "docs", "decompose", name)
	findingsPath := filepath.Join(outputDir, "review-findings.md")

	content, err := os.ReadFile(findingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Review phase has not been run. Consider running `decompose review %s` first.\n", name)
		return
	}

	mismatches := review.ParseMismatchCount(string(content))
	if mismatches > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: Review found %d unresolved MISMATCH(es). Resolve them before implementing or pass --skip-review to suppress.\n", mismatches)
	}
}

// getCommitHash returns the current git HEAD commit hash, or empty string.
func getCommitHash() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// reviewInterpretInstructions returns the instructions for the interpretive triage session.
func reviewInterpretInstructions(name string) string {
	return fmt.Sprintf(`Interpretive Triage Instructions
=================================

Run the following in a fresh Claude Code session with the project root as working directory:

READ FIRST:
- docs/decompose/%[1]s/review-findings.md (the mechanical findings)
- docs/decompose/%[1]s/stage-3-task-index.md (the milestone plan)
- All docs/decompose/%[1]s/tasks_m*.md files (the task specs)
- docs/decompose/%[1]s/stage-1-design-pack.md (the original design)

YOUR TASKS:

1. TRIAGE FALSE POSITIVES (especially from Check 5)
   For each OMISSION finding from Check 5, read the unlisted file and the
   planned files it relates to. Determine whether the planned changes will
   actually break the unlisted file. If not, reclassify as OK.

2. CATCH SEMANTIC CONFLICTS (Check 4 gaps)
   Read task outlines for every multi-milestone file and assess whether the
   changes are compatible. Add new findings with classification MISMATCH or
   OMISSION as appropriate.

3. VERIFY DEPENDENCY DIRECTION (Check 3 validation)
   For each OMISSION from Check 3, read the dependent file and the MODIFY
   target. Assess whether the planned change is backward-compatible.

4. WRITE RECOMMENDED PLAN UPDATES
   Replace the <!-- INTERPRETIVE PASS NEEDED --> stub with actionable
   recommendations grouped by milestone.

5. UPDATE THE SUMMARY TABLE
   Recalculate summary table counts after triage.

OUTPUT:
Write the updated review-findings.md back to docs/decompose/%[1]s/review-findings.md.
Do NOT modify any Stage 3 or Stage 4 files.`, name)
}
