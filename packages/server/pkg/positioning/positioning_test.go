package positioning_test

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/positioning"
)

func TestPositionNodes(t *testing.T) {
	t.Run("Simple Linear Flow", func(t *testing.T) {
		// Create nodes
		startID := idwrap.NewNow()
		node1ID := idwrap.NewNow()
		node2ID := idwrap.NewNow()
		
		nodes := []mnnode.MNode{
			{ID: startID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: node1ID, Name: "Node1", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node2ID, Name: "Node2", NodeKind: mnnode.NODE_KIND_REQUEST},
		}
		
		// Create edges: Start -> Node1 -> Node2
		edges := []edge.Edge{
			{ID: idwrap.NewNow(), SourceID: startID, TargetID: node1ID},
			{ID: idwrap.NewNow(), SourceID: node1ID, TargetID: node2ID},
		}
		
		// Create noop nodes
		noopNodes := []mnnoop.NoopNode{
			{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START},
		}
		
		// Position nodes
		positioner := positioning.NewNodePositioner()
		err := positioner.PositionNodes(nodes, edges, noopNodes)
		if err != nil {
			t.Fatalf("Failed to position nodes: %v", err)
		}
		
		// Verify positions
		// Start should be at level 0 (Y=0)
		if nodes[0].PositionY != 0 {
			t.Errorf("Start node Y position: got %f, want 0", nodes[0].PositionY)
		}
		
		// Node1 should be at level 1 (Y=300)
		if nodes[1].PositionY != 300 {
			t.Errorf("Node1 Y position: got %f, want 300", nodes[1].PositionY)
		}
		
		// Node2 should be at level 2 (Y=600)
		if nodes[2].PositionY != 600 {
			t.Errorf("Node2 Y position: got %f, want 600", nodes[2].PositionY)
		}
	})
	
	t.Run("Parallel Nodes", func(t *testing.T) {
		// Create nodes
		startID := idwrap.NewNow()
		node1ID := idwrap.NewNow()
		node2ID := idwrap.NewNow()
		node3ID := idwrap.NewNow()
		
		nodes := []mnnode.MNode{
			{ID: startID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: node1ID, Name: "Node1", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node2ID, Name: "Node2", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node3ID, Name: "Node3", NodeKind: mnnode.NODE_KIND_REQUEST},
		}
		
		// Create edges: Start -> Node1, Start -> Node2, Node1 -> Node3, Node2 -> Node3
		edges := []edge.Edge{
			{ID: idwrap.NewNow(), SourceID: startID, TargetID: node1ID},
			{ID: idwrap.NewNow(), SourceID: startID, TargetID: node2ID},
			{ID: idwrap.NewNow(), SourceID: node1ID, TargetID: node3ID},
			{ID: idwrap.NewNow(), SourceID: node2ID, TargetID: node3ID},
		}
		
		// Create noop nodes
		noopNodes := []mnnoop.NoopNode{
			{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START},
		}
		
		// Position nodes
		positioner := positioning.NewNodePositioner()
		err := positioner.PositionNodes(nodes, edges, noopNodes)
		if err != nil {
			t.Fatalf("Failed to position nodes: %v", err)
		}
		
		// Verify positions
		// Node1 and Node2 should be at the same Y level (300)
		if nodes[1].PositionY != nodes[2].PositionY {
			t.Errorf("Node1 and Node2 should be at same Y level: %f vs %f", nodes[1].PositionY, nodes[2].PositionY)
		}
		
		// Node3 should be at a deeper level than Node1 and Node2
		if nodes[3].PositionY <= nodes[1].PositionY {
			t.Errorf("Node3 should be deeper than Node1: %f <= %f", nodes[3].PositionY, nodes[1].PositionY)
		}
		
		// Node1 and Node2 should have different X positions
		if nodes[1].PositionX == nodes[2].PositionX {
			t.Errorf("Node1 and Node2 should have different X positions")
		}
	})
	
	t.Run("No Start Node", func(t *testing.T) {
		// Create nodes without explicit start node
		node1ID := idwrap.NewNow()
		node2ID := idwrap.NewNow()
		node3ID := idwrap.NewNow()
		
		nodes := []mnnode.MNode{
			{ID: node1ID, Name: "Node1", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node2ID, Name: "Node2", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node3ID, Name: "Node3", NodeKind: mnnode.NODE_KIND_REQUEST},
		}
		
		// Create edges: Node1 -> Node3, Node2 -> Node3
		edges := []edge.Edge{
			{ID: idwrap.NewNow(), SourceID: node1ID, TargetID: node3ID},
			{ID: idwrap.NewNow(), SourceID: node2ID, TargetID: node3ID},
		}
		
		// No noop nodes
		noopNodes := []mnnoop.NoopNode{}
		
		// Position nodes
		positioner := positioning.NewNodePositioner()
		err := positioner.PositionNodes(nodes, edges, noopNodes)
		if err != nil {
			t.Fatalf("Failed to position nodes: %v", err)
		}
		
		// Verify that nodes are positioned
		for i, node := range nodes {
			if node.PositionX == 0 && node.PositionY == 0 && i > 0 {
				t.Errorf("Node %s seems unpositioned", node.Name)
			}
		}
		
		// Node1 and Node2 should be at level 0 (root nodes)
		if nodes[0].PositionY != 0 || nodes[1].PositionY != 0 {
			t.Errorf("Root nodes should be at Y=0: Node1=%f, Node2=%f", nodes[0].PositionY, nodes[1].PositionY)
		}
		
		// Node3 should be at level 1
		if nodes[2].PositionY != 300 {
			t.Errorf("Node3 should be at Y=300, got %f", nodes[2].PositionY)
		}
	})
}