package mcptools

// --- MCP Tool Types for the decompose server mode (--serve-mcp) ---
// These tools are exposed when the binary runs as an MCP server for Claude Code.
// They allow the /decompose skill to call structured tools instead of shelling out.

// RunStageInput is the input for the run_stage MCP tool.
type RunStageInput struct {
	Name        string `json:"name" jsonschema:"decomposition name (kebab-case)"`
	Stage       int    `json:"stage" jsonschema:"pipeline stage to run (0-4)"`
	ProjectRoot string `json:"projectRoot,omitempty" jsonschema:"path to the target project (default: cwd)"`
}

// RunStageOutput is the result of the run_stage MCP tool.
type RunStageOutput struct {
	FilesWritten []string `json:"filesWritten"`
	Stage        int      `json:"stage"`
	Status       string   `json:"status"` // "completed" or "failed"
	Message      string   `json:"message,omitempty"`
}

// GetStatusInput is the input for the get_status MCP tool.
type GetStatusInput struct {
	Name string `json:"name" jsonschema:"decomposition name"`
}

// GetStatusOutput is the result of the get_status MCP tool.
type GetStatusOutput struct {
	Name            string `json:"name"`
	CompletedStages []int  `json:"completedStages"`
	NextStage       int    `json:"nextStage"`
	CapabilityLevel string `json:"capabilityLevel"`
}

// ListDecompositionsInput is the input for the list_decompositions MCP tool.
type ListDecompositionsInput struct {
	ProjectRoot string `json:"projectRoot,omitempty" jsonschema:"path to the project (default: cwd)"`
}

// ListDecompositionsOutput is the result of the list_decompositions MCP tool.
type ListDecompositionsOutput struct {
	Decompositions []DecompositionSummary `json:"decompositions"`
	HasStage0      bool                   `json:"hasStage0"`
}

// DecompositionSummary is a brief overview of one decomposition.
type DecompositionSummary struct {
	Name            string `json:"name"`
	CompletedStages []int  `json:"completedStages"`
	NextStage       int    `json:"nextStage"`
}
