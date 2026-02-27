package graph

import (
	"context"
	"strings"
)

// ComputeClusters finds connected components in the file-to-file graph
// (IMPORTS edges only) and stores them as ClusterNodes.
//
// Algorithm:
//  1. Build an undirected adjacency list from IMPORTS edges among the given files.
//  2. Find connected components via BFS.
//  3. For each component with >= 2 files, compute a cohesion score and store the cluster.
func ComputeClusters(ctx context.Context, store Store, files []FileNode) ([]ClusterNode, error) {
	filePaths := make(map[string]bool, len(files))
	for _, f := range files {
		filePaths[f.Path] = true
	}

	// Retrieve stats to get edges (we need to query dependencies).
	// Since the Store interface does not expose a ListEdges method,
	// we use GetDependencies per file to build the adjacency list.
	// However, that would be expensive. Instead, for MemStore usage
	// we build adjacency from individual file dependency queries.

	adj := buildAdjacency(ctx, store, files)

	// BFS to find connected components.
	visited := make(map[string]bool, len(files))
	var clusters []ClusterNode

	for _, f := range files {
		if visited[f.Path] {
			continue
		}
		component := bfsComponent(f.Path, adj, visited)
		if len(component) < 2 {
			continue
		}
		cohesion := computeCohesion(component, adj, filePaths)
		name := longestCommonPrefix(component)
		cluster := ClusterNode{
			Name:          name,
			CohesionScore: cohesion,
			Members:       component,
		}
		if err := store.AddCluster(ctx, cluster); err != nil {
			return nil, err
		}
		// Add BELONGS edges for each member.
		for _, member := range component {
			edge := Edge{
				SourceID: member,
				TargetID: name,
				Kind:     EdgeKindBelongs,
			}
			if err := store.AddEdge(ctx, edge); err != nil {
				return nil, err
			}
		}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

// buildAdjacency constructs a bidirectional adjacency list from IMPORTS edges
// using a single pass over all edges (O(E) instead of O(N*E)).
func buildAdjacency(ctx context.Context, store Store, files []FileNode) map[string]map[string]bool {
	adj := make(map[string]map[string]bool, len(files))
	for _, f := range files {
		adj[f.Path] = make(map[string]bool)
	}

	// Single pass: retrieve all edges and filter to IMPORTS between known files.
	edges, err := store.GetAllEdges(ctx)
	if err != nil {
		return adj
	}
	for _, e := range edges {
		if e.Kind != EdgeKindImports {
			continue
		}
		// Only include edges between known files.
		if adj[e.SourceID] != nil && adj[e.TargetID] != nil {
			adj[e.SourceID][e.TargetID] = true
			adj[e.TargetID][e.SourceID] = true
		}
	}

	return adj
}

// bfsComponent performs BFS from start on the adjacency list and returns
// all reachable nodes. It marks visited nodes as it goes.
func bfsComponent(start string, adj map[string]map[string]bool, visited map[string]bool) []string {
	var component []string
	queue := []string{start}
	visited[start] = true

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		component = append(component, node)
		for neighbor := range adj[node] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return component
}

// computeCohesion calculates internal_edges / (internal_edges + external_edges)
// for a connected component. Internal edges connect two members; external edges
// connect a member to a non-member.
func computeCohesion(component []string, adj map[string]map[string]bool, allFiles map[string]bool) float64 {
	memberSet := make(map[string]bool, len(component))
	for _, m := range component {
		memberSet[m] = true
	}

	internalEdges := 0
	externalEdges := 0

	// Count each undirected edge once by only counting when source < target
	// for internal, and always counting outbound for external.
	for _, m := range component {
		for neighbor := range adj[m] {
			if memberSet[neighbor] {
				// Count each internal edge once (when m < neighbor alphabetically).
				if m < neighbor {
					internalEdges++
				}
			} else if allFiles[neighbor] {
				externalEdges++
			}
		}
	}

	total := internalEdges + externalEdges
	if total == 0 {
		return 0
	}
	return float64(internalEdges) / float64(total)
}

// longestCommonPrefix finds the longest common path prefix among a set of
// file paths. Returns an empty string if no common prefix is found.
func longestCommonPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return paths[0]
	}

	prefix := paths[0]
	for _, p := range paths[1:] {
		for !strings.HasPrefix(p, prefix) {
			// Trim to the last path separator (excluding any trailing slash).
			trimmed := strings.TrimRight(prefix, "/")
			idx := strings.LastIndex(trimmed, "/")
			if idx < 0 {
				return ""
			}
			prefix = trimmed[:idx+1] // keep the trailing slash
			if prefix == "/" || prefix == "" {
				return prefix
			}
		}
	}

	// Ensure prefix ends at a directory boundary.
	if !strings.HasSuffix(prefix, "/") {
		idx := strings.LastIndex(prefix, "/")
		if idx >= 0 {
			prefix = prefix[:idx+1]
		}
	}

	return prefix
}
