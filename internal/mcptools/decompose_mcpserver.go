package mcptools

import (
	"context"

	"github.com/dusk-indust/decompose/internal/orchestrator"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewDecomposeMCPServer creates an MCP server with the 3 decompose tools registered:
// run_stage, get_status, and list_decompositions.
func NewDecomposeMCPServer(pipeline orchestrator.Orchestrator, cfg orchestrator.Config) *mcp.Server {
	svc := NewDecomposeService(pipeline, cfg)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "decompose",
		Version: version,
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_stage",
		Description: "Execute a single decomposition pipeline stage (0-4). Returns the files written.",
	}, svc.RunStage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_status",
		Description: "Get the status of a decomposition: which stages are complete and what stage is next.",
	}, svc.GetStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_decompositions",
		Description: "List all decompositions in the project, showing completion status for each.",
	}, svc.ListDecompositions)

	return server
}

// RunDecomposeMCPServerStdio runs the MCP server on stdio transport, blocking
// until stdin is closed or the context is cancelled.
func RunDecomposeMCPServerStdio(ctx context.Context, server *mcp.Server) error {
	return server.Run(ctx, &mcp.StdioTransport{})
}
