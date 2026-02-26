package orchestrator

import "context"

// Stage identifies a pipeline stage (0–4).
type Stage int

const (
	StageDevelopmentStandards    Stage = 0
	StageDesignPack              Stage = 1
	StageImplementationSkeletons Stage = 2
	StageTaskIndex               Stage = 3
	StageTaskSpecifications      Stage = 4
)

func (s Stage) String() string {
	names := [...]string{
		"development-standards",
		"design-pack",
		"implementation-skeletons",
		"task-index",
		"task-specifications",
	}
	if int(s) < len(names) {
		return names[s]
	}
	return "unknown"
}

// StageResult holds the output of a completed stage.
type StageResult struct {
	Stage     Stage
	FilePaths []string // output files written
	Sections  []Section
}

// Section is a named chunk of stage output produced by one agent.
type Section struct {
	Name    string // section identifier (e.g., "platform-baseline")
	Content string // markdown content
	Agent   string // which agent produced this section
}

// ProgressEvent is emitted to the user during pipeline execution.
type ProgressEvent struct {
	Stage   Stage
	Section string
	Status  ProgressStatus
	Message string
}

// ProgressStatus is the state of a section within a stage.
type ProgressStatus string

const (
	ProgressPending  ProgressStatus = "pending"
	ProgressWorking  ProgressStatus = "working"
	ProgressComplete ProgressStatus = "complete"
	ProgressFailed   ProgressStatus = "failed"
)

// Orchestrator coordinates the decomposition pipeline.
type Orchestrator interface {
	// RunStage executes a single pipeline stage.
	RunStage(ctx context.Context, stage Stage) (*StageResult, error)

	// RunPipeline executes stages from..to inclusive.
	RunPipeline(ctx context.Context, from, to Stage) ([]StageResult, error)

	// Progress returns a channel that emits progress events.
	Progress() <-chan ProgressEvent
}
