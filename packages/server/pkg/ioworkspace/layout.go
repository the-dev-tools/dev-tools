package ioworkspace

import (
	"fmt"

	"the-dev-tools/server/pkg/flowgraph"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

// Layout constants for node positioning (kept for backward compatibility)
const (
	NodeSpacingX = 400 // Horizontal spacing between parallel nodes
	NodeSpacingY = 300 // Vertical spacing between levels
	StartX       = 0   // Starting X position
	StartY       = 0   // Starting Y position
)

// EnsureFlowStructure ensures each flow in the bundle has a proper start node
// and positions all nodes using a level-based layout algorithm.
// This should be called after all flow data has been populated but before saving.
func (wb *WorkspaceBundle) EnsureFlowStructure() error {
	for _, flow := range wb.Flows {
		if err := wb.ensureStartNodeForFlow(flow.ID); err != nil {
			return fmt.Errorf("failed to ensure start node for flow %s: %w", flow.Name, err)
		}
		if err := wb.layoutFlowNodes(flow.ID); err != nil {
			return fmt.Errorf("failed to layout nodes for flow %s: %w", flow.Name, err)
		}
	}
	return nil
}

// ensureStartNodeForFlow checks if a flow has a start node and creates one if missing.
// Note: Orphan nodes (nodes with no incoming edges) are intentionally NOT connected to start.
// Disconnected nodes should remain disconnected and will not execute.
func (wb *WorkspaceBundle) ensureStartNodeForFlow(flowID idwrap.IDWrap) error {
	// Check if start node already exists for this flow
	var startNodeID *idwrap.IDWrap
	for j := range wb.FlowNodes {
		if wb.FlowNodes[j].NodeKind == mflow.NODE_KIND_MANUAL_START &&
			wb.FlowNodes[j].FlowID.Compare(flowID) == 0 {
			startNodeID = &wb.FlowNodes[j].ID
			break
		}
	}

	// If no start node exists, create one
	if startNodeID == nil {
		newStartNodeID := idwrap.NewNow()
		startNode := mflow.Node{
			ID:        newStartNodeID,
			FlowID:    flowID,
			Name:      "Start",
			NodeKind:  mflow.NODE_KIND_MANUAL_START,
			PositionX: StartX,
			PositionY: StartY,
		}
		wb.FlowNodes = append(wb.FlowNodes, startNode)
	}

	// Note: We intentionally do NOT auto-connect orphan nodes to start.
	// Disconnected nodes should remain disconnected and will not execute.
	// This allows users to have disabled/draft nodes in their flows.

	return nil
}

// layoutFlowNodes positions flow nodes using a level-based layout algorithm.
// Parallel nodes are positioned at the same Y level, sequential nodes at deeper levels.
// Uses the shared flowgraph package for layout calculation.
func (wb *WorkspaceBundle) layoutFlowNodes(flowID idwrap.IDWrap) error {
	// Collect nodes and edges for this flow
	var flowNodes []mflow.Node
	nodeIndexMap := make(map[idwrap.IDWrap]int) // Maps node ID to index in wb.FlowNodes

	for i := range wb.FlowNodes {
		if wb.FlowNodes[i].FlowID.Compare(flowID) == 0 {
			flowNodes = append(flowNodes, wb.FlowNodes[i])
			nodeIndexMap[wb.FlowNodes[i].ID] = i
		}
	}

	if len(flowNodes) == 0 {
		return nil // No nodes to layout
	}

	// Collect edges for this flow
	var flowEdges []mflow.Edge
	for _, e := range wb.FlowEdges {
		if e.FlowID.Compare(flowID) == 0 {
			flowEdges = append(flowEdges, e)
		}
	}

	// Find start node
	startNode, found := flowgraph.FindStartNode(flowNodes)
	if !found {
		return fmt.Errorf("start node not found for flow")
	}

	// Use horizontal layout (X increases with depth, Y for parallel nodes)
	// This matches HAR import: nodes flow left-to-right with 300px spacing
	config := flowgraph.DefaultHorizontalConfig()

	layoutResult, err := flowgraph.Layout(flowNodes, flowEdges, startNode.ID, config)
	if err != nil {
		return err
	}

	// Apply positions back to the original nodes in wb.FlowNodes
	for nodeID, pos := range layoutResult.Positions {
		if idx, ok := nodeIndexMap[nodeID]; ok {
			wb.FlowNodes[idx].PositionX = pos.X
			wb.FlowNodes[idx].PositionY = pos.Y
		}
	}

	return nil
}
