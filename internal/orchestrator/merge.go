package orchestrator

// MergeStrategy defines how parallel agent outputs are combined.
type MergeStrategy string

const (
	// MergeConcatenate joins sections in template order.
	MergeConcatenate MergeStrategy = "concatenate"
)

// MergePlan describes how to combine sections from parallel agents.
type MergePlan struct {
	Strategy     MergeStrategy
	SectionOrder []string // section names in template order
}

// CoherenceIssue is a contradiction found during post-merge validation.
type CoherenceIssue struct {
	SectionA    string // first conflicting section
	SectionB    string // second conflicting section
	Description string // what the contradiction is
}
