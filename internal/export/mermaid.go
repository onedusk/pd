package export

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dusk-indust/decompose/internal/graph"
)

// GenerateMermaid produces a Mermaid graph TD diagram from a graph store.
// Files are grouped by cluster; IMPORTS edges become arrows.
func GenerateMermaid(ctx context.Context, store graph.Store) (string, error) {
	clusters, err := store.GetClusters(ctx)
	if err != nil {
		return "", fmt.Errorf("get clusters: %w", err)
	}

	edges, err := store.GetAllEdges(ctx)
	if err != nil {
		return "", fmt.Errorf("get edges: %w", err)
	}

	// Build node → ID mapping for Mermaid (alphanumeric only).
	nodeIDs := make(map[string]string)
	nextID := 0
	getID := func(path string) string {
		if id, ok := nodeIDs[path]; ok {
			return id
		}
		id := fmt.Sprintf("N%d", nextID)
		nextID++
		nodeIDs[path] = id
		return id
	}

	// Track which files are in clusters.
	clustered := make(map[string]string) // file → cluster name
	for _, c := range clusters {
		for _, member := range c.Members {
			clustered[member] = c.Name
		}
	}

	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Emit cluster subgraphs.
	for _, c := range clusters {
		if len(c.Members) == 0 {
			continue
		}
		sorted := make([]string, len(c.Members))
		copy(sorted, c.Members)
		sort.Strings(sorted)

		sb.WriteString(fmt.Sprintf("  subgraph %s[\"%.40s\"]\n", getID(c.Name+"_cluster"), c.Name))
		for _, member := range sorted {
			label := shortPath(member)
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", getID(member), label))
		}
		sb.WriteString("  end\n")
	}

	// Emit IMPORTS edges.
	for _, e := range edges {
		if e.Kind != graph.EdgeKindImports {
			continue
		}
		srcID := getID(e.SourceID)
		tgtID := getID(e.TargetID)
		sb.WriteString(fmt.Sprintf("  %s --> %s\n", srcID, tgtID))
	}

	return sb.String(), nil
}

// shortPath returns the last 2 path segments for readability.
func shortPath(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
