//go:build cgo

package graph

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore creates a fresh in-memory KuzuStore with an initialized schema.
// It registers a cleanup function to close the store when the test finishes.
func newTestStore(t *testing.T) *KuzuStore {
	t.Helper()
	s, err := NewKuzuStore()
	require.NoError(t, err, "NewKuzuStore should not fail")
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()
	require.NoError(t, s.InitSchema(ctx), "InitSchema should not fail")
	return s
}

// sorted returns a sorted copy of the given string slice so that assertions
// are deterministic regardless of map iteration order.
func sorted(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestKuzuStore_InitSchema(t *testing.T) {
	s, err := NewKuzuStore()
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()

	// First call creates the tables.
	require.NoError(t, s.InitSchema(ctx))

	// Second call should be idempotent (IF NOT EXISTS).
	require.NoError(t, s.InitSchema(ctx))
}

func TestKuzuStore_FileRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	file := FileNode{
		Path:     "internal/graph/kuzustore.go",
		Language: LangGo,
		LOC:      420,
	}

	require.NoError(t, s.AddFile(ctx, file))

	got, err := s.GetFile(ctx, file.Path)
	require.NoError(t, err)
	require.NotNil(t, got, "GetFile should return a non-nil result")

	assert.Equal(t, file.Path, got.Path)
	assert.Equal(t, file.Language, got.Language)
	assert.Equal(t, file.LOC, got.LOC)
}

func TestKuzuStore_GetFile_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetFile(ctx, "nonexistent.go")
	require.NoError(t, err)
	assert.Nil(t, got, "GetFile should return nil for a missing file")
}

func TestKuzuStore_SymbolRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sym := SymbolNode{
		Name:      "NewKuzuStore",
		Kind:      SymbolKindFunction,
		Exported:  true,
		FilePath:  "internal/graph/kuzustore.go",
		StartLine: 24,
		EndLine:   36,
	}

	require.NoError(t, s.AddSymbol(ctx, sym))

	got, err := s.GetSymbol(ctx, sym.FilePath, sym.Name)
	require.NoError(t, err)
	require.NotNil(t, got, "GetSymbol should return a non-nil result")

	assert.Equal(t, sym.Name, got.Name)
	assert.Equal(t, sym.Kind, got.Kind)
	assert.Equal(t, sym.Exported, got.Exported)
	assert.Equal(t, sym.FilePath, got.FilePath)
	assert.Equal(t, sym.StartLine, got.StartLine)
	assert.Equal(t, sym.EndLine, got.EndLine)
}

func TestKuzuStore_GetSymbol_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.GetSymbol(ctx, "no-file.go", "NoSuchSymbol")
	require.NoError(t, err)
	assert.Nil(t, got, "GetSymbol should return nil for a missing symbol")
}

func TestKuzuStore_QuerySymbols(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	symbols := []SymbolNode{
		{Name: "NewKuzuStore", Kind: SymbolKindFunction, Exported: true, FilePath: "a.go", StartLine: 1, EndLine: 10},
		{Name: "NewMemStore", Kind: SymbolKindFunction, Exported: true, FilePath: "b.go", StartLine: 1, EndLine: 8},
		{Name: "initSchema", Kind: SymbolKindFunction, Exported: false, FilePath: "a.go", StartLine: 12, EndLine: 20},
		{Name: "symbolKey", Kind: SymbolKindFunction, Exported: false, FilePath: "b.go", StartLine: 10, EndLine: 12},
	}
	for _, sym := range symbols {
		require.NoError(t, s.AddSymbol(ctx, sym))
	}

	t.Run("substring match", func(t *testing.T) {
		// "New" should match NewKuzuStore and NewMemStore.
		results, err := s.QuerySymbols(ctx, "New", 10)
		require.NoError(t, err)
		assert.Len(t, results, 2)

		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.Name
		}
		sort.Strings(names)
		assert.Equal(t, []string{"NewKuzuStore", "NewMemStore"}, names)
	})

	t.Run("limit respected", func(t *testing.T) {
		// Query with a broad match but limit=1.
		results, err := s.QuerySymbols(ctx, "New", 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("no match", func(t *testing.T) {
		results, err := s.QuerySymbols(ctx, "ZZZnope", 10)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestKuzuStore_AddEdge_Defines(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create the file and symbol nodes first.
	require.NoError(t, s.AddFile(ctx, FileNode{Path: "main.go", Language: LangGo, LOC: 50}))
	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "main", Kind: SymbolKindFunction, Exported: false,
		FilePath: "main.go", StartLine: 1, EndLine: 10,
	}))

	// DEFINES edge: File -> Symbol. TargetID must be "filePath:name".
	edge := Edge{
		SourceID: "main.go",
		TargetID: "main.go:main",
		Kind:     EdgeKindDefines,
	}
	require.NoError(t, s.AddEdge(ctx, edge))

	// Verify the edge exists via stats.
	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EdgeCount)
}

func TestKuzuStore_AddEdge_Calls(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create two symbols.
	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "caller", Kind: SymbolKindFunction, Exported: true,
		FilePath: "a.go", StartLine: 1, EndLine: 5,
	}))
	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "callee", Kind: SymbolKindFunction, Exported: true,
		FilePath: "b.go", StartLine: 1, EndLine: 5,
	}))

	// CALLS edge: Symbol -> Symbol. Both IDs use "filePath:name".
	edge := Edge{
		SourceID: "a.go:caller",
		TargetID: "b.go:callee",
		Kind:     EdgeKindCalls,
	}
	require.NoError(t, s.AddEdge(ctx, edge))

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EdgeCount)
}

func TestKuzuStore_Dependencies_Downstream(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create a chain: A imports B, B imports C.
	// DirectionDownstream from A follows outgoing IMPORTS edges.
	files := []FileNode{
		{Path: "a.go", Language: LangGo, LOC: 10},
		{Path: "b.go", Language: LangGo, LOC: 20},
		{Path: "c.go", Language: LangGo, LOC: 30},
	}
	for _, f := range files {
		require.NoError(t, s.AddFile(ctx, f))
	}

	// A imports B.
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "b.go", Kind: EdgeKindImports}))
	// B imports C.
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "b.go", TargetID: "c.go", Kind: EdgeKindImports}))

	t.Run("depth 1", func(t *testing.T) {
		chains, err := s.GetDependencies(ctx, "a.go", DirectionDownstream, 1)
		require.NoError(t, err)
		// At depth 1 from A, only B is reachable.
		require.Len(t, chains, 1)
		assert.Equal(t, []string{"a.go", "b.go"}, chains[0].Nodes)
		assert.Equal(t, 1, chains[0].Depth)
	})

	t.Run("depth 10", func(t *testing.T) {
		chains, err := s.GetDependencies(ctx, "a.go", DirectionDownstream, 10)
		require.NoError(t, err)
		// At depth 10 from A, both B (depth 1) and C (depth 2) are reachable.
		require.Len(t, chains, 2)

		terminalNodes := make([]string, len(chains))
		for i, c := range chains {
			terminalNodes[i] = c.Nodes[len(c.Nodes)-1]
		}
		sort.Strings(terminalNodes)
		assert.Equal(t, []string{"b.go", "c.go"}, terminalNodes)
	})

	t.Run("leaf node has no downstream", func(t *testing.T) {
		chains, err := s.GetDependencies(ctx, "c.go", DirectionDownstream, 10)
		require.NoError(t, err)
		assert.Empty(t, chains)
	})
}

func TestKuzuStore_Dependencies_Upstream(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// A imports B, B imports C.
	// DirectionUpstream from C follows incoming IMPORTS edges reversed.
	files := []FileNode{
		{Path: "a.go", Language: LangGo, LOC: 10},
		{Path: "b.go", Language: LangGo, LOC: 20},
		{Path: "c.go", Language: LangGo, LOC: 30},
	}
	for _, f := range files {
		require.NoError(t, s.AddFile(ctx, f))
	}

	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "b.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "b.go", TargetID: "c.go", Kind: EdgeKindImports}))

	// Upstream from B: who imports B? A imports B, so upstream returns A.
	chains, err := s.GetDependencies(ctx, "b.go", DirectionUpstream, 10)
	require.NoError(t, err)
	require.Len(t, chains, 1)
	assert.Equal(t, []string{"b.go", "a.go"}, chains[0].Nodes)
}

func TestKuzuStore_AssessImpact(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Build a small dependency graph:
	//   A imports B
	//   A imports C
	//   B imports D
	//
	// AssessImpact uses GetDependencies with DirectionDownstream, which follows
	// outgoing IMPORTS edges. So for changedFiles=["A"]:
	//   Downstream depth 1: B, C
	//   Downstream depth 10: B, C, D
	//   DirectlyAffected = {B, C}  (depth-1 reachable, minus changed set)
	//   TransitivelyAffected = {B, C, D} (depth-10 reachable, minus changed set)
	files := []FileNode{
		{Path: "a.go", Language: LangGo, LOC: 10},
		{Path: "b.go", Language: LangGo, LOC: 20},
		{Path: "c.go", Language: LangGo, LOC: 30},
		{Path: "d.go", Language: LangGo, LOC: 40},
	}
	for _, f := range files {
		require.NoError(t, s.AddFile(ctx, f))
	}

	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "b.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "c.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "b.go", TargetID: "d.go", Kind: EdgeKindImports}))

	result, err := s.AssessImpact(ctx, []string{"a.go"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, []string{"b.go", "c.go"}, sorted(result.DirectlyAffected))
	assert.Equal(t, []string{"b.go", "c.go", "d.go"}, sorted(result.TransitivelyAffected))

	// RiskScore = len(transitive) / totalFiles = 3/4 = 0.75.
	assert.InDelta(t, 0.75, result.RiskScore, 0.01)
}

func TestKuzuStore_AssessImpact_NoImpact(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// A single file with no imports; changing it affects nothing.
	require.NoError(t, s.AddFile(ctx, FileNode{Path: "solo.go", Language: LangGo, LOC: 5}))

	result, err := s.AssessImpact(ctx, []string{"solo.go"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.DirectlyAffected)
	assert.Empty(t, result.TransitivelyAffected)
	assert.InDelta(t, 0.0, result.RiskScore, 0.001)
}

func TestKuzuStore_Clusters(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Add files and a cluster, then link them with BELONGS edges.
	require.NoError(t, s.AddFile(ctx, FileNode{Path: "svc/a.go", Language: LangGo, LOC: 10}))
	require.NoError(t, s.AddFile(ctx, FileNode{Path: "svc/b.go", Language: LangGo, LOC: 20}))
	require.NoError(t, s.AddCluster(ctx, ClusterNode{
		Name:          "svc-cluster",
		CohesionScore: 0.85,
		Members:       []string{"svc/a.go", "svc/b.go"},
	}))

	// BELONGS edges: File -> Cluster.
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "svc/a.go", TargetID: "svc-cluster", Kind: EdgeKindBelongs}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "svc/b.go", TargetID: "svc-cluster", Kind: EdgeKindBelongs}))

	clusters, err := s.GetClusters(ctx)
	require.NoError(t, err)
	require.Len(t, clusters, 1)

	c := clusters[0]
	assert.Equal(t, "svc-cluster", c.Name)
	assert.InDelta(t, 0.85, c.CohesionScore, 0.001)

	// Members are populated via BELONGS_TO edges in GetClusters.
	assert.Equal(t, []string{"svc/a.go", "svc/b.go"}, sorted(c.Members))
}

func TestKuzuStore_Stats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Start with an empty graph.
	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.FileCount)
	assert.Equal(t, 0, stats.SymbolCount)
	assert.Equal(t, 0, stats.ClusterCount)
	assert.Equal(t, 0, stats.EdgeCount)

	// Populate the graph.
	require.NoError(t, s.AddFile(ctx, FileNode{Path: "x.go", Language: LangGo, LOC: 100}))
	require.NoError(t, s.AddFile(ctx, FileNode{Path: "y.go", Language: LangGo, LOC: 200}))
	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "Foo", Kind: SymbolKindFunction, Exported: true,
		FilePath: "x.go", StartLine: 1, EndLine: 10,
	}))
	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "Bar", Kind: SymbolKindType, Exported: true,
		FilePath: "y.go", StartLine: 1, EndLine: 5,
	}))
	require.NoError(t, s.AddCluster(ctx, ClusterNode{Name: "alpha", CohesionScore: 0.9}))

	// Edges.
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "x.go", TargetID: "y.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "x.go", TargetID: "x.go:Foo", Kind: EdgeKindDefines}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "y.go", TargetID: "y.go:Bar", Kind: EdgeKindDefines}))

	stats, err = s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.FileCount)
	assert.Equal(t, 2, stats.SymbolCount)
	assert.Equal(t, 1, stats.ClusterCount)
	assert.Equal(t, 3, stats.EdgeCount)
}

func TestKuzuStore_Close(t *testing.T) {
	s, err := NewKuzuStore()
	require.NoError(t, err)

	// Close should succeed without error.
	require.NoError(t, s.Close())
}

func TestKuzuStore_MultipleLanguages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	files := []FileNode{
		{Path: "main.go", Language: LangGo, LOC: 50},
		{Path: "app.ts", Language: LangTypeScript, LOC: 120},
		{Path: "lib.py", Language: LangPython, LOC: 80},
		{Path: "core.rs", Language: LangRust, LOC: 200},
	}
	for _, f := range files {
		require.NoError(t, s.AddFile(ctx, f))
	}

	for _, f := range files {
		got, err := s.GetFile(ctx, f.Path)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, f.Language, got.Language, "Language mismatch for %s", f.Path)
	}
}

func TestKuzuStore_SymbolKinds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	kinds := []SymbolKind{
		SymbolKindFunction,
		SymbolKindMethod,
		SymbolKindType,
		SymbolKindInterface,
		SymbolKindClass,
	}

	for i, kind := range kinds {
		sym := SymbolNode{
			Name:      "Sym" + string(kind),
			Kind:      kind,
			Exported:  true,
			FilePath:  "kinds.go",
			StartLine: i*10 + 1,
			EndLine:   i*10 + 9,
		}
		require.NoError(t, s.AddSymbol(ctx, sym))

		got, err := s.GetSymbol(ctx, sym.FilePath, sym.Name)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, kind, got.Kind)
	}
}

func TestKuzuStore_Dependencies_DiamondGraph(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Diamond: A imports B, A imports C, B imports D, C imports D.
	//   A
	//  / \
	// B   C
	//  \ /
	//   D
	files := []FileNode{
		{Path: "a.go", Language: LangGo, LOC: 10},
		{Path: "b.go", Language: LangGo, LOC: 10},
		{Path: "c.go", Language: LangGo, LOC: 10},
		{Path: "d.go", Language: LangGo, LOC: 10},
	}
	for _, f := range files {
		require.NoError(t, s.AddFile(ctx, f))
	}

	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "b.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "c.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "b.go", TargetID: "d.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "c.go", TargetID: "d.go", Kind: EdgeKindImports}))

	chains, err := s.GetDependencies(ctx, "a.go", DirectionDownstream, 10)
	require.NoError(t, err)

	// BFS from A should reach B, C (depth 1) and D (depth 2).
	// D is reached only once due to visited set.
	require.Len(t, chains, 3)

	terminalNodes := make([]string, len(chains))
	for i, c := range chains {
		terminalNodes[i] = c.Nodes[len(c.Nodes)-1]
	}
	sort.Strings(terminalNodes)
	assert.Equal(t, []string{"b.go", "c.go", "d.go"}, terminalNodes)

	// D should be at depth 2.
	for _, c := range chains {
		last := c.Nodes[len(c.Nodes)-1]
		if last == "d.go" {
			assert.Equal(t, 2, c.Depth)
		}
	}
}

func TestKuzuStore_AssessImpact_DiamondGraph(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Same diamond: A->B, A->C, B->D, C->D.
	// ChangedFiles = ["B"]. Downstream from B (depth 1): D.
	// Downstream from B (depth 10): D.
	// DirectlyAffected = ["D"], TransitivelyAffected = ["D"].
	files := []FileNode{
		{Path: "a.go", Language: LangGo, LOC: 10},
		{Path: "b.go", Language: LangGo, LOC: 10},
		{Path: "c.go", Language: LangGo, LOC: 10},
		{Path: "d.go", Language: LangGo, LOC: 10},
	}
	for _, f := range files {
		require.NoError(t, s.AddFile(ctx, f))
	}

	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "b.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "c.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "b.go", TargetID: "d.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "c.go", TargetID: "d.go", Kind: EdgeKindImports}))

	result, err := s.AssessImpact(ctx, []string{"b.go"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, []string{"d.go"}, sorted(result.DirectlyAffected))
	assert.Equal(t, []string{"d.go"}, sorted(result.TransitivelyAffected))

	// RiskScore = 1/4 = 0.25.
	assert.InDelta(t, 0.25, result.RiskScore, 0.01)
}

func TestKuzuStore_AssessImpact_MultipleChangedFiles(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// A imports B, C imports D. Change A and C.
	// Downstream from A: B. Downstream from C: D.
	// DirectlyAffected = {B, D}, TransitivelyAffected = {B, D}.
	files := []FileNode{
		{Path: "a.go", Language: LangGo, LOC: 10},
		{Path: "b.go", Language: LangGo, LOC: 10},
		{Path: "c.go", Language: LangGo, LOC: 10},
		{Path: "d.go", Language: LangGo, LOC: 10},
	}
	for _, f := range files {
		require.NoError(t, s.AddFile(ctx, f))
	}

	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "a.go", TargetID: "b.go", Kind: EdgeKindImports}))
	require.NoError(t, s.AddEdge(ctx, Edge{SourceID: "c.go", TargetID: "d.go", Kind: EdgeKindImports}))

	result, err := s.AssessImpact(ctx, []string{"a.go", "c.go"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, []string{"b.go", "d.go"}, sorted(result.DirectlyAffected))
	assert.Equal(t, []string{"b.go", "d.go"}, sorted(result.TransitivelyAffected))

	// RiskScore = 2/4 = 0.5.
	assert.InDelta(t, 0.5, result.RiskScore, 0.01)
}

func TestKuzuStore_EdgeKindInherits(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "Base", Kind: SymbolKindClass, Exported: true,
		FilePath: "base.py", StartLine: 1, EndLine: 10,
	}))
	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "Derived", Kind: SymbolKindClass, Exported: true,
		FilePath: "derived.py", StartLine: 1, EndLine: 15,
	}))

	edge := Edge{
		SourceID: "derived.py:Derived",
		TargetID: "base.py:Base",
		Kind:     EdgeKindInherits,
	}
	require.NoError(t, s.AddEdge(ctx, edge))

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EdgeCount)
}

func TestKuzuStore_EdgeKindImplements(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "Store", Kind: SymbolKindInterface, Exported: true,
		FilePath: "store.go", StartLine: 1, EndLine: 30,
	}))
	require.NoError(t, s.AddSymbol(ctx, SymbolNode{
		Name: "KuzuStore", Kind: SymbolKindType, Exported: true,
		FilePath: "kuzustore.go", StartLine: 1, EndLine: 50,
	}))

	edge := Edge{
		SourceID: "kuzustore.go:KuzuStore",
		TargetID: "store.go:Store",
		Kind:     EdgeKindImplements,
	}
	require.NoError(t, s.AddEdge(ctx, edge))

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EdgeCount)
}
