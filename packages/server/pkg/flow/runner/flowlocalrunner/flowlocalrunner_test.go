package flowlocalrunner_test

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/runner"
	flowlocalrunner "the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
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
	predecessors := flowlocalrunner.BuildPredecessorMap(edgesMap)
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
		_ = flowlocalrunner.BuildPredecessorMap(edgesMap)
	}
}

func TestLoopNodeEmitsFinalSuccessStatus(t *testing.T) {
	nodeID := idwrap.NewNow()
	loopNode := nfor.New(nodeID, "loop", 0, time.Millisecond, mnfor.ErrorHandling_ERROR_HANDLING_IGNORE)
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID: loopNode,
	}
	edgesMap := make(edge.EdgesMap)
	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), idwrap.NewNow(), nodeID, nodeMap, edgesMap, 0)

	statusChan := make(chan runner.FlowNodeStatus, 8)
	flowStatusChan := make(chan runner.FlowStatus, 2)

	if err := flowRunner.Run(context.Background(), statusChan, flowStatusChan, nil); err != nil {
		t.Fatalf("flow runner returned error: %v", err)
	}

	var states []mnnode.NodeState
	for status := range statusChan {
		states = append(states, status.State)
	}

	if len(states) < 2 {
		t.Fatalf("expected at least 2 statuses (RUNNING and SUCCESS), got %d", len(states))
	}
	if states[0] != mnnode.NODE_STATE_RUNNING {
		t.Fatalf("expected first status to be RUNNING, got %v", states[0])
	}
	if states[len(states)-1] != mnnode.NODE_STATE_SUCCESS {
		t.Fatalf("expected final status to be SUCCESS, got %v", states[len(states)-1])
	}
}
