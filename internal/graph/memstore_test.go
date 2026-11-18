package graph

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chainEnds returns the sorted last node of every chain.
func chainEnds(chains []DependencyChain) []string {
	out := make([]string, 0, len(chains))
	for _, c := range chains {
		out = append(out, c.Nodes[len(c.Nodes)-1])
	}
	sort.Strings(out)
	return out
}

// TestMemStore_GetDependencies_ImportsOnly checks that traversal follows
// IMPORTS edges only, so callee names, symbol ids, and cluster names stored by
// build_graph are never returned as files.
func TestMemStore_GetDependencies_ImportsOnly(t *testing.T) {
	ctx := context.Background()
	store := NewMemStore()

	for _, p := range []string{"a.go", "b.go", "c.go"} {
		require.NoError(t, store.AddFile(ctx, FileNode{Path: p, Language: LangGo, LOC: 10}))
	}
	edges := []Edge{
		{SourceID: "a.go", TargetID: "b.go", Kind: EdgeKindImports},
		{SourceID: "b.go", TargetID: "c.go", Kind: EdgeKindImports},
		{SourceID: "a.go", TargetID: "fmt.Sprintf", Kind: EdgeKindCalls},
		{SourceID: "c.go", TargetID: "fmt.Sprintf", Kind: EdgeKindCalls},
		{SourceID: "a.go", TargetID: "a.go:Run:3", Kind: EdgeKindDefines},
		{SourceID: "a.go", TargetID: "pkg/", Kind: EdgeKindBelongs},
		{SourceID: "b.go", TargetID: "pkg/", Kind: EdgeKindBelongs},
	}
	for _, e := range edges {
		require.NoError(t, store.AddEdge(ctx, e))
	}

	down, err := store.GetDependencies(ctx, "a.go", DirectionDownstream, 5)
	require.NoError(t, err)
	assert.Equal(t, []string{"b.go", "c.go"}, chainEnds(down))

	up, err := store.GetDependencies(ctx, "c.go", DirectionUpstream, 5)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.go", "b.go"}, chainEnds(up))

	// A callee name or cluster name is not a file, so nothing imports it.
	for _, id := range []string{"fmt.Sprintf", "pkg/"} {
		up, err := store.GetDependencies(ctx, id, DirectionUpstream, 5)
		require.NoError(t, err)
		assert.Empty(t, up, id)
	}
}
