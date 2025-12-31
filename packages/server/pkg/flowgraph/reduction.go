package flowgraph

import (
	"slices"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

// DefaultMaxEdgesForReduction is the default threshold for skipping transitive reduction.
// Graphs with more edges than this will not be reduced (performance optimization).
const DefaultMaxEdgesForReduction = 2000

// ApplyTransitiveReduction removes redundant edges from the graph.
// An edge A->C is redundant if there exists a path A->...->C via other edges.
//
// If maxEdges is > 0 and len(edges) > maxEdges, the reduction is skipped for performance.
// Pass 0 to use DefaultMaxEdgesForReduction.
func ApplyTransitiveReduction(edges []mflow.Edge, maxEdges int) []mflow.Edge {
	if len(edges) == 0 {
		return edges
	}

	if maxEdges == 0 {
		maxEdges = DefaultMaxEdgesForReduction
	}

	// Performance optimization: Skip reduction for large graphs to avoid O(E^2) complexity
	if maxEdges > 0 && len(edges) > maxEdges {
		return edges
	}

	// Build adjacency map
	adjMap := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, edge := range edges {
		adjMap[edge.SourceID] = append(adjMap[edge.SourceID], edge.TargetID)
	}

	// For each edge, check if there's an alternative path
	var reducedEdges []mflow.Edge
	for _, edge := range edges {
		if !HasAlternativePath(adjMap, edge.SourceID, edge.TargetID) {
			reducedEdges = append(reducedEdges, edge)
		}
	}

	return reducedEdges
}

// HasAlternativePath checks if there's a path from source to target
// that doesn't use the direct edge (i.e., goes through other nodes).
func HasAlternativePath(adjMap map[idwrap.IDWrap][]idwrap.IDWrap, source, target idwrap.IDWrap) bool {
	visited := make(map[idwrap.IDWrap]bool)
	var queue []idwrap.IDWrap

	// Start from source, explore all neighbors except the direct target
	for _, neighbor := range adjMap[source] {
		if neighbor != target {
			queue = append(queue, neighbor)
			visited[neighbor] = true
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == target {
			return true // Found alternative path
		}

		for _, neighbor := range adjMap[current] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}

	return false
}

// ConnectOrphans creates edges from startNode to all nodes with no incoming edges.
// This ensures all nodes are reachable from the start node.
func ConnectOrphans(nodes []mflow.Node, edges []mflow.Edge, flowID, startNodeID idwrap.IDWrap) []mflow.Edge {
	// Build set of nodes that have incoming edges
	hasIncoming := make(map[idwrap.IDWrap]bool)
	for _, e := range edges {
		hasIncoming[e.TargetID] = true
	}

	// Connect orphan nodes to start
	result := slices.Clone(edges)

	for _, node := range nodes {
		if node.ID == startNodeID {
			continue
		}
		if !hasIncoming[node.ID] {
			result = append(result, mflow.Edge{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      node.ID,
				SourceHandler: mflow.HandleUnspecified,
			})
		}
	}

	return result
}
