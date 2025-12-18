package ioworkspace

import (
	"testing"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"

	"github.com/stretchr/testify/require"
)

func TestEnsureFlowStructure_CreatesStartNode(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: flowID, Name: "Test Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: nodeID, FlowID: flowID, Name: "Request 1", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: nodeID},
		},
	}

	err := bundle.EnsureFlowStructure()
	require.NoError(t, err, "EnsureFlowStructure failed")

	// Should have 2 nodes now (start + request)
	if len(bundle.FlowNodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(bundle.FlowNodes))
	}

	// Should have 1 noop node (start)
	if len(bundle.FlowNoopNodes) != 1 {
		t.Errorf("Expected 1 noop node, got %d", len(bundle.FlowNoopNodes))
	}

	// Should have start node type
	if bundle.FlowNoopNodes[0].Type != mflow.NODE_NO_OP_KIND_START {
		t.Errorf("Expected start node type, got %d", bundle.FlowNoopNodes[0].Type)
	}

	// Should NOT have any edges - orphan nodes are intentionally left disconnected
	// They will not execute, which is the expected behavior for disconnected nodes
	if len(bundle.FlowEdges) != 0 {
		t.Errorf("Expected 0 edges (orphan nodes should not be auto-connected), got %d", len(bundle.FlowEdges))
	}
}

func TestEnsureFlowStructure_DoesNotDuplicateStartNode(t *testing.T) {
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: flowID, Name: "Test Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_NO_OP},
			{ID: requestNodeID, FlowID: flowID, Name: "Request 1", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowNoopNodes: []mflow.NodeNoop{
			{FlowNodeID: startNodeID, Type: mflow.NODE_NO_OP_KIND_START},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: requestNodeID},
		},
		FlowEdges: []edge.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: requestNodeID},
		},
	}

	err := bundle.EnsureFlowStructure()
	require.NoError(t, err, "EnsureFlowStructure failed")

	// Should still have 2 nodes (not create duplicate start)
	if len(bundle.FlowNodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(bundle.FlowNodes))
	}

	// Should still have 1 noop node
	if len(bundle.FlowNoopNodes) != 1 {
		t.Errorf("Expected 1 noop node, got %d", len(bundle.FlowNoopNodes))
	}

	// Should still have 1 edge
	if len(bundle.FlowEdges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(bundle.FlowEdges))
	}
}

func TestEnsureFlowStructure_PositionsNodes(t *testing.T) {
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: flowID, Name: "Test Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_NO_OP},
			{ID: node1ID, FlowID: flowID, Name: "Request 1", NodeKind: mflow.NODE_KIND_REQUEST},
			{ID: node2ID, FlowID: flowID, Name: "Request 2", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowNoopNodes: []mflow.NodeNoop{
			{FlowNodeID: startNodeID, Type: mflow.NODE_NO_OP_KIND_START},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: node1ID},
			{FlowNodeID: node2ID},
		},
		FlowEdges: []edge.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: node1ID},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: node1ID, TargetID: node2ID},
		},
	}

	err := bundle.EnsureFlowStructure()
	require.NoError(t, err, "EnsureFlowStructure failed")

	// Find nodes by ID and check positions
	nodeMap := make(map[idwrap.IDWrap]*mflow.Node)
	for i := range bundle.FlowNodes {
		nodeMap[bundle.FlowNodes[i].ID] = &bundle.FlowNodes[i]
	}

	// Start node should be at level 0 (Y = 0)
	startNode := nodeMap[startNodeID]
	if startNode.PositionY != 0 {
		t.Errorf("Start node Y position should be 0, got %f", startNode.PositionY)
	}

	// Node1 should be at level 1 (Y = NodeSpacingY)
	node1 := nodeMap[node1ID]
	if node1.PositionY != NodeSpacingY {
		t.Errorf("Node1 Y position should be %d, got %f", NodeSpacingY, node1.PositionY)
	}

	// Node2 should be at level 2 (Y = 2*NodeSpacingY)
	node2 := nodeMap[node2ID]
	if node2.PositionY != 2*NodeSpacingY {
		t.Errorf("Node2 Y position should be %d, got %f", 2*NodeSpacingY, node2.PositionY)
	}
}

func TestEnsureFlowStructure_ParallelNodes(t *testing.T) {
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	node1ID := idwrap.NewNow()
	node2ID := idwrap.NewNow()

	// Create parallel structure: start -> node1, start -> node2
	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: flowID, Name: "Test Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_NO_OP},
			{ID: node1ID, FlowID: flowID, Name: "Request 1", NodeKind: mflow.NODE_KIND_REQUEST},
			{ID: node2ID, FlowID: flowID, Name: "Request 2", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowNoopNodes: []mflow.NodeNoop{
			{FlowNodeID: startNodeID, Type: mflow.NODE_NO_OP_KIND_START},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: node1ID},
			{FlowNodeID: node2ID},
		},
		FlowEdges: []edge.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: node1ID},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: node2ID},
		},
	}

	err := bundle.EnsureFlowStructure()
	require.NoError(t, err, "EnsureFlowStructure failed")

	// Find nodes by ID
	nodeMap := make(map[idwrap.IDWrap]*mflow.Node)
	for i := range bundle.FlowNodes {
		nodeMap[bundle.FlowNodes[i].ID] = &bundle.FlowNodes[i]
	}

	// Both node1 and node2 should be at the same Y level (level 1)
	node1 := nodeMap[node1ID]
	node2 := nodeMap[node2ID]

	if node1.PositionY != node2.PositionY {
		t.Errorf("Parallel nodes should be at same Y level, got %f and %f", node1.PositionY, node2.PositionY)
	}

	if node1.PositionY != NodeSpacingY {
		t.Errorf("Parallel nodes should be at level 1 (Y=%d), got %f", NodeSpacingY, node1.PositionY)
	}

	// Nodes should have different X positions
	if node1.PositionX == node2.PositionX {
		t.Errorf("Parallel nodes should have different X positions, both at %f", node1.PositionX)
	}
}
