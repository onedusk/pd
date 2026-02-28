//go:build cgo

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dusk-indust/decompose/internal/export"
	"github.com/dusk-indust/decompose/internal/graph"
)

func runDiagram(projectRoot string) error {
	graphPath := filepath.Join(projectRoot, ".decompose", "graph")
	if _, err := os.Stat(graphPath); err != nil {
		return fmt.Errorf("no graph found at %s\nRun 'build_graph' via MCP first to index the codebase", graphPath)
	}

	store, err := graph.NewKuzuFileStore(graphPath)
	if err != nil {
		return fmt.Errorf("open graph: %w", err)
	}
	defer store.Close()

	ctx := context.Background()
	mermaid, err := export.GenerateMermaid(ctx, store)
	if err != nil {
		return err
	}

	fmt.Print(mermaid)
	return nil
}
