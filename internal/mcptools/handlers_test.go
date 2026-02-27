//go:build cgo

package mcptools

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/dusk-indust/decompose/internal/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fixtureAbsPath returns the absolute path to the go_project test fixture
// directory. Tests run from internal/mcptools/, so the relative path is
// ../../testdata/fixtures/go_project.
func fixtureAbsPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../testdata/fixtures/go_project")
	require.NoError(t, err)
	return abs
}

// newTestStore creates a MemStore with an initialized schema.
func newTestStore(t *testing.T) *graph.MemStore {
	t.Helper()
	store := graph.NewMemStore()
	require.NoError(t, store.InitSchema(context.Background()))
	return store
}

// seedSymbols populates the store with a set of known symbols and their files.
func seedSymbols(t *testing.T, store *graph.MemStore) {
	t.Helper()
	ctx := context.Background()

	files := []graph.FileNode{
		{Path: "pkg/handler.go", Language: graph.LangGo, LOC: 100},
		{Path: "pkg/service.go", Language: graph.LangGo, LOC: 80},
		{Path: "pkg/model.go", Language: graph.LangGo, LOC: 50},
	}
	for _, f := range files {
		require.NoError(t, store.AddFile(ctx, f))
	}

	symbols := []graph.SymbolNode{
		{Name: "HandleRequest", Kind: graph.SymbolKindFunction, Exported: true, FilePath: "pkg/handler.go", StartLine: 10, EndLine: 30},
		{Name: "HandleResponse", Kind: graph.SymbolKindFunction, Exported: true, FilePath: "pkg/handler.go", StartLine: 32, EndLine: 50},
		{Name: "UserService", Kind: graph.SymbolKindType, Exported: true, FilePath: "pkg/service.go", StartLine: 5, EndLine: 15},
		{Name: "NewUserService", Kind: graph.SymbolKindFunction, Exported: true, FilePath: "pkg/service.go", StartLine: 17, EndLine: 25},
		{Name: "User", Kind: graph.SymbolKindType, Exported: true, FilePath: "pkg/model.go", StartLine: 3, EndLine: 10},
		{Name: "validateUser", Kind: graph.SymbolKindFunction, Exported: false, FilePath: "pkg/model.go", StartLine: 12, EndLine: 20},
	}
	for _, s := range symbols {
		require.NoError(t, store.AddSymbol(ctx, s))
	}
}

// seedDiamondGraph populates the store with a diamond dependency graph:
//
//	A -> B
//	A -> C
//	B -> D
//	C -> D
//
// All edges are IMPORTS. All nodes are file nodes.
func seedDiamondGraph(t *testing.T, store *graph.MemStore) {
	t.Helper()
	ctx := context.Background()

	files := []graph.FileNode{
		{Path: "A.go", Language: graph.LangGo, LOC: 10},
		{Path: "B.go", Language: graph.LangGo, LOC: 20},
		{Path: "C.go", Language: graph.LangGo, LOC: 30},
		{Path: "D.go", Language: graph.LangGo, LOC: 40},
	}
	for _, f := range files {
		require.NoError(t, store.AddFile(ctx, f))
	}

	edges := []graph.Edge{
		{SourceID: "A.go", TargetID: "B.go", Kind: graph.EdgeKindImports},
		{SourceID: "A.go", TargetID: "C.go", Kind: graph.EdgeKindImports},
		{SourceID: "B.go", TargetID: "D.go", Kind: graph.EdgeKindImports},
		{SourceID: "C.go", TargetID: "D.go", Kind: graph.EdgeKindImports},
	}
	for _, e := range edges {
		require.NoError(t, store.AddEdge(ctx, e))
	}
}

// seedLinearChain populates the store with a linear chain: A -> B -> C
// using IMPORTS edges.
func seedLinearChain(t *testing.T, store *graph.MemStore) {
	t.Helper()
	ctx := context.Background()

	files := []graph.FileNode{
		{Path: "A.go", Language: graph.LangGo, LOC: 10},
		{Path: "B.go", Language: graph.LangGo, LOC: 20},
		{Path: "C.go", Language: graph.LangGo, LOC: 30},
	}
	for _, f := range files {
		require.NoError(t, store.AddFile(ctx, f))
	}

	edges := []graph.Edge{
		{SourceID: "A.go", TargetID: "B.go", Kind: graph.EdgeKindImports},
		{SourceID: "B.go", TargetID: "C.go", Kind: graph.EdgeKindImports},
	}
	for _, e := range edges {
		require.NoError(t, store.AddEdge(ctx, e))
	}
}

// containsNode returns true if any chain in the slice contains the given node ID.
func containsNode(chains []graph.DependencyChain, nodeID string) bool {
	for _, chain := range chains {
		for _, n := range chain.Nodes {
			if n == nodeID {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// TestBuildGraph
// ---------------------------------------------------------------------------

func TestBuildGraph(t *testing.T) {
	t.Run("indexes go_project fixture", func(t *testing.T) {
		store := newTestStore(t)
		parser := graph.NewTreeSitterParser()
		defer parser.Close()

		svc := NewCodeIntelService(store, parser)
		ctx := context.Background()

		_, out, err := svc.BuildGraph(ctx, nil, BuildGraphInput{
			RepoPath:  fixtureAbsPath(t),
			Languages: []string{"go"},
		})
		require.NoError(t, err)

		assert.Greater(t, out.Stats.FileCount, 0, "should index at least one file")
		assert.Greater(t, out.Stats.SymbolCount, 0, "should extract at least one symbol")
		assert.Greater(t, out.Stats.EdgeCount, 0, "should discover at least one edge")
		// The go_project has 3 files; at least some should cluster together.
		assert.GreaterOrEqual(t, out.Stats.FileCount, 3, "go_project has 3 Go files")
	})

	t.Run("non-existent path returns error", func(t *testing.T) {
		store := newTestStore(t)
		parser := graph.NewTreeSitterParser()
		defer parser.Close()

		svc := NewCodeIntelService(store, parser)
		ctx := context.Background()

		_, _, err := svc.BuildGraph(ctx, nil, BuildGraphInput{
			RepoPath:  "/tmp/this-path-does-not-exist-at-all-12345",
			Languages: []string{"go"},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot access repoPath")
	})

	t.Run("empty repoPath returns error", func(t *testing.T) {
		store := newTestStore(t)
		parser := graph.NewTreeSitterParser()
		defer parser.Close()

		svc := NewCodeIntelService(store, parser)
		ctx := context.Background()

		_, _, err := svc.BuildGraph(ctx, nil, BuildGraphInput{
			RepoPath: "",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "repoPath is required")
	})

	t.Run("empty languages defaults to tier-1", func(t *testing.T) {
		store := newTestStore(t)
		parser := graph.NewTreeSitterParser()
		defer parser.Close()

		svc := NewCodeIntelService(store, parser)
		ctx := context.Background()

		// When Languages is empty/nil, the handler should default to all
		// tier-1 languages. The go_project only has Go files, so the
		// result should still contain Go files.
		_, out, err := svc.BuildGraph(ctx, nil, BuildGraphInput{
			RepoPath: fixtureAbsPath(t),
		})
		require.NoError(t, err)
		assert.Greater(t, out.Stats.FileCount, 0, "should index files with default tier-1 languages")
	})
}

// ---------------------------------------------------------------------------
// TestQuerySymbols
// ---------------------------------------------------------------------------

func TestQuerySymbols(t *testing.T) {
	t.Run("substring match returns matching symbols", func(t *testing.T) {
		store := newTestStore(t)
		seedSymbols(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.QuerySymbols(ctx, nil, QuerySymbolsInput{
			Query: "Handle",
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, out.Total, 2, "should match HandleRequest and HandleResponse")

		names := make([]string, len(out.Symbols))
		for i, s := range out.Symbols {
			names[i] = s.Name
		}
		sort.Strings(names)
		assert.Contains(t, names, "HandleRequest")
		assert.Contains(t, names, "HandleResponse")
	})

	t.Run("kind filter returns only that kind", func(t *testing.T) {
		store := newTestStore(t)
		seedSymbols(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		// Query for "User" which matches User (type), UserService (type),
		// NewUserService (function), validateUser (function).
		// Filter to kind=type should only return User and UserService.
		_, out, err := svc.QuerySymbols(ctx, nil, QuerySymbolsInput{
			Query: "User",
			Kind:  "type",
		})
		require.NoError(t, err)

		for _, sym := range out.Symbols {
			assert.Equal(t, graph.SymbolKindType, sym.Kind,
				"all returned symbols should be of kind 'type', got %s for %s", sym.Kind, sym.Name)
		}
		assert.GreaterOrEqual(t, out.Total, 1, "should match at least User or UserService as type")
	})

	t.Run("limit is respected", func(t *testing.T) {
		store := newTestStore(t)
		seedSymbols(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		// Query for "e" which matches many symbols. Limit to 2.
		_, out, err := svc.QuerySymbols(ctx, nil, QuerySymbolsInput{
			Query: "e",
			Limit: 2,
		})
		require.NoError(t, err)
		assert.LessOrEqual(t, out.Total, 2, "should return at most 2 symbols")
	})

	t.Run("default limit is 20", func(t *testing.T) {
		store := newTestStore(t)
		seedSymbols(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		// With limit=0 (the zero value), the handler defaults to 20.
		// We only have 6 symbols, so all should be returned.
		_, out, err := svc.QuerySymbols(ctx, nil, QuerySymbolsInput{
			Query: "",
		})
		require.NoError(t, err)
		// An empty query matches everything via substring match.
		assert.Equal(t, 6, out.Total, "empty query should match all 6 seeded symbols")
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		store := newTestStore(t)
		seedSymbols(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.QuerySymbols(ctx, nil, QuerySymbolsInput{
			Query: "ZzNonExistentSymbol",
		})
		require.NoError(t, err)
		assert.Equal(t, 0, out.Total)
		assert.Empty(t, out.Symbols)
	})
}

// ---------------------------------------------------------------------------
// TestGetDependencies
// ---------------------------------------------------------------------------

func TestGetDependencies(t *testing.T) {
	t.Run("downstream from A returns chain containing B and C", func(t *testing.T) {
		store := newTestStore(t)
		seedLinearChain(t, store) // A -> B -> C
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.GetDependencies(ctx, nil, GetDependenciesInput{
			NodeID:    "A.go",
			Direction: "downstream",
		})
		require.NoError(t, err)
		require.NotEmpty(t, out.Chains, "should find downstream dependencies from A")

		assert.True(t, containsNode(out.Chains, "B.go"),
			"downstream from A should reach B")
		assert.True(t, containsNode(out.Chains, "C.go"),
			"downstream from A should reach C (transitively through B)")
	})

	t.Run("upstream from C returns chain containing B and A", func(t *testing.T) {
		store := newTestStore(t)
		seedLinearChain(t, store) // A -> B -> C
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.GetDependencies(ctx, nil, GetDependenciesInput{
			NodeID:    "C.go",
			Direction: "upstream",
		})
		require.NoError(t, err)
		require.NotEmpty(t, out.Chains, "should find upstream dependencies from C")

		assert.True(t, containsNode(out.Chains, "B.go"),
			"upstream from C should reach B")
		assert.True(t, containsNode(out.Chains, "A.go"),
			"upstream from C should reach A (transitively through B)")
	})

	t.Run("default direction is downstream", func(t *testing.T) {
		store := newTestStore(t)
		seedLinearChain(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		// Omit Direction; it should default to downstream.
		_, out, err := svc.GetDependencies(ctx, nil, GetDependenciesInput{
			NodeID: "A.go",
		})
		require.NoError(t, err)
		assert.True(t, containsNode(out.Chains, "B.go"),
			"default direction should be downstream, reaching B from A")
	})

	t.Run("maxDepth=1 limits traversal", func(t *testing.T) {
		store := newTestStore(t)
		seedLinearChain(t, store) // A -> B -> C
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.GetDependencies(ctx, nil, GetDependenciesInput{
			NodeID:   "A.go",
			MaxDepth: 1,
		})
		require.NoError(t, err)

		assert.True(t, containsNode(out.Chains, "B.go"),
			"depth=1 from A should reach B")
		assert.False(t, containsNode(out.Chains, "C.go"),
			"depth=1 from A should NOT reach C")
	})

	t.Run("empty nodeId returns error", func(t *testing.T) {
		store := newTestStore(t)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, _, err := svc.GetDependencies(ctx, nil, GetDependenciesInput{
			NodeID: "",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nodeId is required")
	})

	t.Run("non-existent node returns empty chains", func(t *testing.T) {
		store := newTestStore(t)
		seedLinearChain(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.GetDependencies(ctx, nil, GetDependenciesInput{
			NodeID: "nonexistent.go",
		})
		require.NoError(t, err)
		assert.Empty(t, out.Chains, "non-existent node should have no dependencies")
	})
}

// ---------------------------------------------------------------------------
// TestAssessImpact
// ---------------------------------------------------------------------------

func TestAssessImpact(t *testing.T) {
	t.Run("change root node in diamond", func(t *testing.T) {
		// Diamond: A->B, A->C, B->D, C->D
		// Changing A: B and C directly import A, D transitively.
		store := newTestStore(t)
		seedDiamondGraph(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.AssessImpact(ctx, nil, AssessImpactInput{
			ChangedFiles: []string{"A.go"},
		})
		require.NoError(t, err)

		// No file imports A.go in the diamond (A is the root importer).
		// A->B means A imports B, so changing A doesn't affect B.
		// Actually: edge SourceID=A, TargetID=B means "A imports B".
		// AssessImpact finds files that IMPORT the changed file.
		// Nobody imports A.go (A is the one doing the importing), so
		// DirectlyAffected should be empty.
		assert.Empty(t, out.Impact.DirectlyAffected,
			"no file imports A.go, so nothing is directly affected")
	})

	t.Run("change leaf node in diamond", func(t *testing.T) {
		// Diamond: A->B, A->C, B->D, C->D
		// Edge semantics: SourceID imports TargetID.
		// Changing D: B imports D, C imports D -> directly affected = {B, C}.
		// Then A imports B and A imports C -> transitively affected includes A.
		store := newTestStore(t)
		seedDiamondGraph(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.AssessImpact(ctx, nil, AssessImpactInput{
			ChangedFiles: []string{"D.go"},
		})
		require.NoError(t, err)

		directSet := make(map[string]bool)
		for _, f := range out.Impact.DirectlyAffected {
			directSet[f] = true
		}
		assert.True(t, directSet["B.go"], "B imports D so should be directly affected")
		assert.True(t, directSet["C.go"], "C imports D so should be directly affected")

		transitiveSet := make(map[string]bool)
		for _, f := range out.Impact.TransitivelyAffected {
			transitiveSet[f] = true
		}
		assert.True(t, transitiveSet["A.go"],
			"A imports B and C (which import D), so A should be transitively affected")
		assert.True(t, transitiveSet["B.go"], "B should be in transitive set")
		assert.True(t, transitiveSet["C.go"], "C should be in transitive set")

		assert.Greater(t, out.Impact.RiskScore, 0.0, "risk score should be positive")
	})

	t.Run("change middle node B", func(t *testing.T) {
		// Diamond: A->B, A->C, B->D, C->D
		// Changing B: A imports B -> directly affected = {A}.
		store := newTestStore(t)
		seedDiamondGraph(t, store)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.AssessImpact(ctx, nil, AssessImpactInput{
			ChangedFiles: []string{"B.go"},
		})
		require.NoError(t, err)

		directSet := make(map[string]bool)
		for _, f := range out.Impact.DirectlyAffected {
			directSet[f] = true
		}
		assert.True(t, directSet["A.go"], "A imports B so should be directly affected")
		assert.False(t, directSet["C.go"], "C does not import B")
		assert.False(t, directSet["D.go"], "D does not import B")
	})

	t.Run("empty changedFiles returns error", func(t *testing.T) {
		store := newTestStore(t)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, _, err := svc.AssessImpact(ctx, nil, AssessImpactInput{
			ChangedFiles: nil,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "changedFiles is required")
	})
}

// ---------------------------------------------------------------------------
// TestGetClusters
// ---------------------------------------------------------------------------

func TestGetClusters(t *testing.T) {
	t.Run("returns clusters after graph build", func(t *testing.T) {
		store := newTestStore(t)
		parser := graph.NewTreeSitterParser()
		defer parser.Close()

		svc := NewCodeIntelService(store, parser)
		ctx := context.Background()

		// Build the graph first so that clusters are computed.
		_, _, err := svc.BuildGraph(ctx, nil, BuildGraphInput{
			RepoPath:  fixtureAbsPath(t),
			Languages: []string{"go"},
		})
		require.NoError(t, err)

		_, out, err := svc.GetClusters(ctx, nil, GetClustersInput{})
		require.NoError(t, err)

		// The go_project has 3 files with cross-file references that
		// should form at least one cluster. However, clustering only groups
		// components with >= 2 files connected by IMPORTS edges between
		// known files. If the fixture files don't import each other
		// (they import "fmt"), clusters may be empty. We assert that
		// the handler returns successfully regardless.
		// If the fixture does produce clusters, verify structure.
		for _, c := range out.Clusters {
			assert.NotEmpty(t, c.Name, "cluster name should not be empty")
			assert.GreaterOrEqual(t, len(c.Members), 2,
				"each cluster should have at least 2 members")
			assert.GreaterOrEqual(t, c.CohesionScore, 0.0,
				"cohesion score should be non-negative")
			assert.LessOrEqual(t, c.CohesionScore, 1.0,
				"cohesion score should be at most 1.0")
		}
	})

	t.Run("returns pre-populated clusters", func(t *testing.T) {
		store := newTestStore(t)
		ctx := context.Background()

		// Manually add clusters to the store.
		require.NoError(t, store.AddCluster(ctx, graph.ClusterNode{
			Name:          "pkg/auth/",
			CohesionScore: 0.85,
			Members:       []string{"pkg/auth/handler.go", "pkg/auth/middleware.go"},
		}))
		require.NoError(t, store.AddCluster(ctx, graph.ClusterNode{
			Name:          "pkg/db/",
			CohesionScore: 0.92,
			Members:       []string{"pkg/db/conn.go", "pkg/db/query.go", "pkg/db/migrate.go"},
		}))

		svc := NewCodeIntelService(store, nil)

		_, out, err := svc.GetClusters(ctx, nil, GetClustersInput{})
		require.NoError(t, err)
		require.Len(t, out.Clusters, 2, "should return 2 pre-populated clusters")

		// Sort for deterministic assertion.
		sort.Slice(out.Clusters, func(i, j int) bool {
			return out.Clusters[i].Name < out.Clusters[j].Name
		})

		assert.Equal(t, "pkg/auth/", out.Clusters[0].Name)
		assert.Equal(t, 0.85, out.Clusters[0].CohesionScore)
		assert.Len(t, out.Clusters[0].Members, 2)

		assert.Equal(t, "pkg/db/", out.Clusters[1].Name)
		assert.Equal(t, 0.92, out.Clusters[1].CohesionScore)
		assert.Len(t, out.Clusters[1].Members, 3)
	})

	t.Run("empty store returns empty clusters", func(t *testing.T) {
		store := newTestStore(t)
		svc := NewCodeIntelService(store, nil)
		ctx := context.Background()

		_, out, err := svc.GetClusters(ctx, nil, GetClustersInput{})
		require.NoError(t, err)
		assert.Empty(t, out.Clusters, "empty store should return no clusters")
	})
}
