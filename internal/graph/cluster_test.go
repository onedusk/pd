package graph

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupStore creates a MemStore and populates it with the given files and edges.
func setupStore(t *testing.T, files []FileNode, edges []Edge) *MemStore {
	t.Helper()
	ctx := context.Background()
	store := NewMemStore()
	require.NoError(t, store.InitSchema(ctx))

	for _, f := range files {
		require.NoError(t, store.AddFile(ctx, f))
	}
	for _, e := range edges {
		require.NoError(t, store.AddEdge(ctx, e))
	}
	return store
}

// sortedMembers returns a sorted copy of cluster members for deterministic comparison.
func sortedMembers(members []string) []string {
	out := make([]string, len(members))
	copy(out, members)
	sort.Strings(out)
	return out
}

func TestComputeClusters_NoEdges(t *testing.T) {
	// Three files with no IMPORTS edges between them.
	// Each file is a singleton component (size < 2), so zero clusters.
	files := []FileNode{
		{Path: "src/pkg/a.go", Language: LangGo, LOC: 50},
		{Path: "src/pkg/b.go", Language: LangGo, LOC: 60},
		{Path: "src/pkg/c.go", Language: LangGo, LOC: 70},
	}

	store := setupStore(t, files, nil)
	ctx := context.Background()

	clusters, err := ComputeClusters(ctx, store, files)
	require.NoError(t, err)
	assert.Empty(t, clusters, "expected zero clusters when there are no edges")

	// Verify nothing was stored.
	stored, err := store.GetClusters(ctx)
	require.NoError(t, err)
	assert.Empty(t, stored)
}

func TestComputeClusters_OnePair(t *testing.T) {
	// Three files, but only A→B has an IMPORTS edge.
	// A and B should form one cluster; C is a singleton and gets skipped.
	files := []FileNode{
		{Path: "src/pkg/a.go", Language: LangGo, LOC: 50},
		{Path: "src/pkg/b.go", Language: LangGo, LOC: 60},
		{Path: "src/pkg/c.go", Language: LangGo, LOC: 70},
	}
	edges := []Edge{
		{SourceID: "src/pkg/a.go", TargetID: "src/pkg/b.go", Kind: EdgeKindImports},
	}

	store := setupStore(t, files, edges)
	ctx := context.Background()

	clusters, err := ComputeClusters(ctx, store, files)
	require.NoError(t, err)
	require.Len(t, clusters, 1, "expected exactly one cluster")

	members := sortedMembers(clusters[0].Members)
	assert.Equal(t, []string{"src/pkg/a.go", "src/pkg/b.go"}, members)

	// The singleton C should NOT appear in any cluster.
	for _, c := range clusters {
		for _, m := range c.Members {
			assert.NotEqual(t, "src/pkg/c.go", m, "singleton C must not appear in a cluster")
		}
	}

	// Verify BELONGS edges were stored: one per member of the cluster.
	stats, err := store.Stats(ctx)
	require.NoError(t, err)
	// Original 1 IMPORTS edge + 2 BELONGS edges = 3 total edges.
	assert.Equal(t, 3, stats.EdgeCount, "expected 1 IMPORTS + 2 BELONGS edges")
}

func TestComputeClusters_TwoGroups(t *testing.T) {
	// Six files in two separate groups of 3, each fully connected within the group.
	// Group 1: src/alpha/a.go, src/alpha/b.go, src/alpha/c.go
	// Group 2: src/beta/x.go, src/beta/y.go, src/beta/z.go
	files := []FileNode{
		{Path: "src/alpha/a.go", Language: LangGo, LOC: 30},
		{Path: "src/alpha/b.go", Language: LangGo, LOC: 40},
		{Path: "src/alpha/c.go", Language: LangGo, LOC: 50},
		{Path: "src/beta/x.go", Language: LangGo, LOC: 35},
		{Path: "src/beta/y.go", Language: LangGo, LOC: 45},
		{Path: "src/beta/z.go", Language: LangGo, LOC: 55},
	}
	edges := []Edge{
		// Group 1 edges: a→b, a→c, b→c
		{SourceID: "src/alpha/a.go", TargetID: "src/alpha/b.go", Kind: EdgeKindImports},
		{SourceID: "src/alpha/a.go", TargetID: "src/alpha/c.go", Kind: EdgeKindImports},
		{SourceID: "src/alpha/b.go", TargetID: "src/alpha/c.go", Kind: EdgeKindImports},
		// Group 2 edges: x→y, x→z, y→z
		{SourceID: "src/beta/x.go", TargetID: "src/beta/y.go", Kind: EdgeKindImports},
		{SourceID: "src/beta/x.go", TargetID: "src/beta/z.go", Kind: EdgeKindImports},
		{SourceID: "src/beta/y.go", TargetID: "src/beta/z.go", Kind: EdgeKindImports},
	}

	store := setupStore(t, files, edges)
	ctx := context.Background()

	clusters, err := ComputeClusters(ctx, store, files)
	require.NoError(t, err)
	require.Len(t, clusters, 2, "expected two clusters")

	// Sort clusters by name for deterministic checks.
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	alphaMembers := sortedMembers(clusters[0].Members)
	betaMembers := sortedMembers(clusters[1].Members)

	assert.Equal(t, []string{"src/alpha/a.go", "src/alpha/b.go", "src/alpha/c.go"}, alphaMembers)
	assert.Equal(t, []string{"src/beta/x.go", "src/beta/y.go", "src/beta/z.go"}, betaMembers)
}

func TestComputeClusters_CohesionScore(t *testing.T) {
	// Because buildAdjacency creates bidirectional edges and BFS finds all
	// reachable nodes, any file connected by an edge to a component member
	// will be pulled into that component. This means external edges (edges
	// to known files outside the component) are structurally impossible:
	// cohesion = internal / (internal + external) is always 1.0 for any
	// non-trivial cluster produced by ComputeClusters.
	//
	// We verify this property with two scenarios:
	// 1. A fully connected 3-node cluster (3 internal edges) -> 1.0
	// 2. A simple pair (1 internal edge) -> 1.0

	// Scenario 1: fully connected 3-node cluster.
	files2 := []FileNode{
		{Path: "src/alpha/a.go", Language: LangGo, LOC: 30},
		{Path: "src/alpha/b.go", Language: LangGo, LOC: 40},
		{Path: "src/alpha/c.go", Language: LangGo, LOC: 50},
	}
	edges2 := []Edge{
		{SourceID: "src/alpha/a.go", TargetID: "src/alpha/b.go", Kind: EdgeKindImports},
		{SourceID: "src/alpha/a.go", TargetID: "src/alpha/c.go", Kind: EdgeKindImports},
		{SourceID: "src/alpha/b.go", TargetID: "src/alpha/c.go", Kind: EdgeKindImports},
	}

	store := setupStore(t, files2, edges2)
	ctx := context.Background()

	clusters, err := ComputeClusters(ctx, store, files2)
	require.NoError(t, err)
	require.Len(t, clusters, 1)

	// Fully connected group with only internal edges: cohesion must be 1.0.
	assert.Equal(t, 1.0, clusters[0].CohesionScore,
		"fully internal cluster should have cohesion 1.0")

	// Now test that a chain (2 files, 1 edge) also gets 1.0 since there are
	// no external edges.
	files3 := []FileNode{
		{Path: "src/beta/d.go", Language: LangGo, LOC: 35},
		{Path: "src/beta/e.go", Language: LangGo, LOC: 45},
	}
	edges3 := []Edge{
		{SourceID: "src/beta/d.go", TargetID: "src/beta/e.go", Kind: EdgeKindImports},
	}

	store2 := setupStore(t, files3, edges3)
	clusters2, err := ComputeClusters(ctx, store2, files3)
	require.NoError(t, err)
	require.Len(t, clusters2, 1)

	assert.Equal(t, 1.0, clusters2[0].CohesionScore,
		"pair with no external edges should have cohesion 1.0")
}

func TestComputeClusters_ClusterNames(t *testing.T) {
	// Cluster names are derived from the longest common path prefix of members.
	// Group 1: src/alpha/ prefix
	// Group 2: src/beta/sub/ prefix
	files := []FileNode{
		{Path: "src/alpha/foo.go", Language: LangGo, LOC: 30},
		{Path: "src/alpha/bar.go", Language: LangGo, LOC: 40},
		{Path: "src/beta/sub/one.go", Language: LangGo, LOC: 50},
		{Path: "src/beta/sub/two.go", Language: LangGo, LOC: 60},
	}
	edges := []Edge{
		{SourceID: "src/alpha/foo.go", TargetID: "src/alpha/bar.go", Kind: EdgeKindImports},
		{SourceID: "src/beta/sub/one.go", TargetID: "src/beta/sub/two.go", Kind: EdgeKindImports},
	}

	store := setupStore(t, files, edges)
	ctx := context.Background()

	clusters, err := ComputeClusters(ctx, store, files)
	require.NoError(t, err)
	require.Len(t, clusters, 2)

	// Sort clusters by name for deterministic assertion order.
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	assert.Equal(t, "src/alpha/", clusters[0].Name,
		"cluster name should be the common path prefix 'src/alpha/'")
	assert.Equal(t, "src/beta/sub/", clusters[1].Name,
		"cluster name should be the common path prefix 'src/beta/sub/'")
}
