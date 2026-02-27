package orchestrator

import (
	"fmt"
	"strings"
)

// Per-stage merge plans defining section order for each pipeline stage.
var (
	// Stage1MergePlan defines the section order for the design-pack stage.
	Stage1MergePlan = MergePlan{
		Strategy: MergeConcatenate,
		SectionOrder: []string{
			"assumptions", "platform-baseline", "data-model", "architecture",
			"features", "integrations", "security", "adrs", "pdrs", "prd",
			"data-lifecycle", "testing", "implementation-plan",
		},
	}

	// Stage2MergePlan defines the section order for the implementation-skeletons stage.
	Stage2MergePlan = MergePlan{
		Strategy: MergeConcatenate,
		SectionOrder: []string{"data-model-code", "interface-contracts", "documentation"},
	}

	// Stage3MergePlan defines the section order for the task-index stage.
	Stage3MergePlan = MergePlan{
		Strategy: MergeConcatenate,
		SectionOrder: []string{"progress", "dependencies", "directory-tree"},
	}
)

// Merger combines parallel agent outputs according to a MergePlan.
type Merger struct {
	plan MergePlan
}

// NewMerger creates a Merger with the given merge plan.
func NewMerger(plan MergePlan) *Merger {
	return &Merger{plan: plan}
}

// Merge combines sections according to the merge plan's section order.
// It validates that every section in the plan has a corresponding Section,
// checks for duplicate section names, sorts by plan order, and appends
// any extra sections not in the plan at the end. Sections are concatenated
// with "\n\n---\n\n" separators.
func (m *Merger) Merge(sections []Section) (string, error) {
	// Check for duplicate section names.
	seen := make(map[string]int, len(sections))
	for _, sec := range sections {
		seen[sec.Name]++
	}
	var duplicates []string
	for name, count := range seen {
		if count > 1 {
			duplicates = append(duplicates, fmt.Sprintf("%q (x%d)", name, count))
		}
	}
	if len(duplicates) > 0 {
		return "", fmt.Errorf("merge: duplicate section names: %s", strings.Join(duplicates, ", "))
	}

	// Build a lookup from section name to Section.
	byName := make(map[string]Section, len(sections))
	for _, sec := range sections {
		byName[sec.Name] = sec
	}

	// Validate that every section name in the plan has a corresponding Section.
	var missing []string
	for _, name := range m.plan.SectionOrder {
		if _, ok := byName[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("merge: missing sections required by plan: %s", strings.Join(missing, ", "))
	}

	// Build the ordered set: plan-ordered sections first, then extras.
	planned := make(map[string]bool, len(m.plan.SectionOrder))
	for _, name := range m.plan.SectionOrder {
		planned[name] = true
	}

	ordered := make([]string, 0, len(sections))
	for _, name := range m.plan.SectionOrder {
		ordered = append(ordered, byName[name].Content)
	}

	// Append extra sections not in the plan, preserving input order.
	for _, sec := range sections {
		if !planned[sec.Name] {
			ordered = append(ordered, sec.Content)
		}
	}

	return strings.Join(ordered, "\n\n---\n\n"), nil
}
