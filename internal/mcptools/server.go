package mcptools

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is set by the linker at build time.
var version = "dev"

// NewCodeIntelMCPServer creates an MCP server with all 5 code intelligence tools registered.
func NewCodeIntelMCPServer(svc *CodeIntelService) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "decompose-codeintel",
		Version: version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "build_graph",
		Description: "Index a repository and build the code intelligence graph. Walks the file tree, parses source files using tree-sitter, extracts symbols and dependencies, and computes file clusters.",
	}, svc.BuildGraph)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_symbols",
		Description: "Search for symbols (functions, classes, types, etc.) by name substring match. Optionally filter by symbol kind and limit results.",
	}, svc.QuerySymbols)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_dependencies",
		Description: "Traverse the dependency graph upstream or downstream from a file or symbol. Returns dependency chains up to the specified depth.",
	}, svc.GetDependencies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "assess_impact",
		Description: "Compute the blast radius of modifying a set of files. Returns directly and transitively affected files with a risk score.",
	}, svc.AssessImpact)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_clusters",
		Description: "Return all file clusters discovered during graph building. Clusters are groups of tightly connected files with cohesion scores.",
	}, svc.GetClusters)

	return server
}

// RunMCPServer starts an HTTP server exposing the code intelligence MCP tools.
func RunMCPServer(ctx context.Context, svc *CodeIntelService, addr string) error {
	server := NewCodeIntelMCPServer(svc)

	handler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return server },
		nil,
	)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Shutdown gracefully when context is cancelled.
	go func() {
		<-ctx.Done()
		httpServer.Shutdown(context.Background())
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
