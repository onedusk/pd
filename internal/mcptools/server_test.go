//go:build cgo

package mcptools

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/dusk-indust/decompose/internal/graph"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupServerClient wires an MCP server and client together using in-memory
// transports. It returns the connected client session and the underlying
// CodeIntelService so that tests can inspect state when needed.
func setupServerClient(t *testing.T) (*mcp.ClientSession, *CodeIntelService) {
	t.Helper()

	store := graph.NewMemStore()
	parser := graph.NewTreeSitterParser()
	svc := NewCodeIntelService(store, parser)
	server := NewCodeIntelMCPServer(svc)

	st, ct := mcp.NewInMemoryTransports()

	ctx := context.Background()

	_, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)

	t.Cleanup(func() {
		session.Close()
	})

	return session, svc
}

// TestMCPListTools verifies that the MCP server exposes exactly 5 tools with
// the expected names.
func TestMCPListTools(t *testing.T) {
	session, _ := setupServerClient(t)
	ctx := context.Background()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err)

	require.Len(t, result.Tools, 5, "expected 5 registered tools")

	names := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		names[i] = tool.Name
	}
	sort.Strings(names)

	expected := []string{
		"assess_impact",
		"build_graph",
		"get_clusters",
		"get_dependencies",
		"query_symbols",
	}
	assert.Equal(t, expected, names)
}

// TestMCPBuildGraph calls the build_graph tool via the MCP client-server
// transport and checks that the returned stats contain files, symbols, and
// edges.
func TestMCPBuildGraph(t *testing.T) {
	session, _ := setupServerClient(t)
	ctx := context.Background()

	absPath := fixtureAbsPath(t)

	args := BuildGraphInput{
		RepoPath:  absPath,
		Languages: []string{"go"},
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "build_graph",
		Arguments: args,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "build_graph should not return an error")

	// The structured output should contain the stats.
	require.NotNil(t, result.StructuredContent, "expected structured content from build_graph")

	raw, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)

	var output BuildGraphOutput
	err = json.Unmarshal(raw, &output)
	require.NoError(t, err)

	assert.Equal(t, 3, output.Stats.FileCount, "fixture has 3 go files")
	assert.Greater(t, output.Stats.SymbolCount, 0, "expected at least one symbol")
	assert.Greater(t, output.Stats.EdgeCount, 0, "expected at least one edge")
}

// TestMCPQuerySymbols builds the graph via MCP, then queries for symbols,
// ensuring results are returned.
func TestMCPQuerySymbols(t *testing.T) {
	session, _ := setupServerClient(t)
	ctx := context.Background()

	absPath := fixtureAbsPath(t)

	// First, build the graph so there are symbols to query.
	buildArgs := BuildGraphInput{
		RepoPath:  absPath,
		Languages: []string{"go"},
	}
	buildResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "build_graph",
		Arguments: buildArgs,
	})
	require.NoError(t, err)
	require.False(t, buildResult.IsError, "build_graph should succeed")

	// Query for a symbol that should exist in the fixture project.
	queryArgs := QuerySymbolsInput{
		Query: "Run",
		Limit: 10,
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_symbols",
		Arguments: queryArgs,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "query_symbols should not return an error")

	require.NotNil(t, result.StructuredContent, "expected structured content from query_symbols")

	raw, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)

	var output QuerySymbolsOutput
	err = json.Unmarshal(raw, &output)
	require.NoError(t, err)

	assert.Greater(t, output.Total, 0, "expected at least one symbol matching 'Run'")

	// Check that at least one symbol name contains "Run".
	found := false
	for _, sym := range output.Symbols {
		if sym.Name == "Run" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected to find a symbol named 'Run' in results")
}

// TestMCPCallUnknownTool verifies that calling a non-existent tool returns an
// error.
func TestMCPCallUnknownTool(t *testing.T) {
	session, _ := setupServerClient(t)
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "nonexistent_tool",
		Arguments: map[string]any{},
	})

	// The MCP SDK may return an error at the protocol level or set IsError on
	// the result. Accept either behavior.
	if err != nil {
		// Protocol-level error is acceptable for unknown tools.
		return
	}

	require.NotNil(t, result)
	assert.True(t, result.IsError, "calling an unknown tool should set IsError")
}
