package flowgraph

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestLayout_SingleNode(t *testing.T) {
	startID := idwrap.NewNow()
	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
	}

	config := DefaultHorizontalConfig()
	result, err := Layout(nodes, nil, startID, config)
	if err != nil {
		t.Fatalf("Layout failed: %v", err)
	}

	if len(result.Positions) != 1 {
		t.Errorf("Expected 1 position, got %d", len(result.Positions))
	}

	pos := result.Positions[startID]
	if pos.X != 0 || pos.Y != 0 {
		t.Errorf("Expected position (0, 0), got (%f, %f)", pos.X, pos.Y)
	}

	if result.Levels[startID] != 0 {
		t.Errorf("Expected level 0, got %d", result.Levels[startID])
	}
}

func TestLayout_SequentialChain(t *testing.T) {
	// Start -> A -> B -> C
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	nodeC := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: nodeB, Name: "B", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: nodeC, Name: "C", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeA},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeB, TargetID: nodeC},
	}

	config := DefaultHorizontalConfig()
	result, err := Layout(nodes, edges, startID, config)
	if err != nil {
		t.Fatalf("Layout failed: %v", err)
	}

	// Check levels
	expectedLevels := map[idwrap.IDWrap]int{
		startID: 0,
		nodeA:   1,
		nodeB:   2,
		nodeC:   3,
	}

	for id, expectedLevel := range expectedLevels {
		if result.Levels[id] != expectedLevel {
			t.Errorf("Node level mismatch: expected %d, got %d", expectedLevel, result.Levels[id])
		}
	}

	// Check horizontal positions (X increases with level)
	if result.Positions[startID].X != 0 {
		t.Errorf("Start X should be 0, got %f", result.Positions[startID].X)
	}
	if result.Positions[nodeA].X != 300 {
		t.Errorf("Node A X should be 300, got %f", result.Positions[nodeA].X)
	}
	if result.Positions[nodeB].X != 600 {
		t.Errorf("Node B X should be 600, got %f", result.Positions[nodeB].X)
	}
	if result.Positions[nodeC].X != 900 {
		t.Errorf("Node C X should be 900, got %f", result.Positions[nodeC].X)
	}

	// All Y should be 0 (single node per level)
	for _, node := range nodes {
		if result.Positions[node.ID].Y != 0 {
			t.Errorf("Node %s Y should be 0, got %f", node.Name, result.Positions[node.ID].Y)
		}
	}
}

func TestLayout_ParallelNodes(t *testing.T) {
	// Start -> [A, B] (parallel)
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: nodeB, Name: "B", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeA},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeB},
	}

	config := DefaultHorizontalConfig()
	result, err := Layout(nodes, edges, startID, config)
	if err != nil {
		t.Fatalf("Layout failed: %v", err)
	}

	// Both A and B should be at level 1
	if result.Levels[nodeA] != 1 || result.Levels[nodeB] != 1 {
		t.Errorf("Parallel nodes should be at level 1")
	}

	// Both should have same X (300)
	if result.Positions[nodeA].X != 300 || result.Positions[nodeB].X != 300 {
		t.Errorf("Parallel nodes should have same X")
	}

	// Y should be centered: -75 and +75 (spacing 150)
	posA := result.Positions[nodeA]
	posB := result.Positions[nodeB]
	if posA.Y != -75 || posB.Y != 75 {
		t.Errorf("Expected Y positions -75 and 75, got %f and %f", posA.Y, posB.Y)
	}
}

func TestLayout_DiamondPattern(t *testing.T) {
	// Start -> [A, B] -> End
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	endID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: nodeB, Name: "B", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: endID, Name: "End", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeA},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeB},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: endID},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeB, TargetID: endID},
	}

	config := DefaultHorizontalConfig()
	result, err := Layout(nodes, edges, startID, config)
	if err != nil {
		t.Fatalf("Layout failed: %v", err)
	}

	// End should be at level 2 (max(1, 1) + 1)
	if result.Levels[endID] != 2 {
		t.Errorf("End node should be at level 2, got %d", result.Levels[endID])
	}

	// End X should be 600
	if result.Positions[endID].X != 600 {
		t.Errorf("End X should be 600, got %f", result.Positions[endID].X)
	}
}

func TestLayout_VerticalOrientation(t *testing.T) {
	// Start -> A -> B
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: nodeB, Name: "B", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeA},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
	}

	config := DefaultVerticalConfig()
	result, err := Layout(nodes, edges, startID, config)
	if err != nil {
		t.Fatalf("Layout failed: %v", err)
	}

	// Y should increase with level (vertical flow)
	if result.Positions[startID].Y != 0 {
		t.Errorf("Start Y should be 0, got %f", result.Positions[startID].Y)
	}
	if result.Positions[nodeA].Y != 300 {
		t.Errorf("Node A Y should be 300, got %f", result.Positions[nodeA].Y)
	}
	if result.Positions[nodeB].Y != 600 {
		t.Errorf("Node B Y should be 600, got %f", result.Positions[nodeB].Y)
	}

	// All X should be 0 (single node per level)
	for _, node := range nodes {
		if result.Positions[node.ID].X != 0 {
			t.Errorf("Node %s X should be 0, got %f", node.Name, result.Positions[node.ID].X)
		}
	}
}

func TestApplyLayout(t *testing.T) {
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	result := &LayoutResult{
		Positions: map[idwrap.IDWrap]Position{
			startID: {X: 0, Y: 0},
			nodeA:   {X: 100, Y: 50},
		},
	}

	ApplyLayout(nodes, result)

	if nodes[0].PositionX != 0 || nodes[0].PositionY != 0 {
		t.Errorf("Start position not applied correctly")
	}
	if nodes[1].PositionX != 100 || nodes[1].PositionY != 50 {
		t.Errorf("Node A position not applied correctly")
	}
}

func TestTransitiveReduction_RemovesRedundantEdge(t *testing.T) {
	// A -> B -> C and A -> C (redundant)
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	nodeC := idwrap.NewNow()
	flowID := idwrap.NewNow()

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeB, TargetID: nodeC},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeC}, // Redundant
	}

	reduced := ApplyTransitiveReduction(edges, 0)

	if len(reduced) != 2 {
		t.Errorf("Expected 2 edges after reduction, got %d", len(reduced))
	}

	// Verify A->C is removed
	for _, e := range reduced {
		if e.SourceID == nodeA && e.TargetID == nodeC {
			t.Errorf("Redundant edge A->C should be removed")
		}
	}
}

func TestTransitiveReduction_KeepsNecessaryEdges(t *testing.T) {
	// A -> B, A -> C (parallel, both necessary)
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	nodeC := idwrap.NewNow()
	flowID := idwrap.NewNow()

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeC},
	}

	reduced := ApplyTransitiveReduction(edges, 0)

	if len(reduced) != 2 {
		t.Errorf("Expected 2 edges (both necessary), got %d", len(reduced))
	}
}

func TestTransitiveReduction_SkipsLargeGraphs(t *testing.T) {
	// Create more edges than maxEdges
	flowID := idwrap.NewNow()
	var edges []mflow.Edge
	for i := 0; i < 10; i++ {
		edges = append(edges, mflow.Edge{
			ID:       idwrap.NewNow(),
			FlowID:   flowID,
			SourceID: idwrap.NewNow(),
			TargetID: idwrap.NewNow(),
		})
	}

	// With maxEdges = 5, should skip reduction
	reduced := ApplyTransitiveReduction(edges, 5)

	if len(reduced) != 10 {
		t.Errorf("Should skip reduction for large graphs, got %d edges", len(reduced))
	}
}

func TestLinearizeNodes(t *testing.T) {
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	nodeC := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: nodeB, Name: "B", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: nodeC, Name: "C", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeA},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeB},
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeC},
	}

	result := LinearizeNodes(startID, nodes, edges)

	if len(result) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(result))
	}

	// First should be start
	if result[0].ID != startID {
		t.Errorf("First node should be start")
	}

	// A and B should come before C (since C depends on A)
	cIndex := -1
	aIndex := -1
	for i, n := range result {
		if n.ID == nodeC {
			cIndex = i
		}
		if n.ID == nodeA {
			aIndex = i
		}
	}

	if aIndex > cIndex {
		t.Errorf("A should come before C in BFS order")
	}
}

func TestLinearizeNodes_DisconnectedNodes(t *testing.T) {
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	disconnected := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: disconnected, Name: "Disconnected", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeA},
	}

	result := LinearizeNodes(startID, nodes, edges)

	if len(result) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(result))
	}

	// Disconnected should be last
	if result[2].ID != disconnected {
		t.Errorf("Disconnected node should be last")
	}
}

func TestFindStartNode(t *testing.T) {
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
	}

	found, ok := FindStartNode(nodes)
	if !ok {
		t.Error("Should find start node")
	}
	if found.ID != startID {
		t.Error("Should return the start node")
	}
}

func TestFindStartNode_NotFound(t *testing.T) {
	nodes := []mflow.Node{
		{ID: idwrap.NewNow(), Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	_, ok := FindStartNode(nodes)
	if ok {
		t.Error("Should not find start node")
	}
}

func TestEdgeExists(t *testing.T) {
	nodeA := idwrap.NewNow()
	nodeB := idwrap.NewNow()
	nodeC := idwrap.NewNow()
	flowID := idwrap.NewNow()

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nodeA, TargetID: nodeB},
	}

	if !EdgeExists(edges, nodeA, nodeB) {
		t.Error("Edge A->B should exist")
	}

	if EdgeExists(edges, nodeA, nodeC) {
		t.Error("Edge A->C should not exist")
	}
}

func TestConnectOrphans(t *testing.T) {
	startID := idwrap.NewNow()
	nodeA := idwrap.NewNow()
	orphan := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nodes := []mflow.Node{
		{ID: startID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
		{ID: nodeA, Name: "A", NodeKind: mflow.NODE_KIND_REQUEST},
		{ID: orphan, Name: "Orphan", NodeKind: mflow.NODE_KIND_REQUEST},
	}

	edges := []mflow.Edge{
		{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startID, TargetID: nodeA},
	}

	result := ConnectOrphans(nodes, edges, flowID, startID)

	// Should have 2 edges now: Start->A and Start->Orphan
	if len(result) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(result))
	}

	// Verify Start->Orphan exists
	found := false
	for _, e := range result {
		if e.SourceID == startID && e.TargetID == orphan {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should create edge from start to orphan")
	}
}
