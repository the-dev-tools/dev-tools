package positioning_test

import (
	"testing"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/positioning"
	"time"
)

func TestPositionNodesWithCycles(t *testing.T) {
	t.Run("Simple Cycle", func(t *testing.T) {
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
		
		// Create edges with a cycle: Start -> Node1 -> Node2 -> Node3 -> Node1
		edges := []edge.Edge{
			{ID: idwrap.NewNow(), SourceID: startID, TargetID: node1ID},
			{ID: idwrap.NewNow(), SourceID: node1ID, TargetID: node2ID},
			{ID: idwrap.NewNow(), SourceID: node2ID, TargetID: node3ID},
			{ID: idwrap.NewNow(), SourceID: node3ID, TargetID: node1ID}, // Cycle back to Node1
		}
		
		// Create noop nodes
		noopNodes := []mnnoop.NoopNode{
			{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START},
		}
		
		// Position nodes with timeout to detect deadlock
		positioner := positioning.NewNodePositioner()
		done := make(chan error, 1)
		
		go func() {
			done <- positioner.PositionNodes(nodes, edges, noopNodes)
		}()
		
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Failed to position nodes: %v", err)
			}
			// Verify that nodes are positioned
			for _, node := range nodes {
				t.Logf("Node %s positioned at (%f, %f)", node.Name, node.PositionX, node.PositionY)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Positioning timed out - possible deadlock")
		}
	})
	
	t.Run("Complex Cycle with Multiple Paths", func(t *testing.T) {
		// Create nodes
		startID := idwrap.NewNow()
		nodeAID := idwrap.NewNow()
		nodeBID := idwrap.NewNow()
		nodeCID := idwrap.NewNow()
		nodeDID := idwrap.NewNow()
		
		nodes := []mnnode.MNode{
			{ID: startID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: nodeAID, Name: "NodeA", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: nodeBID, Name: "NodeB", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: nodeCID, Name: "NodeC", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: nodeDID, Name: "NodeD", NodeKind: mnnode.NODE_KIND_REQUEST},
		}
		
		// Create complex edges with cycles
		// Start -> A -> B -> D
		//      \-> C -> B
		//          D -> C (cycle)
		edges := []edge.Edge{
			{ID: idwrap.NewNow(), SourceID: startID, TargetID: nodeAID},
			{ID: idwrap.NewNow(), SourceID: startID, TargetID: nodeCID},
			{ID: idwrap.NewNow(), SourceID: nodeAID, TargetID: nodeBID},
			{ID: idwrap.NewNow(), SourceID: nodeCID, TargetID: nodeBID},
			{ID: idwrap.NewNow(), SourceID: nodeBID, TargetID: nodeDID},
			{ID: idwrap.NewNow(), SourceID: nodeDID, TargetID: nodeCID}, // Cycle
		}
		
		// Create noop nodes
		noopNodes := []mnnoop.NoopNode{
			{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START},
		}
		
		// Position nodes with timeout
		positioner := positioning.NewNodePositioner()
		done := make(chan error, 1)
		
		go func() {
			done <- positioner.PositionNodes(nodes, edges, noopNodes)
		}()
		
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Failed to position nodes: %v", err)
			}
			// Verify positioning
			t.Log("Complex cycle test completed without deadlock")
		case <-time.After(2 * time.Second):
			t.Fatal("Positioning timed out - possible deadlock")
		}
	})

	t.Run("Self-referencing Node", func(t *testing.T) {
		// Create nodes
		startID := idwrap.NewNow()
		node1ID := idwrap.NewNow()
		
		nodes := []mnnode.MNode{
			{ID: startID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: node1ID, Name: "Node1", NodeKind: mnnode.NODE_KIND_REQUEST},
		}
		
		// Create edges with self-reference
		edges := []edge.Edge{
			{ID: idwrap.NewNow(), SourceID: startID, TargetID: node1ID},
			{ID: idwrap.NewNow(), SourceID: node1ID, TargetID: node1ID}, // Self-reference
		}
		
		// Create noop nodes
		noopNodes := []mnnoop.NoopNode{
			{FlowNodeID: startID, Type: mnnoop.NODE_NO_OP_KIND_START},
		}
		
		// Position nodes
		positioner := positioning.NewNodePositioner()
		done := make(chan error, 1)
		
		go func() {
			done <- positioner.PositionNodes(nodes, edges, noopNodes)
		}()
		
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Failed to position nodes: %v", err)
			}
			t.Log("Self-referencing node test completed")
		case <-time.After(2 * time.Second):
			t.Fatal("Positioning timed out - possible deadlock")
		}
	})

	t.Run("Disconnected Graph with Cycles", func(t *testing.T) {
		// Create multiple disconnected components with cycles
		node1ID := idwrap.NewNow()
		node2ID := idwrap.NewNow()
		node3ID := idwrap.NewNow()
		node4ID := idwrap.NewNow()
		
		nodes := []mnnode.MNode{
			{ID: node1ID, Name: "Node1", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node2ID, Name: "Node2", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node3ID, Name: "Node3", NodeKind: mnnode.NODE_KIND_REQUEST},
			{ID: node4ID, Name: "Node4", NodeKind: mnnode.NODE_KIND_REQUEST},
		}
		
		// Create two disconnected cycles
		edges := []edge.Edge{
			// First cycle: 1 -> 2 -> 1
			{ID: idwrap.NewNow(), SourceID: node1ID, TargetID: node2ID},
			{ID: idwrap.NewNow(), SourceID: node2ID, TargetID: node1ID},
			// Second cycle: 3 -> 4 -> 3
			{ID: idwrap.NewNow(), SourceID: node3ID, TargetID: node4ID},
			{ID: idwrap.NewNow(), SourceID: node4ID, TargetID: node3ID},
		}
		
		// No noop nodes
		noopNodes := []mnnoop.NoopNode{}
		
		// Position nodes
		positioner := positioning.NewNodePositioner()
		done := make(chan error, 1)
		
		go func() {
			done <- positioner.PositionNodes(nodes, edges, noopNodes)
		}()
		
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Failed to position nodes: %v", err)
			}
			t.Log("Disconnected graph with cycles test completed")
		case <-time.After(2 * time.Second):
			t.Fatal("Positioning timed out - possible deadlock")
		}
	})
}