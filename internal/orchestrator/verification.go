package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Severity indicates how critical a verification finding is.
type Severity string

const (
	SeverityCritical Severity = "critical" // Blocks stage progression
	SeverityWarning  Severity = "warning"  // Should fix, does not block
	SeverityInfo     Severity = "info"     // Informational observation
)

// VerificationFinding is a single issue found during verification.
type VerificationFinding struct {
	ID          string   `json:"id"`
	Severity    Severity `json:"severity"`
	Category    string   `json:"category"`    // "completeness", "coherence", "cross-stage", "methodology"
	Section     string   `json:"section"`     // Which section the issue was found in
	Description string   `json:"description"` // Human-readable description
	Suggestion  string   `json:"suggestion"`  // How to fix it
}

// VerificationReport is the structured output of a verification pass.
type VerificationReport struct {
	Stage     Stage                 `json:"stage"`
	Timestamp time.Time             `json:"timestamp"`
	Passed    bool                  `json:"passed"`
	Findings  []VerificationFinding `json:"findings"`
	Summary   string                `json:"summary"`
}

// HasCritical returns true if the report contains any critical-severity findings.
func (r *VerificationReport) HasCritical() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// Markdown formats the report as human-readable markdown.
func (r *VerificationReport) Markdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Verification Report: Stage %d (%s)\n\n", int(r.Stage), r.Stage)

	status := "PASSED"
	if !r.Passed {
		status = "FAILED"
	}
	critCount, warnCount, infoCount := r.countBySeverity()
	fmt.Fprintf(&b, "## Summary\n\n%s with %d critical, %d warnings, %d info findings.\n\n",
		status, critCount, warnCount, infoCount)

	if r.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", r.Summary)
	}

	writeFindingsByLevel := func(sev Severity, heading string) {
		findings := r.findingsBySeverity(sev)
		if len(findings) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", heading)
		for _, f := range findings {
			fmt.Fprintf(&b, "### %s: %s\n\n", f.ID, f.Description)
			fmt.Fprintf(&b, "- **Category**: %s\n", f.Category)
			if f.Section != "" {
				fmt.Fprintf(&b, "- **Section**: %s\n", f.Section)
			}
			if f.Suggestion != "" {
				fmt.Fprintf(&b, "- **Suggestion**: %s\n", f.Suggestion)
			}
			b.WriteString("\n")
		}
	}

	writeFindingsByLevel(SeverityCritical, "Critical Findings")
	writeFindingsByLevel(SeverityWarning, "Warnings")
	writeFindingsByLevel(SeverityInfo, "Info")

	return b.String()
}

func (r *VerificationReport) countBySeverity() (critical, warning, info int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		case SeverityInfo:
			info++
		}
	}
	return
}

func (r *VerificationReport) findingsBySeverity(sev Severity) []VerificationFinding {
	var out []VerificationFinding
	for _, f := range r.Findings {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	return out
}

// VerificationRule is a single check to apply to a stage output.
type VerificationRule struct {
	ID          string
	Category    string
	Severity    Severity
	Description string
	Check       func(content string, priorStages []StageResult) *VerificationFinding
}

// RunLocalVerification runs the appropriate rule set for the given stage
// against the stage output content and prior stage results. This is the
// graceful-degradation path when no A2A VerificationAgent is available.
func RunLocalVerification(stage Stage, content string, priorStages []StageResult) *VerificationReport {
	rules := RulesForStage(stage)
	rules = append(rules, CrossStageRules(stage)...)

	var findings []VerificationFinding
	for _, rule := range rules {
		if f := rule.Check(content, priorStages); f != nil {
			findings = append(findings, *f)
		}
	}

	passed := true
	for _, f := range findings {
		if f.Severity == SeverityCritical {
			passed = false
			break
		}
	}

	return &VerificationReport{
		Stage:     stage,
		Timestamp: time.Now(),
		Passed:    passed,
		Findings:  findings,
		Summary:   fmt.Sprintf("Verified stage %d (%s) with %d findings.", int(stage), stage, len(findings)),
	}
}

// RulesForStage returns the validation rules for a specific stage.
func RulesForStage(stage Stage) []VerificationRule {
	switch stage {
	case StageDevelopmentStandards:
		return stage0Rules()
	case StageDesignPack:
		return stage1Rules()
	case StageImplementationSkeletons:
		return stage2Rules()
	case StageTaskIndex:
		return stage3Rules()
	case StageTaskSpecifications:
		return stage4Rules()
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Stage 0 rules
// ---------------------------------------------------------------------------

// stage0RequiredSections are the 4 sections required in development standards.
var stage0RequiredSections = []struct {
	pattern string
	name    string
}{
	{`(?i)code\s+change\s+checklist`, "Code Change Checklist"},
	{`(?i)changeset\s+format`, "Changeset Format"},
	{`(?i)escalation`, "Escalation Guidance"},
	{`(?i)testing\s+guidance`, "Testing Guidance"},
}

func stage0Rules() []VerificationRule {
	var rules []VerificationRule
	for i, sec := range stage0RequiredSections {
		sec := sec
		rules = append(rules, VerificationRule{
			ID:          fmt.Sprintf("V-0.%02d", i+1),
			Category:    "completeness",
			Severity:    SeverityCritical,
			Description: fmt.Sprintf("Stage 0 must contain %q section", sec.name),
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !regexp.MustCompile(sec.pattern).MatchString(content) {
					return &VerificationFinding{
						ID:          fmt.Sprintf("V-0.%02d", i+1),
						Severity:    SeverityCritical,
						Category:    "completeness",
						Section:     "development-standards",
						Description: fmt.Sprintf("Missing required section: %s", sec.name),
						Suggestion:  fmt.Sprintf("Add a %q section to the development standards.", sec.name),
					}
				}
				return nil
			},
		})
	}
	return rules
}

// ---------------------------------------------------------------------------
// Stage 1 rules
// ---------------------------------------------------------------------------

// stage1RequiredSections are the sections required in the design pack.
var stage1RequiredSections = []struct {
	pattern string
	name    string
}{
	{`(?i)assumptions?\s*((&|and)\s*)?constraints?`, "Assumptions & Constraints"},
	{`(?i)(platform|tooling)\s+((&|and)\s*)?(baseline|stack)`, "Platform & Tooling Baseline"},
	{`(?i)data\s+model`, "Data Model"},
	{`(?i)architecture`, "Architecture"},
	{`(?i)features?`, "Features"},
	{`(?i)integration\s+points?`, "Integration Points"},
	{`(?i)security`, "Security & Privacy"},
}

var adrHeadingRe = regexp.MustCompile(`(?im)^#+\s+ADR[-\s]?\d`)
var pdrHeadingRe = regexp.MustCompile(`(?im)^#+\s+PDR[-\s]?\d`)

func stage1Rules() []VerificationRule {
	var rules []VerificationRule

	// Required sections.
	for i, sec := range stage1RequiredSections {
		sec := sec
		idx := i + 1
		rules = append(rules, VerificationRule{
			ID:          fmt.Sprintf("V-1.%02d", idx),
			Category:    "completeness",
			Severity:    SeverityCritical,
			Description: fmt.Sprintf("Stage 1 must contain %q section", sec.name),
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !regexp.MustCompile(sec.pattern).MatchString(content) {
					return &VerificationFinding{
						ID:          fmt.Sprintf("V-1.%02d", idx),
						Severity:    SeverityCritical,
						Category:    "completeness",
						Section:     "design-pack",
						Description: fmt.Sprintf("Missing required section: %s", sec.name),
						Suggestion:  fmt.Sprintf("Add a %q section to the design pack.", sec.name),
					}
				}
				return nil
			},
		})
	}

	nextID := len(stage1RequiredSections) + 1

	// ADR count >= 3.
	adrID := nextID
	rules = append(rules, VerificationRule{
		ID:          fmt.Sprintf("V-1.%02d", adrID),
		Category:    "methodology",
		Severity:    SeverityCritical,
		Description: "Stage 1 must contain at least 3 Architecture Decision Records",
		Check: func(content string, _ []StageResult) *VerificationFinding {
			count := len(adrHeadingRe.FindAllString(content, -1))
			if count < 3 {
				return &VerificationFinding{
					ID:          fmt.Sprintf("V-1.%02d", adrID),
					Severity:    SeverityCritical,
					Category:    "methodology",
					Section:     "adrs",
					Description: fmt.Sprintf("Found %d ADR(s), minimum is 3.", count),
					Suggestion:  "Add Architecture Decision Records covering persistence, framework, and data storage choices.",
				}
			}
			return nil
		},
	})
	nextID++

	// PDR count >= 2.
	pdrID := nextID
	rules = append(rules, VerificationRule{
		ID:          fmt.Sprintf("V-1.%02d", pdrID),
		Category:    "methodology",
		Severity:    SeverityWarning,
		Description: "Stage 1 should contain at least 2 Product Decision Records",
		Check: func(content string, _ []StageResult) *VerificationFinding {
			count := len(pdrHeadingRe.FindAllString(content, -1))
			if count < 2 {
				return &VerificationFinding{
					ID:          fmt.Sprintf("V-1.%02d", pdrID),
					Severity:    SeverityWarning,
					Category:    "methodology",
					Section:     "pdrs",
					Description: fmt.Sprintf("Found %d PDR(s), recommended minimum is 2.", count),
					Suggestion:  "Add Product Decision Records covering mental models and friction/simplicity choices.",
				}
			}
			return nil
		},
	})
	nextID++

	// Data model should contain relationship indicators.
	relID := nextID
	rules = append(rules, VerificationRule{
		ID:          fmt.Sprintf("V-1.%02d", relID),
		Category:    "completeness",
		Severity:    SeverityWarning,
		Description: "Data model should define relationships and cardinality",
		Check: func(content string, _ []StageResult) *VerificationFinding {
			relPatterns := regexp.MustCompile(`(?i)(one-to-many|many-to-many|one-to-one|has.many|belongs.to|foreign.key|cardinality|relationship)`)
			if !relPatterns.MatchString(content) {
				return &VerificationFinding{
					ID:          fmt.Sprintf("V-1.%02d", relID),
					Severity:    SeverityWarning,
					Category:    "completeness",
					Section:     "data-model",
					Description: "Data model does not appear to define entity relationships or cardinality.",
					Suggestion:  "Add relationship definitions (one-to-many, many-to-many) and cardinality for each entity.",
				}
			}
			return nil
		},
	})

	return rules
}

// ---------------------------------------------------------------------------
// Stage 2 rules
// ---------------------------------------------------------------------------

var codeBlockPresenceRe = regexp.MustCompile("(?s)```[a-zA-Z]*\n.+?```")

func stage2Rules() []VerificationRule {
	return []VerificationRule{
		{
			ID:          "V-2.01",
			Category:    "completeness",
			Severity:    SeverityCritical,
			Description: "Stage 2 must contain code blocks",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !codeBlockPresenceRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-2.01",
						Severity:    SeverityCritical,
						Category:    "completeness",
						Section:     "implementation-skeletons",
						Description: "No code blocks found. Stage 2 must contain compilable type definitions.",
						Suggestion:  "Add fenced code blocks with type definitions for all entities from Stage 1.",
					}
				}
				return nil
			},
		},
		{
			ID:          "V-2.02",
			Category:    "completeness",
			Severity:    SeverityWarning,
			Description: "Stage 2 should define serialization format",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				serRe := regexp.MustCompile(`(?i)(json|xml|protobuf|marshal|serialize|encoding|serde)`)
				if !serRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-2.02",
						Severity:    SeverityWarning,
						Category:    "completeness",
						Section:     "implementation-skeletons",
						Description: "No serialization format mentioned. Skeleton types should define how data is serialized.",
						Suggestion:  "Add JSON tags, serialization annotations, or document the wire format for each type.",
					}
				}
				return nil
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Stage 3 rules
// ---------------------------------------------------------------------------

var milestoneHeadingRe = regexp.MustCompile(`(?im)^#+\s+M\d+|(?im)milestone\s+\d+`)
var criticalPathRe = regexp.MustCompile(`(?i)critical\s+path`)
var directoryTreeRe = regexp.MustCompile(`(?i)directory\s+tree|target\s+directory|file\s+tree`)
var dependencyGraphRe = regexp.MustCompile(`(?i)dependency\s+graph|milestone\s+depend`)

func stage3Rules() []VerificationRule {
	return []VerificationRule{
		{
			ID:          "V-3.01",
			Category:    "completeness",
			Severity:    SeverityCritical,
			Description: "Stage 3 must contain milestone definitions",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !milestoneHeadingRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-3.01",
						Severity:    SeverityCritical,
						Category:    "completeness",
						Section:     "task-index",
						Description: "No milestone definitions found.",
						Suggestion:  "Add milestone headings (e.g., ## M1: ...) with task counts and descriptions.",
					}
				}
				return nil
			},
		},
		{
			ID:          "V-3.02",
			Category:    "completeness",
			Severity:    SeverityCritical,
			Description: "Stage 3 must contain a dependency graph",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !dependencyGraphRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-3.02",
						Severity:    SeverityCritical,
						Category:    "completeness",
						Section:     "dependencies",
						Description: "No milestone dependency graph found.",
						Suggestion:  "Add a milestone dependency graph showing sequential and parallel execution paths.",
					}
				}
				return nil
			},
		},
		{
			ID:          "V-3.03",
			Category:    "completeness",
			Severity:    SeverityWarning,
			Description: "Stage 3 should identify the critical path",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !criticalPathRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-3.03",
						Severity:    SeverityWarning,
						Category:    "completeness",
						Section:     "dependencies",
						Description: "No critical path identified in the dependency graph.",
						Suggestion:  "Identify the critical path through the milestone dependency graph.",
					}
				}
				return nil
			},
		},
		{
			ID:          "V-3.04",
			Category:    "completeness",
			Severity:    SeverityCritical,
			Description: "Stage 3 must contain a target directory tree",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !directoryTreeRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-3.04",
						Severity:    SeverityCritical,
						Category:    "completeness",
						Section:     "directory-tree",
						Description: "No target directory tree found.",
						Suggestion:  "Add a directory tree showing every file to be created/modified/deleted, annotated with milestone.",
					}
				}
				return nil
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Stage 4 rules
// ---------------------------------------------------------------------------

var taskIDRe = regexp.MustCompile(`T-\d+\.\d+`)
var acceptanceCriteriaRe = regexp.MustCompile(`(?i)(acceptance|done\s+when|criteria|✅)`)
var fileActionRe = regexp.MustCompile(`(?i)\b(CREATE|MODIFY|DELETE)\b`)
var dependsOnRe = regexp.MustCompile(`(?i)depends?\s+on`)

func stage4Rules() []VerificationRule {
	return []VerificationRule{
		{
			ID:          "V-4.01",
			Category:    "completeness",
			Severity:    SeverityCritical,
			Description: "Stage 4 must contain task IDs in T-MM.SS format",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !taskIDRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-4.01",
						Severity:    SeverityCritical,
						Category:    "completeness",
						Section:     "task-specifications",
						Description: "No task IDs found in T-MM.SS format.",
						Suggestion:  "Add task IDs following the T-{MM}.{SS} format (e.g., T-01.01).",
					}
				}
				return nil
			},
		},
		{
			ID:          "V-4.02",
			Category:    "methodology",
			Severity:    SeverityCritical,
			Description: "Stage 4 tasks must have acceptance criteria",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				// Check that acceptance criteria appear at least once per task-like section.
				taskCount := len(taskIDRe.FindAllString(content, -1))
				acceptanceCount := len(acceptanceCriteriaRe.FindAllString(content, -1))
				if taskCount > 0 && acceptanceCount == 0 {
					return &VerificationFinding{
						ID:          "V-4.02",
						Severity:    SeverityCritical,
						Category:    "methodology",
						Section:     "task-specifications",
						Description: fmt.Sprintf("Found %d tasks but no acceptance criteria sections.", taskCount),
						Suggestion:  "Add binary, testable acceptance criteria to every task.",
					}
				}
				// Warn if significantly fewer acceptance sections than tasks.
				if taskCount > 0 && acceptanceCount < taskCount/2 {
					return &VerificationFinding{
						ID:          "V-4.02",
						Severity:    SeverityWarning,
						Category:    "methodology",
						Section:     "task-specifications",
						Description: fmt.Sprintf("Found %d tasks but only %d acceptance criteria sections. Some tasks may be missing criteria.", taskCount, acceptanceCount),
						Suggestion:  "Ensure every task has its own acceptance criteria section.",
					}
				}
				return nil
			},
		},
		{
			ID:          "V-4.03",
			Category:    "completeness",
			Severity:    SeverityWarning,
			Description: "Stage 4 tasks should specify file actions",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !fileActionRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-4.03",
						Severity:    SeverityWarning,
						Category:    "completeness",
						Section:     "task-specifications",
						Description: "No file actions (CREATE/MODIFY/DELETE) found in task specifications.",
						Suggestion:  "Each task should specify file path + action (CREATE, MODIFY, or DELETE).",
					}
				}
				return nil
			},
		},
		{
			ID:          "V-4.04",
			Category:    "completeness",
			Severity:    SeverityWarning,
			Description: "Stage 4 tasks should specify dependencies",
			Check: func(content string, _ []StageResult) *VerificationFinding {
				if !dependsOnRe.MatchString(content) {
					return &VerificationFinding{
						ID:          "V-4.04",
						Severity:    SeverityWarning,
						Category:    "completeness",
						Section:     "task-specifications",
						Description: "No dependency declarations found in task specifications.",
						Suggestion:  "Each task should declare dependencies on other task IDs or \"None\".",
					}
				}
				return nil
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Cross-stage rules
// ---------------------------------------------------------------------------

// CrossStageRules returns rules that validate coherence between stages.
// These rules check that references in stage N resolve against stage N-1 outputs.
func CrossStageRules(stage Stage) []VerificationRule {
	switch stage {
	case StageImplementationSkeletons:
		return crossStage2Rules()
	case StageTaskIndex:
		return crossStage3Rules()
	case StageTaskSpecifications:
		return crossStage4Rules()
	default:
		return nil
	}
}

// crossStage2Rules checks Stage 2 against Stage 1.
func crossStage2Rules() []VerificationRule {
	return []VerificationRule{
		{
			ID:          "V-X2.01",
			Category:    "cross-stage",
			Severity:    SeverityWarning,
			Description: "Stage 2 types should reference entities from Stage 1 data model",
			Check: func(content string, priorStages []StageResult) *VerificationFinding {
				stage1 := findStageContent(priorStages, StageDesignPack)
				if stage1 == "" {
					return nil // Can't cross-check without Stage 1
				}
				// Extract entity-like names from Stage 1 data model section.
				entities := extractEntityNames(stage1)
				if len(entities) == 0 {
					return nil
				}
				// Check how many Stage 1 entities appear in Stage 2.
				missing := 0
				var missingNames []string
				for _, entity := range entities {
					if !strings.Contains(strings.ToLower(content), strings.ToLower(entity)) {
						missing++
						missingNames = append(missingNames, entity)
					}
				}
				if missing > 0 && missing == len(entities) {
					return &VerificationFinding{
						ID:          "V-X2.01",
						Severity:    SeverityCritical,
						Category:    "cross-stage",
						Section:     "implementation-skeletons",
						Description: fmt.Sprintf("None of the %d entities from Stage 1 data model appear in Stage 2.", len(entities)),
						Suggestion:  fmt.Sprintf("Add type definitions for: %s", strings.Join(missingNames, ", ")),
					}
				}
				if missing > 0 {
					return &VerificationFinding{
						ID:          "V-X2.01",
						Severity:    SeverityWarning,
						Category:    "cross-stage",
						Section:     "implementation-skeletons",
						Description: fmt.Sprintf("%d of %d Stage 1 entities missing from Stage 2: %s", missing, len(entities), strings.Join(missingNames, ", ")),
						Suggestion:  "Add type definitions for the missing entities.",
					}
				}
				return nil
			},
		},
	}
}

// crossStage3Rules checks Stage 3 against Stage 1.
func crossStage3Rules() []VerificationRule {
	return []VerificationRule{
		{
			ID:          "V-X3.01",
			Category:    "cross-stage",
			Severity:    SeverityWarning,
			Description: "Stage 3 milestones should cover Stage 1 features",
			Check: func(content string, priorStages []StageResult) *VerificationFinding {
				stage1 := findStageContent(priorStages, StageDesignPack)
				if stage1 == "" {
					return nil
				}
				// Extract feature-like items from Stage 1.
				features := extractFeatureNames(stage1)
				if len(features) == 0 {
					return nil
				}
				missing := 0
				for _, feature := range features {
					if !strings.Contains(strings.ToLower(content), strings.ToLower(feature)) {
						missing++
					}
				}
				if missing > len(features)/2 {
					return &VerificationFinding{
						ID:          "V-X3.01",
						Severity:    SeverityWarning,
						Category:    "cross-stage",
						Section:     "task-index",
						Description: fmt.Sprintf("%d of %d Stage 1 features may not be covered by milestones.", missing, len(features)),
						Suggestion:  "Verify that every feature from Stage 1 maps to at least one milestone.",
					}
				}
				return nil
			},
		},
	}
}

// crossStage4Rules checks Stage 4 against Stage 3.
func crossStage4Rules() []VerificationRule {
	return []VerificationRule{
		{
			ID:          "V-X4.01",
			Category:    "cross-stage",
			Severity:    SeverityWarning,
			Description: "Stage 4 tasks should cover files from Stage 3 directory tree",
			Check: func(content string, priorStages []StageResult) *VerificationFinding {
				stage3 := findStageContent(priorStages, StageTaskIndex)
				if stage3 == "" {
					return nil
				}
				// Extract file paths from Stage 3 directory tree.
				files := extractTreeFiles(stage3)
				if len(files) == 0 {
					return nil
				}
				missing := 0
				for _, file := range files {
					// Check for the filename (not full path) in Stage 4 content.
					base := file
					if idx := strings.LastIndex(file, "/"); idx >= 0 {
						base = file[idx+1:]
					}
					if !strings.Contains(content, base) {
						missing++
					}
				}
				if missing > len(files)/3 {
					return &VerificationFinding{
						ID:          "V-X4.01",
						Severity:    SeverityWarning,
						Category:    "cross-stage",
						Section:     "task-specifications",
						Description: fmt.Sprintf("%d of %d files from Stage 3 directory tree may not have corresponding tasks.", missing, len(files)),
						Suggestion:  "Ensure every file in Stage 3's directory tree has at least one task in Stage 4.",
					}
				}
				return nil
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers for cross-stage content extraction
// ---------------------------------------------------------------------------

// findStageContent returns the concatenated content of all sections for a
// given stage from prior stage results.
func findStageContent(priorStages []StageResult, target Stage) string {
	for _, sr := range priorStages {
		if sr.Stage == target {
			var parts []string
			for _, sec := range sr.Sections {
				parts = append(parts, sec.Content)
			}
			return strings.Join(parts, "\n")
		}
	}
	return ""
}

// entityNameRe matches capitalized words that look like entity names in data model
// sections (PascalCase words of 3+ chars that aren't common prose words).
var entityNameRe = regexp.MustCompile(`\b([A-Z][a-z]+(?:[A-Z][a-z]+)+)\b`)

// extractEntityNames finds PascalCase names from the data model section of content.
func extractEntityNames(content string) []string {
	// Focus on the data model section if identifiable.
	lower := strings.ToLower(content)
	start := strings.Index(lower, "data model")
	if start < 0 {
		start = 0
	}
	section := content[start:]
	// Limit search to a reasonable window.
	if len(section) > 5000 {
		section = section[:5000]
	}

	matches := entityNameRe.FindAllString(section, -1)
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	return unique
}

// featureItemRe matches markdown list items that look like feature declarations.
var featureItemRe = regexp.MustCompile(`(?m)^[-*]\s+\*?\*?(.+?)\*?\*?\s*[-–—:]`)

// extractFeatureNames finds feature names from the features section.
func extractFeatureNames(content string) []string {
	lower := strings.ToLower(content)
	start := strings.Index(lower, "feature")
	if start < 0 {
		return nil
	}
	section := content[start:]
	if len(section) > 5000 {
		section = section[:5000]
	}

	matches := featureItemRe.FindAllStringSubmatch(section, -1)
	var features []string
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		if len(name) > 2 && len(name) < 80 {
			features = append(features, name)
		}
	}
	return features
}

// treeFileRe matches file paths in a directory tree listing.
var treeFileRe = regexp.MustCompile(`(?m)[├└│\s]*(\S+\.\w{1,10})\s`)

// extractTreeFiles finds file paths from a directory tree section.
func extractTreeFiles(content string) []string {
	lower := strings.ToLower(content)
	start := strings.Index(lower, "directory tree")
	if start < 0 {
		start = strings.Index(lower, "file tree")
	}
	if start < 0 {
		start = 0
	}
	section := content[start:]
	if len(section) > 10000 {
		section = section[:10000]
	}

	matches := treeFileRe.FindAllStringSubmatch(section, -1)
	seen := make(map[string]bool)
	var files []string
	for _, m := range matches {
		path := m[1]
		if !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	return files
}
