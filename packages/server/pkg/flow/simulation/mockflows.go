//nolint:revive // exported
package simulation

import (
	"time"

	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/mocknode"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

// MockFlowParams defines the parameters for creating mock flows
type MockFlowParams struct {
	RequestCount int           // Number of request nodes to create
	ForLoopCount int           // Number of for loop nodes to create
	Delay        time.Duration // Execution delay per node
}

// MockFlowResult contains the three data structures FlowLocalRunner needs
type MockFlowResult struct {
	Nodes       map[idwrap.IDWrap]node.FlowNode
	Edges       []mflow.Edge
	EdgesMap    mflow.EdgesMap
	StartNodeID idwrap.IDWrap
}

// CreateMockFlow creates a simple linear mock flow for testing
// Flow pattern: start → request1 → request2 → ... → forLoop1 → forLoop2 → ...
func CreateMockFlow(params MockFlowParams) MockFlowResult {
	// Calculate total nodes: 1 start + request nodes + for loop nodes
	totalNodes := 1 + params.RequestCount + params.ForLoopCount
	nodes := make(map[idwrap.IDWrap]node.FlowNode, totalNodes)
	edges := make([]mflow.Edge, 0, totalNodes-1) // n-1 edges for linear flow

	// Generate all node IDs first
	nodeIDs := make([]idwrap.IDWrap, totalNodes)
	for i := range nodeIDs {
		nodeIDs[i] = idwrap.NewNow()
	}

	// Create start node (index 0)
	startNodeID := nodeIDs[0]
	var startNextIDs []idwrap.IDWrap
	if totalNodes > 1 {
		startNextIDs = []idwrap.IDWrap{nodeIDs[1]} // Point to first non-start node
	}
	startNode := mocknode.NewDelayedMockNode(startNodeID, startNextIDs, 0)
	nodes[startNodeID] = startNode

	// Create request nodes
	currentIndex := 1
	for i := range params.RequestCount {
		nodeID := nodeIDs[currentIndex]

		// Determine next node ID (empty for last node)
		var nextIDs []idwrap.IDWrap
		if currentIndex < len(nodeIDs)-1 {
			nextIDs = []idwrap.IDWrap{nodeIDs[currentIndex+1]}
		}

		requestNode := mocknode.NewDelayedMockNode(nodeID, nextIDs, params.Delay)
		nodes[nodeID] = requestNode

		// Create edge from previous node to this request node
		if i == 0 {
			// Edge from start to first request node
			edgeID := idwrap.NewNow()
			edges = append(edges, mflow.NewEdge(edgeID, startNodeID, nodeID, mflow.HandleThen))
		} else {
			// Edge from previous request node to this one
			edgeID := idwrap.NewNow()
			edges = append(edges, mflow.NewEdge(edgeID, nodeIDs[currentIndex-1], nodeID, mflow.HandleThen))
		}

		currentIndex++
	}

	// Create for loop nodes
	for range params.ForLoopCount {
		nodeID := nodeIDs[currentIndex]

		// Determine next node ID (empty for last node)
		var nextIDs []idwrap.IDWrap
		if currentIndex < len(nodeIDs)-1 {
			nextIDs = []idwrap.IDWrap{nodeIDs[currentIndex+1]}
		}

		forLoopNode := mocknode.NewDelayedMockNode(nodeID, nextIDs, params.Delay)
		nodes[nodeID] = forLoopNode

		// Create edge from previous node to this for loop node
		var sourceID idwrap.IDWrap
		if currentIndex == 1 && params.RequestCount == 0 {
			// Edge from start to first for loop node (no request nodes)
			sourceID = startNodeID
		} else {
			// Edge from previous node
			sourceID = nodeIDs[currentIndex-1]
		}

		edgeID := idwrap.NewNow()
		edges = append(edges, mflow.NewEdge(edgeID, sourceID, nodeID, mflow.HandleThen))

		currentIndex++
	}

	// Create edges map from edges
	edgesMap := mflow.NewEdgesMap(edges)

	return MockFlowResult{
		Nodes:       nodes,
		Edges:       edges,
		EdgesMap:    edgesMap,
		StartNodeID: startNodeID,
	}
}
