package flowlocalrunner

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
)

func legacyGetPredecessorNodes(nodeID idwrap.IDWrap, edgesMap edge.EdgesMap) []idwrap.IDWrap {
	var predecessors []idwrap.IDWrap
	seen := make(map[idwrap.IDWrap]bool)

	for sourceID, edges := range edgesMap {
		for _, targetNodes := range edges {
			for _, targetID := range targetNodes {
				if targetID == nodeID && !seen[sourceID] {
					predecessors = append(predecessors, sourceID)
					seen[sourceID] = true
				}
			}
		}
	}

	return predecessors
}

func buildDenseEdges(nodeCount int, fanout int) edge.EdgesMap {
	nodes := make([]idwrap.IDWrap, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = idwrap.NewNow()
	}

	var edges []edge.Edge
	for i := 0; i < nodeCount; i++ {
		for j := 1; j <= fanout; j++ {
			targetIndex := (i + j) % nodeCount
			edges = append(edges, edge.NewEdge(idwrap.NewNow(), nodes[i], nodes[targetIndex], edge.HandleUnspecified, int32(edge.EdgeKindNoOp)))
		}
	}

	return edge.NewEdgesMap(edges)
}

func BenchmarkLegacyPredecessorLookup(b *testing.B) {
	edgesMap := buildDenseEdges(100, 4)
	var targets []idwrap.IDWrap
	for id := range edgesMap {
		targets = append(targets, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, target := range targets {
			_ = legacyGetPredecessorNodes(target, edgesMap)
		}
	}
}

func BenchmarkCachedPredecessorLookup(b *testing.B) {
	edgesMap := buildDenseEdges(100, 4)
	predecessors := buildPredecessorMap(edgesMap)
	var targets []idwrap.IDWrap
	for id := range edgesMap {
		targets = append(targets, id)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, target := range targets {
			_ = predecessors[target]
		}
	}
}

func BenchmarkBuildPredecessorMap(b *testing.B) {
	edgesMap := buildDenseEdges(100, 4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildPredecessorMap(edgesMap)
	}
}
