package graph

// --- Enums ---

// NodeKind classifies nodes in the code intelligence graph.
type NodeKind string

const (
	NodeKindFile    NodeKind = "file"
	NodeKindSymbol  NodeKind = "symbol"
	NodeKindCluster NodeKind = "cluster"
)

// SymbolKind classifies symbols within the code graph.
type SymbolKind string

const (
	SymbolKindFunction  SymbolKind = "function"
	SymbolKindClass     SymbolKind = "class"
	SymbolKindType      SymbolKind = "type"
	SymbolKindEnum      SymbolKind = "enum"
	SymbolKindInterface SymbolKind = "interface"
	SymbolKindVariable  SymbolKind = "variable"
	SymbolKindMethod    SymbolKind = "method"
)

// EdgeKind classifies relationships between nodes.
type EdgeKind string

const (
	EdgeKindDefines    EdgeKind = "DEFINES"
	EdgeKindImports    EdgeKind = "IMPORTS"
	EdgeKindCalls      EdgeKind = "CALLS"
	EdgeKindInherits   EdgeKind = "INHERITS"
	EdgeKindImplements EdgeKind = "IMPLEMENTS"
	EdgeKindBelongs    EdgeKind = "BELONGS"
)

// Language identifies a programming language for parsing.
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "typescript"
	LangPython     Language = "python"
	LangRust       Language = "rust"
)

// Tier1Languages are languages with full graph support (symbol extraction,
// call chains, dependency edges, cluster detection) tested in CI.
var Tier1Languages = []Language{LangGo, LangTypeScript, LangPython, LangRust}

// --- Models ---

// FileNode represents a source file in the code graph.
type FileNode struct {
	Path     string   `json:"path"`
	Language Language `json:"language"`
	LOC      int      `json:"loc"`
}

// SymbolNode represents a named symbol (function, class, type, etc.).
type SymbolNode struct {
	Name      string     `json:"name"`
	Kind      SymbolKind `json:"kind"`
	Exported  bool       `json:"exported"`
	FilePath  string     `json:"filePath"`
	StartLine int        `json:"startLine"`
	EndLine   int        `json:"endLine"`
}

// ClusterNode represents a group of tightly connected files.
type ClusterNode struct {
	Name          string   `json:"name"`
	CohesionScore float64  `json:"cohesionScore"`
	Members       []string `json:"members"` // file paths
}

// Edge represents a relationship between two nodes.
type Edge struct {
	SourceID string   `json:"sourceId"`
	TargetID string   `json:"targetId"`
	Kind     EdgeKind `json:"kind"`
}

// GraphStats summarizes a code intelligence graph.
type GraphStats struct {
	FileCount    int `json:"fileCount"`
	SymbolCount  int `json:"symbolCount"`
	ClusterCount int `json:"clusterCount"`
	EdgeCount    int `json:"edgeCount"`
}

// DependencyChain is an ordered sequence of nodes forming a dependency path.
type DependencyChain struct {
	Nodes []string `json:"nodes"` // node IDs in order
	Depth int      `json:"depth"`
}

// ImpactResult describes the blast radius of changing a set of files.
type ImpactResult struct {
	DirectlyAffected     []string `json:"directlyAffected"`     // files that import changed files
	TransitivelyAffected []string `json:"transitivelyAffected"` // full downstream closure
	RiskScore            float64  `json:"riskScore"`            // 0.0–1.0, based on fan-out
}
