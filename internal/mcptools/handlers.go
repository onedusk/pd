package mcptools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dusk-indust/decompose/internal/graph"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CodeIntelService holds the graph store and parser used by MCP tool handlers.
type CodeIntelService struct {
	store       graph.Store
	parser      graph.Parser
	projectRoot string // used for persisting the graph to disk
}

// NewCodeIntelService creates a CodeIntelService with the given store and parser.
func NewCodeIntelService(store graph.Store, parser graph.Parser) *CodeIntelService {
	return &CodeIntelService{store: store, parser: parser}
}

// SetProjectRoot sets the project root used for graph persistence.
func (s *CodeIntelService) SetProjectRoot(root string) {
	s.projectRoot = root
}

// extToLanguage maps file extensions to graph.Language.
var extToLanguage = map[string]graph.Language{
	".go":  graph.LangGo,
	".ts":  graph.LangTypeScript,
	".tsx": graph.LangTypeScript,
	".py":  graph.LangPython,
	".rs":  graph.LangRust,
}

// BuildGraph walks a repository, parses source files, populates the graph store,
// and runs clustering. Returns graph statistics.
func (s *CodeIntelService) BuildGraph(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input BuildGraphInput,
) (*mcp.CallToolResult, BuildGraphOutput, error) {
	if input.RepoPath == "" {
		return nil, BuildGraphOutput{}, fmt.Errorf("repoPath is required")
	}

	info, err := os.Stat(input.RepoPath)
	if err != nil {
		return nil, BuildGraphOutput{}, fmt.Errorf("cannot access repoPath: %w", err)
	}
	if !info.IsDir() {
		return nil, BuildGraphOutput{}, fmt.Errorf("repoPath is not a directory: %s", input.RepoPath)
	}

	// Build allowed language set.
	allowedLangs := make(map[graph.Language]bool)
	if len(input.Languages) == 0 {
		for _, l := range graph.Tier1Languages {
			allowedLangs[l] = true
		}
	} else {
		for _, l := range input.Languages {
			allowedLangs[graph.Language(strings.ToLower(l))] = true
		}
	}

	// Build excluded directory set.
	excludeSet := make(map[string]bool, len(input.ExcludeDirs))
	for _, d := range input.ExcludeDirs {
		excludeSet[d] = true
	}

	if err := s.store.InitSchema(ctx); err != nil {
		return nil, BuildGraphOutput{}, fmt.Errorf("init schema: %w", err)
	}

	var files []graph.FileNode

	walkErr := filepath.WalkDir(input.RepoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || excludeSet[name] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		lang, ok := extToLanguage[ext]
		if !ok || !allowedLangs[lang] {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		relPath, err := filepath.Rel(input.RepoPath, path)
		if err != nil {
			relPath = path
		}

		result, err := s.parser.Parse(ctx, relPath, source, lang)
		if err != nil {
			return nil // skip unparseable files
		}

		if err := s.store.AddFile(ctx, result.File); err != nil {
			return fmt.Errorf("add file %s: %w", relPath, err)
		}
		files = append(files, result.File)

		for _, sym := range result.Symbols {
			if err := s.store.AddSymbol(ctx, sym); err != nil {
				return fmt.Errorf("add symbol %s: %w", sym.Name, err)
			}
		}

		for _, edge := range result.Edges {
			if err := s.store.AddEdge(ctx, edge); err != nil {
				return fmt.Errorf("add edge %s->%s: %w", edge.SourceID, edge.TargetID, err)
			}
		}

		return nil
	})
	if walkErr != nil {
		return nil, BuildGraphOutput{}, fmt.Errorf("walk: %w", walkErr)
	}

	// Run clustering on the indexed files.
	if _, err := graph.ComputeClusters(ctx, s.store, files); err != nil {
		return nil, BuildGraphOutput{}, fmt.Errorf("compute clusters: %w", err)
	}

	stats, err := s.store.Stats(ctx)
	if err != nil {
		return nil, BuildGraphOutput{}, fmt.Errorf("stats: %w", err)
	}

	// Persist graph to disk for the augment hook.
	if s.projectRoot != "" {
		persistPath := filepath.Join(s.projectRoot, ".decompose", "graph")
		if err := persistGraph(ctx, s.store, persistPath, files); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to persist graph: %v\n", err)
		}
	}

	return nil, BuildGraphOutput{Stats: *stats}, nil
}

// persistGraph copies graph data from the in-memory store to a file-based
// KuzuDB at persistPath. This enables the `augment` CLI command to query
// the graph without needing the MCP server running.
func persistGraph(ctx context.Context, src graph.Store, persistPath string, files []graph.FileNode) error {
	// Remove old graph to avoid stale data.
	os.RemoveAll(persistPath)

	dst, err := graph.NewKuzuFileStore(persistPath)
	if err != nil {
		return fmt.Errorf("open file store: %w", err)
	}
	defer dst.Close()

	if err := dst.InitSchema(ctx); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	for _, f := range files {
		if err := dst.AddFile(ctx, f); err != nil {
			return fmt.Errorf("add file %s: %w", f.Path, err)
		}
	}

	symbols, err := src.QuerySymbols(ctx, "", 100000)
	if err != nil {
		return fmt.Errorf("query symbols: %w", err)
	}
	for _, sym := range symbols {
		if err := dst.AddSymbol(ctx, sym); err != nil {
			return fmt.Errorf("add symbol %s: %w", sym.Name, err)
		}
	}

	clusters, err := src.GetClusters(ctx)
	if err != nil {
		return fmt.Errorf("get clusters: %w", err)
	}
	for _, c := range clusters {
		if err := dst.AddCluster(ctx, c); err != nil {
			return fmt.Errorf("add cluster %s: %w", c.Name, err)
		}
	}

	return nil
}

// QuerySymbols searches for symbols by name substring match.
func (s *CodeIntelService) QuerySymbols(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input QuerySymbolsInput,
) (*mcp.CallToolResult, QuerySymbolsOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	symbols, err := s.store.QuerySymbols(ctx, input.Query, limit)
	if err != nil {
		return nil, QuerySymbolsOutput{}, fmt.Errorf("query symbols: %w", err)
	}

	// Filter by kind if specified.
	if input.Kind != "" {
		kind := graph.SymbolKind(strings.ToLower(input.Kind))
		filtered := symbols[:0]
		for _, sym := range symbols {
			if sym.Kind == kind {
				filtered = append(filtered, sym)
			}
		}
		symbols = filtered
	}

	return nil, QuerySymbolsOutput{
		Symbols: symbols,
		Total:   len(symbols),
	}, nil
}

// GetDependencies traverses the dependency graph from a given node.
func (s *CodeIntelService) GetDependencies(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input GetDependenciesInput,
) (*mcp.CallToolResult, GetDependenciesOutput, error) {
	if input.NodeID == "" {
		return nil, GetDependenciesOutput{}, fmt.Errorf("nodeId is required")
	}

	direction := graph.DirectionDownstream
	if strings.EqualFold(input.Direction, "upstream") {
		direction = graph.DirectionUpstream
	}

	maxDepth := input.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 5
	}

	chains, err := s.store.GetDependencies(ctx, input.NodeID, direction, maxDepth)
	if err != nil {
		return nil, GetDependenciesOutput{}, fmt.Errorf("get dependencies: %w", err)
	}

	return nil, GetDependenciesOutput{Chains: chains}, nil
}

// AssessImpact computes the blast radius of modifying a set of files.
func (s *CodeIntelService) AssessImpact(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input AssessImpactInput,
) (*mcp.CallToolResult, AssessImpactOutput, error) {
	if len(input.ChangedFiles) == 0 {
		return nil, AssessImpactOutput{}, fmt.Errorf("changedFiles is required")
	}

	impact, err := s.store.AssessImpact(ctx, input.ChangedFiles)
	if err != nil {
		return nil, AssessImpactOutput{}, fmt.Errorf("assess impact: %w", err)
	}

	return nil, AssessImpactOutput{Impact: *impact}, nil
}

// GetClusters returns all file clusters in the graph.
func (s *CodeIntelService) GetClusters(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ GetClustersInput,
) (*mcp.CallToolResult, GetClustersOutput, error) {
	clusters, err := s.store.GetClusters(ctx)
	if err != nil {
		return nil, GetClustersOutput{}, fmt.Errorf("get clusters: %w", err)
	}

	return nil, GetClustersOutput{Clusters: clusters}, nil
}
