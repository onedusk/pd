package review

import (
	"fmt"
	"sort"
	"strings"
)

// Markdown formats the ReviewReport as the markdown document specified
// in docs/review-phase.md. Produces the mechanical findings with a stub
// for the interpretive pass.
func (r *ReviewReport) Markdown() string {
	var b strings.Builder

	// Header.
	fmt.Fprintf(&b, "# Review Findings: %s\n\n", r.Name)
	fmt.Fprintf(&b, "**Review date:** %s\n", r.Timestamp.Format("2006-01-02"))
	if r.CommitHash != "" {
		fmt.Fprintf(&b, "**Codebase state:** %s\n", r.CommitHash)
	}
	graphStatus := "no"
	if r.GraphIndexed {
		graphStatus = "yes"
	}
	fmt.Fprintf(&b, "**Graph indexed:** %s\n\n", graphStatus)

	// Summary table.
	fmt.Fprintln(&b, "## Summary")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Check | Findings | Mismatches | Omissions | Stale |")
	fmt.Fprintln(&b, "|-------|----------|-----------|-----------|-------|")

	totalFindings, totalMismatches, totalOmissions, totalStale := 0, 0, 0, 0
	for _, cs := range r.Checks {
		fmt.Fprintf(&b, "| %d. %s | %d | %s | %s | %s |\n",
			cs.Check, cs.Name, cs.Total,
			dashIfZero(cs.Mismatches), dashIfZero(cs.Omissions), dashIfZero(cs.Stale))
		totalFindings += cs.Total
		totalMismatches += cs.Mismatches
		totalOmissions += cs.Omissions
		totalStale += cs.Stale
	}
	fmt.Fprintf(&b, "| **Total** | **%d** | **%s** | **%s** | **%s** |\n\n",
		totalFindings, dashIfZero(totalMismatches), dashIfZero(totalOmissions), dashIfZero(totalStale))

	// Per-check findings sections.
	for checkNum := 1; checkNum <= 5; checkNum++ {
		name, ok := checkNames[checkNum]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "## Check %d: %s\n\n", checkNum, name)

		checkFindings := r.findingsByCheck(checkNum)
		if len(checkFindings) == 0 {
			fmt.Fprintln(&b, "No findings.")
			fmt.Fprintln(&b)
			continue
		}

		for _, f := range checkFindings {
			fmt.Fprintf(&b, "- **%s** [%s] `%s`: %s\n", f.ID, f.Classification, f.FilePath, f.Description)
			if f.TaskID != "" {
				fmt.Fprintf(&b, "  - Task: %s", f.TaskID)
				if f.Milestone != "" {
					fmt.Fprintf(&b, " (%s)", f.Milestone)
				}
				fmt.Fprintln(&b)
			}
			if f.Suggestion != "" {
				fmt.Fprintf(&b, "  - Suggestion: %s\n", f.Suggestion)
			}
		}
		fmt.Fprintln(&b)
	}

	// Recommended Plan Updates section.
	fmt.Fprintln(&b, "## Recommended Plan Updates")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "<!-- INTERPRETIVE PASS NEEDED -->")
	fmt.Fprintln(&b, "<!-- Run `decompose review-interpret` to fill in actionable recommendations -->")
	fmt.Fprintln(&b)

	actionableFindings := r.actionableFindings()
	if len(actionableFindings) == 0 {
		fmt.Fprintln(&b, "No actionable findings.")
	} else {
		// Group by milestone.
		byMilestone := make(map[string][]ReviewFinding)
		noMilestone := []ReviewFinding{}
		for _, f := range actionableFindings {
			if f.Milestone != "" {
				byMilestone[f.Milestone] = append(byMilestone[f.Milestone], f)
			} else {
				noMilestone = append(noMilestone, f)
			}
		}

		// Sort milestone keys.
		var milestones []string
		for ms := range byMilestone {
			milestones = append(milestones, ms)
		}
		sort.Strings(milestones)

		for _, ms := range milestones {
			fmt.Fprintf(&b, "### %s\n\n", ms)
			for _, f := range byMilestone[ms] {
				fmt.Fprintf(&b, "- [%s] %s `%s`: %s\n", f.Classification, f.ID, f.FilePath, f.Description)
			}
			fmt.Fprintln(&b)
		}

		if len(noMilestone) > 0 {
			fmt.Fprintln(&b, "### Unassigned")
			fmt.Fprintln(&b)
			for _, f := range noMilestone {
				fmt.Fprintf(&b, "- [%s] %s `%s`: %s\n", f.Classification, f.ID, f.FilePath, f.Description)
			}
			fmt.Fprintln(&b)
		}
	}

	return b.String()
}

// MismatchCount returns the number of MISMATCH findings in the report.
func (r *ReviewReport) MismatchCount() int {
	count := 0
	for _, f := range r.Findings {
		if f.Classification == ClassMismatch {
			count++
		}
	}
	return count
}

// findingsByCheck returns findings for a specific check number.
func (r *ReviewReport) findingsByCheck(check int) []ReviewFinding {
	var out []ReviewFinding
	for _, f := range r.Findings {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

// actionableFindings returns MISMATCH and OMISSION findings (not STALE or OK).
func (r *ReviewReport) actionableFindings() []ReviewFinding {
	var out []ReviewFinding
	for _, f := range r.Findings {
		if f.Classification == ClassMismatch || f.Classification == ClassOmission {
			out = append(out, f)
		}
	}
	return out
}

// dashIfZero returns "-" if n is 0, otherwise the number as a string.
func dashIfZero(n int) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}

// ParseMismatchCount reads a review-findings.md file and extracts the mismatch count
// from the summary table. Returns 0 if the file doesn't exist or can't be parsed.
func ParseMismatchCount(content string) int {
	// Look for the Total row: | **Total** | **N** | **X** | ...
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "**Total**") {
			continue
		}
		// Split by | and find the mismatches column (3rd data column).
		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}
		// parts[0] = "", parts[1] = Total, parts[2] = findings, parts[3] = mismatches
		mismatchStr := strings.TrimSpace(parts[3])
		mismatchStr = strings.Trim(mismatchStr, "*")
		if mismatchStr == "-" {
			return 0
		}
		n := 0
		for _, c := range mismatchStr {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		return n
	}
	return 0
}
