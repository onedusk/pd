package graph

import (
	"context"
	"strings"
	"sync"
)

// Compile-time assertion: *MemStore satisfies Store.
var _ Store = (*MemStore)(nil)

// MemStore implements Store using Go maps. Thread-safe via sync.RWMutex.
type MemStore struct {
	mu       sync.RWMutex
	files    map[string]FileNode
	symbols  map[string]SymbolNode // key: "filePath:name"
	edges    []Edge
	clusters []ClusterNode
}

// NewMemStore returns an initialized MemStore ready for use.
func NewMemStore() *MemStore {
	return &MemStore{
		files:   make(map[string]FileNode),
		symbols: make(map[string]SymbolNode),
	}
}

// symbolKey builds the composite lookup key for a symbol.
func symbolKey(filePath, name string) string {
	return filePath + ":" + name
}

// InitSchema is a no-op for the in-memory store.
func (m *MemStore) InitSchema(_ context.Context) error {
	return nil
}

// AddFile stores a file node keyed by its path.
func (m *MemStore) AddFile(_ context.Context, node FileNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[node.Path] = node
	return nil
}

// AddSymbol stores a symbol node keyed by "filePath:name".
func (m *MemStore) AddSymbol(_ context.Context, node SymbolNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.symbols[symbolKey(node.FilePath, node.Name)] = node
	return nil
}

// AddCluster appends a cluster to the internal slice.
func (m *MemStore) AddCluster(_ context.Context, node ClusterNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusters = append(m.clusters, node)
	return nil
}

// AddEdge appends an edge to the internal slice.
func (m *MemStore) AddEdge(_ context.Context, edge Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.edges = append(m.edges, edge)
	return nil
}

// GetFile returns the file node for the given path, or nil if not found.
func (m *MemStore) GetFile(_ context.Context, path string) (*FileNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[path]
	if !ok {
		return nil, nil
	}
	return &f, nil
}

// GetSymbol returns the symbol for the given file path and name, or nil if not found.
func (m *MemStore) GetSymbol(_ context.Context, filePath, name string) (*SymbolNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.symbols[symbolKey(filePath, name)]
	if !ok {
		return nil, nil
	}
	return &s, nil
}

// QuerySymbols returns symbols whose name contains query (case-insensitive),
// up to limit results. A limit <= 0 returns all matches.
func (m *MemStore) QuerySymbols(_ context.Context, query string, limit int) ([]SymbolNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lowerQuery := strings.ToLower(query)
	var results []SymbolNode
	for _, sym := range m.symbols {
		if strings.Contains(strings.ToLower(sym.Name), lowerQuery) {
			results = append(results, sym)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// GetDependencies performs a BFS on edges from nodeID in the given direction,
// up to maxDepth hops. It returns one DependencyChain per reachable node.
func (m *MemStore) GetDependencies(_ context.Context, nodeID string, direction Direction, maxDepth int) ([]DependencyChain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if maxDepth <= 0 {
		return nil, nil
	}

	// BFS state: each entry tracks the path from nodeID to the current node.
	type bfsEntry struct {
		id   string
		path []string
	}

	visited := map[string]bool{nodeID: true}
	queue := []bfsEntry{{id: nodeID, path: []string{nodeID}}}
	var chains []DependencyChain

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var nextQueue []bfsEntry
		for _, entry := range queue {
			neighbors := m.neighbors(entry.id, direction)
			for _, nb := range neighbors {
				if visited[nb] {
					continue
				}
				visited[nb] = true
				newPath := make([]string, len(entry.path), len(entry.path)+1)
				copy(newPath, entry.path)
				newPath = append(newPath, nb)
				chains = append(chains, DependencyChain{
					Nodes: newPath,
					Depth: len(newPath) - 1,
				})
				nextQueue = append(nextQueue, bfsEntry{id: nb, path: newPath})
			}
		}
		queue = nextQueue
	}

	return chains, nil
}

// neighbors returns IDs reachable from id in one hop along the given direction.
func (m *MemStore) neighbors(id string, direction Direction) []string {
	var result []string
	for _, e := range m.edges {
		switch direction {
		case DirectionDownstream:
			// downstream: id is a dependency of others -> follow edges where SourceID matches
			if e.SourceID == id {
				result = append(result, e.TargetID)
			}
		case DirectionUpstream:
			// upstream: id depends on others -> follow edges where TargetID matches
			if e.TargetID == id {
				result = append(result, e.SourceID)
			}
		}
	}
	return result
}

// AssessImpact computes the blast radius of changing the given files.
// It follows IMPORTS edges to find direct and transitive dependents.
func (m *MemStore) AssessImpact(_ context.Context, changedFiles []string) (*ImpactResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	changedSet := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changedSet[f] = true
	}

	// DirectlyAffected: files that IMPORT any changed file.
	// An IMPORTS edge with SourceID=A, TargetID=B means "A imports B".
	// So files importing a changed file have SourceID as the affected file
	// and TargetID as the changed file.
	directSet := make(map[string]bool)
	for _, e := range m.edges {
		if e.Kind != EdgeKindImports {
			continue
		}
		if changedSet[e.TargetID] && !changedSet[e.SourceID] {
			directSet[e.SourceID] = true
		}
	}

	directlyAffected := setToSlice(directSet)

	// TransitivelyAffected: expand from directly affected files iteratively.
	// For each affected file, find files that import it, until no new files.
	allAffected := make(map[string]bool)
	for k := range directSet {
		allAffected[k] = true
	}

	frontier := make(map[string]bool)
	for k := range directSet {
		frontier[k] = true
	}

	for len(frontier) > 0 {
		nextFrontier := make(map[string]bool)
		for _, e := range m.edges {
			if e.Kind != EdgeKindImports {
				continue
			}
			// e.SourceID imports e.TargetID; if TargetID is in frontier,
			// SourceID is transitively affected.
			if frontier[e.TargetID] && !changedSet[e.SourceID] && !allAffected[e.SourceID] {
				allAffected[e.SourceID] = true
				nextFrontier[e.SourceID] = true
			}
		}
		frontier = nextFrontier
	}

	transitivelyAffected := setToSlice(allAffected)

	var riskScore float64
	if len(m.files) > 0 {
		riskScore = float64(len(transitivelyAffected)) / float64(len(m.files))
	}

	return &ImpactResult{
		DirectlyAffected:     directlyAffected,
		TransitivelyAffected: transitivelyAffected,
		RiskScore:            riskScore,
	}, nil
}

// GetClusters returns all stored clusters.
func (m *MemStore) GetClusters(_ context.Context) ([]ClusterNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ClusterNode, len(m.clusters))
	copy(out, m.clusters)
	return out, nil
}

// GetAllEdges returns a copy of all edges in the store.
func (m *MemStore) GetAllEdges(_ context.Context) ([]Edge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Edge, len(m.edges))
	copy(out, m.edges)
	return out, nil
}

// Stats returns counts of all node and edge types in the graph.
func (m *MemStore) Stats(_ context.Context) (*GraphStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &GraphStats{
		FileCount:    len(m.files),
		SymbolCount:  len(m.symbols),
		ClusterCount: len(m.clusters),
		EdgeCount:    len(m.edges),
	}, nil
}

// Close is a no-op for the in-memory store.
func (m *MemStore) Close() error {
	return nil
}

// setToSlice converts a string bool map to a slice.
func setToSlice(s map[string]bool) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out
}
