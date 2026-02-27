//go:build cgo

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dusk-indust/decompose/internal/graph"
)

// runAugment queries the persistent graph index and prints context for the
// given search pattern. Designed to be called from the PreToolUse hook script
// (must complete in <5s). Prints nothing and exits 0 if no graph exists.
func runAugment(projectRoot, pattern string) error {
	if pattern == "" {
		return nil
	}

	graphPath := filepath.Join(projectRoot, ".decompose", "graph")
	if _, err := os.Stat(graphPath); err != nil {
		return nil // no graph index, exit silently
	}

	store, err := graph.NewKuzuFileStore(graphPath)
	if err != nil {
		return nil // can't open graph, exit silently
	}
	defer store.Close()

	ctx := context.Background()

	// Query symbols matching the pattern.
	symbols, err := store.QuerySymbols(ctx, pattern, 10)
	if err != nil || len(symbols) == 0 {
		return nil // no matches
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Graph Context for %q\n\n", pattern))

	// Format symbol matches.
	sb.WriteString("**Symbols found:**\n")
	for _, sym := range symbols {
		sb.WriteString(fmt.Sprintf("- `%s %s` in `%s:%d`", sym.Kind, sym.Name, sym.FilePath, sym.StartLine))
		if sym.Exported {
			sb.WriteString(" (exported)")
		}
		sb.WriteString("\n")
	}

	// Get dependencies for the first matching symbol's file.
	// Downstream = follow imports from this file = what this file depends on.
	// Upstream = reverse imports = who imports this file.
	primaryFile := symbols[0].FilePath
	deps, err := store.GetDependencies(ctx, primaryFile, graph.DirectionDownstream, 2)
	if err == nil && len(deps) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Dependencies (imports from `%s`):**\n", primaryFile))
		for _, chain := range deps {
			if len(chain.Nodes) > 1 {
				sb.WriteString(fmt.Sprintf("- `%s`\n", chain.Nodes[len(chain.Nodes)-1]))
			}
		}
	}

	dependents, err := store.GetDependencies(ctx, primaryFile, graph.DirectionUpstream, 2)
	if err == nil && len(dependents) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Dependents (%d files import `%s`):**\n", len(dependents), primaryFile))
		shown := 0
		for _, chain := range dependents {
			if len(chain.Nodes) > 1 && shown < 8 {
				sb.WriteString(fmt.Sprintf("- `%s`\n", chain.Nodes[len(chain.Nodes)-1]))
				shown++
			}
		}
		if len(dependents) > 8 {
			sb.WriteString(fmt.Sprintf("- ... (%d more)\n", len(dependents)-8))
		}
	}

	// Get clusters.
	clusters, err := store.GetClusters(ctx)
	if err == nil {
		for _, c := range clusters {
			for _, member := range c.Members {
				if member == primaryFile {
					sb.WriteString(fmt.Sprintf("\n**Cluster:** %s (cohesion: %.2f) — %d files\n",
						c.Name, c.CohesionScore, len(c.Members)))
					break
				}
			}
		}
	}

	fmt.Print(sb.String())
	return nil
}
