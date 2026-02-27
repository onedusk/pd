package mcptools

import "github.com/dusk-indust/decompose/internal/graph"

// --- MCP Tool Input Types ---
// These structs define the JSON schema for each MCP tool's input.
// The MCP Go SDK auto-generates JSON schemas from struct tags.

// BuildGraphInput is the input for the build_graph MCP tool.
type BuildGraphInput struct {
	RepoPath    string   `json:"repoPath" jsonschema:"the absolute path to the repository to index"`
	Languages   []string `json:"languages,omitempty" jsonschema:"languages to index (default: tier-1). Values: go, typescript, python, rust"`
	ExcludeDirs []string `json:"excludeDirs,omitempty" jsonschema:"directories to exclude from indexing (e.g. vendor, node_modules)"`
}

// BuildGraphOutput is the result of the build_graph MCP tool.
type BuildGraphOutput struct {
	Stats graph.GraphStats `json:"stats"`
}

// QuerySymbolsInput is the input for the query_symbols MCP tool.
type QuerySymbolsInput struct {
	Query string `json:"query" jsonschema:"search query for symbol names (substring match)"`
	Kind  string `json:"kind,omitempty" jsonschema:"filter by symbol kind: function, class, type, enum, interface, variable, method"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results (default: 20)"`
}

// QuerySymbolsOutput is the result of the query_symbols MCP tool.
type QuerySymbolsOutput struct {
	Symbols []graph.SymbolNode `json:"symbols"`
	Total   int                `json:"total"`
}

// GetDependenciesInput is the input for the get_dependencies MCP tool.
type GetDependenciesInput struct {
	NodeID    string `json:"nodeId" jsonschema:"file path or qualified symbol name"`
	Direction string `json:"direction,omitempty" jsonschema:"upstream (what it depends on) or downstream (what depends on it). Default: downstream"`
	MaxDepth  int    `json:"maxDepth,omitempty" jsonschema:"maximum traversal depth (default: 5)"`
}

// GetDependenciesOutput is the result of the get_dependencies MCP tool.
type GetDependenciesOutput struct {
	Chains []graph.DependencyChain `json:"chains"`
}

// AssessImpactInput is the input for the assess_impact MCP tool.
type AssessImpactInput struct {
	ChangedFiles []string `json:"changedFiles" jsonschema:"list of file paths that will be modified"`
}

// AssessImpactOutput is the result of the assess_impact MCP tool.
type AssessImpactOutput struct {
	Impact graph.ImpactResult `json:"impact"`
}

// GetClustersInput is the input for the get_clusters MCP tool.
type GetClustersInput struct{}

// GetClustersOutput is the result of the get_clusters MCP tool.
type GetClustersOutput struct {
	Clusters []graph.ClusterNode `json:"clusters"`
}
