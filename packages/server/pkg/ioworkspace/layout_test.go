package ioworkspace

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"

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

	// Should have 1 manual start node
	var startNodeCount int
	for _, node := range bundle.FlowNodes {
		if node.NodeKind == mflow.NODE_KIND_MANUAL_START {
			startNodeCount++
		}
	}
	if startNodeCount != 1 {
		t.Errorf("Expected 1 manual start node, got %d", startNodeCount)
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
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: requestNodeID, FlowID: flowID, Name: "Request 1", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: requestNodeID},
		},
		FlowEdges: []mflow.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: requestNodeID},
		},
	}

	err := bundle.EnsureFlowStructure()
	require.NoError(t, err, "EnsureFlowStructure failed")

	// Should still have 2 nodes (not create duplicate start)
	if len(bundle.FlowNodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(bundle.FlowNodes))
	}

	// Should still have 1 manual start node
	var startNodeCount int
	for _, node := range bundle.FlowNodes {
		if node.NodeKind == mflow.NODE_KIND_MANUAL_START {
			startNodeCount++
		}
	}
	if startNodeCount != 1 {
		t.Errorf("Expected 1 manual start node, got %d", startNodeCount)
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
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: node1ID, FlowID: flowID, Name: "Request 1", NodeKind: mflow.NODE_KIND_REQUEST},
			{ID: node2ID, FlowID: flowID, Name: "Request 2", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: node1ID},
			{FlowNodeID: node2ID},
		},
		FlowEdges: []mflow.Edge{
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

	// Horizontal layout: X increases with depth (300px spacing), Y stays at 0 for sequential nodes
	const spacingPrimary = 300 // X spacing between levels (matches DefaultHorizontalConfig)

	// Start node should be at level 0 (X = 0)
	startNode := nodeMap[startNodeID]
	if startNode.PositionX != 0 {
		t.Errorf("Start node X position should be 0, got %f", startNode.PositionX)
	}

	// Node1 should be at level 1 (X = 300)
	node1 := nodeMap[node1ID]
	if node1.PositionX != spacingPrimary {
		t.Errorf("Node1 X position should be %d, got %f", spacingPrimary, node1.PositionX)
	}

	// Node2 should be at level 2 (X = 600)
	node2 := nodeMap[node2ID]
	if node2.PositionX != 2*spacingPrimary {
		t.Errorf("Node2 X position should be %d, got %f", 2*spacingPrimary, node2.PositionX)
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
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: node1ID, FlowID: flowID, Name: "Request 1", NodeKind: mflow.NODE_KIND_REQUEST},
			{ID: node2ID, FlowID: flowID, Name: "Request 2", NodeKind: mflow.NODE_KIND_REQUEST},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{FlowNodeID: node1ID},
			{FlowNodeID: node2ID},
		},
		FlowEdges: []mflow.Edge{
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

	// Horizontal layout: parallel nodes have same X (level), different Y (vertical spread)
	const spacingPrimary = 300 // X spacing between levels (matches DefaultHorizontalConfig)

	node1 := nodeMap[node1ID]
	node2 := nodeMap[node2ID]

	// Both nodes should be at the same X level (level 1 = X=300)
	if node1.PositionX != node2.PositionX {
		t.Errorf("Parallel nodes should be at same X level, got %f and %f", node1.PositionX, node2.PositionX)
	}

	if node1.PositionX != spacingPrimary {
		t.Errorf("Parallel nodes should be at level 1 (X=%d), got %f", spacingPrimary, node1.PositionX)
	}

	// Nodes should have different Y positions (spread vertically)
	if node1.PositionY == node2.PositionY {
		t.Errorf("Parallel nodes should have different Y positions, both at %f", node1.PositionY)
	}
}
