package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ReviewDecision represents the outcome of a human review.
type ReviewDecision string

const (
	ReviewApproved ReviewDecision = "approved"
	ReviewRejected ReviewDecision = "rejected"
	ReviewRevise   ReviewDecision = "revise"
)

// ImplementationArtifact represents a file changed during implementation.
type ImplementationArtifact struct {
	Path       string `json:"path"`
	Action     string `json:"action"` // CREATE, MODIFY, DELETE
	DiffOrBody string `json:"diffOrBody"`
}

// ReviewStrategy defines how implementation outputs are reviewed.
type ReviewStrategy interface {
	// RequestReview presents the milestone output for review and blocks
	// until a decision is made.
	RequestReview(ctx context.Context, milestone *MilestoneNode, artifacts []ImplementationArtifact) (ReviewDecision, error)
}

// ---------------------------------------------------------------------------
// CLI Review Strategy
// ---------------------------------------------------------------------------

// CLIReviewStrategy implements ReviewStrategy by prompting the user in the
// terminal for approval.
type CLIReviewStrategy struct {
	reader io.Reader
	writer io.Writer
}

// NewCLIReviewStrategy creates a CLIReviewStrategy using the given reader
// and writer (typically os.Stdin and os.Stdout).
func NewCLIReviewStrategy(reader io.Reader, writer io.Writer) *CLIReviewStrategy {
	return &CLIReviewStrategy{reader: reader, writer: writer}
}

func (c *CLIReviewStrategy) RequestReview(_ context.Context, milestone *MilestoneNode, artifacts []ImplementationArtifact) (ReviewDecision, error) {
	fmt.Fprintf(c.writer, "\n=== Review: %s (%s) ===\n\n", milestone.ID, milestone.Name)
	fmt.Fprintf(c.writer, "Changed files:\n")
	for _, a := range artifacts {
		fmt.Fprintf(c.writer, "  [%s] %s\n", a.Action, a.Path)
	}
	fmt.Fprintf(c.writer, "\n")

	scanner := bufio.NewScanner(c.reader)
	for {
		fmt.Fprintf(c.writer, "[a]pprove / [r]eject / [v]iew diffs: ")
		if !scanner.Scan() {
			return ReviewRejected, fmt.Errorf("input stream closed")
		}

		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		switch {
		case input == "a" || input == "approve":
			return ReviewApproved, nil
		case input == "r" || input == "reject":
			return ReviewRejected, nil
		case input == "v" || input == "view":
			for _, a := range artifacts {
				fmt.Fprintf(c.writer, "\n--- %s [%s] ---\n", a.Path, a.Action)
				if a.DiffOrBody != "" {
					fmt.Fprintf(c.writer, "%s\n", a.DiffOrBody)
				} else {
					fmt.Fprintf(c.writer, "(no diff available)\n")
				}
			}
			fmt.Fprintf(c.writer, "\n")
		default:
			fmt.Fprintf(c.writer, "Unknown option %q. Please enter a, r, or v.\n", input)
		}
	}
}

// ---------------------------------------------------------------------------
// PR Review Strategy
// ---------------------------------------------------------------------------

// PRReviewStrategy implements ReviewStrategy by creating a GitHub PR and
// waiting for it to be merged or closed.
type PRReviewStrategy struct {
	baseBranch string
	remote     string
}

// NewPRReviewStrategy creates a PRReviewStrategy targeting the given base
// branch and remote.
func NewPRReviewStrategy(baseBranch, remote string) *PRReviewStrategy {
	return &PRReviewStrategy{baseBranch: baseBranch, remote: remote}
}

func (p *PRReviewStrategy) RequestReview(ctx context.Context, milestone *MilestoneNode, artifacts []ImplementationArtifact) (ReviewDecision, error) {
	branchName := fmt.Sprintf("implement/%s", strings.ToLower(milestone.ID))

	// Create and checkout branch.
	if err := runGit(ctx, "checkout", "-b", branchName); err != nil {
		return ReviewRejected, fmt.Errorf("create branch: %w", err)
	}

	// Stage changed files.
	for _, a := range artifacts {
		if a.Action == "DELETE" {
			_ = runGit(ctx, "rm", a.Path)
		} else {
			_ = runGit(ctx, "add", a.Path)
		}
	}

	// Commit.
	commitMsg := fmt.Sprintf("implement %s: %s", milestone.ID, milestone.Name)
	if err := runGit(ctx, "commit", "-m", commitMsg); err != nil {
		return ReviewRejected, fmt.Errorf("commit: %w", err)
	}

	// Push.
	if err := runGit(ctx, "push", "-u", p.remote, branchName); err != nil {
		return ReviewRejected, fmt.Errorf("push: %w", err)
	}

	// Create PR.
	body := fmt.Sprintf("## %s: %s\n\nAutomated implementation milestone.", milestone.ID, milestone.Name)
	prCmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--base", p.baseBranch,
		"--head", branchName,
		"--title", commitMsg,
		"--body", body,
	)
	if output, err := prCmd.CombinedOutput(); err != nil {
		return ReviewRejected, fmt.Errorf("create PR: %w (%s)", err, string(output))
	}

	// Poll PR status until merged or closed.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ReviewRejected, ctx.Err()
		case <-ticker.C:
			state, err := getPRState(ctx, branchName)
			if err != nil {
				continue // transient error, keep polling
			}
			switch state {
			case "MERGED":
				return ReviewApproved, nil
			case "CLOSED":
				return ReviewRejected, nil
			}
			// OPEN: keep polling
		}
	}
}

func runGit(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func getPRState(ctx context.Context, branch string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", branch, "--json", "state", "-q", ".state")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ---------------------------------------------------------------------------
// File Review Strategy
// ---------------------------------------------------------------------------

// FileReviewStrategy implements ReviewStrategy by writing a review report
// and polling for an approval sentinel file.
type FileReviewStrategy struct {
	reportDir    string
	pollInterval time.Duration
}

// NewFileReviewStrategy creates a FileReviewStrategy that writes reports to
// and watches for sentinel files in the given directory.
func NewFileReviewStrategy(reportDir string) *FileReviewStrategy {
	return &FileReviewStrategy{
		reportDir:    reportDir,
		pollInterval: 5 * time.Second,
	}
}

func (f *FileReviewStrategy) RequestReview(ctx context.Context, milestone *MilestoneNode, artifacts []ImplementationArtifact) (ReviewDecision, error) {
	if err := os.MkdirAll(f.reportDir, 0o755); err != nil {
		return ReviewRejected, fmt.Errorf("create report dir: %w", err)
	}

	baseName := fmt.Sprintf("milestone-%s-review", strings.ToLower(milestone.ID))

	// Write the review report.
	reportPath := filepath.Join(f.reportDir, baseName+".md")
	var report strings.Builder
	fmt.Fprintf(&report, "# Review: %s — %s\n\n", milestone.ID, milestone.Name)
	fmt.Fprintf(&report, "## Changed Files\n\n")
	for _, a := range artifacts {
		fmt.Fprintf(&report, "- [%s] `%s`\n", a.Action, a.Path)
	}
	fmt.Fprintf(&report, "\n## Instructions\n\n")
	fmt.Fprintf(&report, "Review the changes above.\n")
	fmt.Fprintf(&report, "- To approve: create `%s.approved` in this directory\n", baseName)
	fmt.Fprintf(&report, "- To reject: create `%s.rejected` in this directory\n", baseName)

	if err := os.WriteFile(reportPath, []byte(report.String()), 0o644); err != nil {
		return ReviewRejected, fmt.Errorf("write review report: %w", err)
	}

	// Poll for sentinel files.
	approvedPath := filepath.Join(f.reportDir, baseName+".approved")
	rejectedPath := filepath.Join(f.reportDir, baseName+".rejected")

	ticker := time.NewTicker(f.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ReviewRejected, ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(approvedPath); err == nil {
				return ReviewApproved, nil
			}
			if _, err := os.Stat(rejectedPath); err == nil {
				return ReviewRejected, nil
			}
		}
	}
}
