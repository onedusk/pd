package graph

import (
	"context"
	"io"
)

// Store is the interface for the code intelligence graph backend.
// Implementations: KuzuStore (production), MemoryStore (testing).
// All graph DB access goes through this interface (ADR-006).
type Store interface {
	io.Closer

	// Schema setup — called once before any data is inserted.
	InitSchema(ctx context.Context) error

	// Write operations.
	AddFile(ctx context.Context, node FileNode) error
	AddSymbol(ctx context.Context, node SymbolNode) error
	AddCluster(ctx context.Context, node ClusterNode) error
	AddEdge(ctx context.Context, edge Edge) error

	// Read operations.
	GetFile(ctx context.Context, path string) (*FileNode, error)
	GetSymbol(ctx context.Context, filePath, name string) (*SymbolNode, error)
	QuerySymbols(ctx context.Context, query string, limit int) ([]SymbolNode, error)

	// Graph traversal.
	GetDependencies(ctx context.Context, nodeID string, direction Direction, maxDepth int) ([]DependencyChain, error)
	AssessImpact(ctx context.Context, changedFiles []string) (*ImpactResult, error)
	GetClusters(ctx context.Context) ([]ClusterNode, error)

	// Stats.
	Stats(ctx context.Context) (*GraphStats, error)
}

// Direction controls dependency traversal direction.
type Direction string

const (
	DirectionUpstream   Direction = "upstream"   // what does this depend on?
	DirectionDownstream Direction = "downstream" // what depends on this?
)
