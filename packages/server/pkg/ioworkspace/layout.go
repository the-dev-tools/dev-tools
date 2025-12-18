package ioworkspace

import (
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

// Layout constants for node positioning
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
	for i := range wb.FlowNoopNodes {
		if wb.FlowNoopNodes[i].Type == mflow.NODE_NO_OP_KIND_START {
			// Find the corresponding flow node
			for j := range wb.FlowNodes {
				if wb.FlowNodes[j].ID.Compare(wb.FlowNoopNodes[i].FlowNodeID) == 0 &&
					wb.FlowNodes[j].FlowID.Compare(flowID) == 0 {
					startNodeID = &wb.FlowNodes[j].ID
					break
				}
			}
		}
		if startNodeID != nil {
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
			NodeKind:  mflow.NODE_KIND_NO_OP,
			PositionX: StartX,
			PositionY: StartY,
		}
		wb.FlowNodes = append(wb.FlowNodes, startNode)

		noopNode := mflow.NodeNoop{
			FlowNodeID: newStartNodeID,
			Type:       mflow.NODE_NO_OP_KIND_START,
		}
		wb.FlowNoopNodes = append(wb.FlowNoopNodes, noopNode)
	}

	// Note: We intentionally do NOT auto-connect orphan nodes to start.
	// Disconnected nodes should remain disconnected and will not execute.
	// This allows users to have disabled/draft nodes in their flows.

	return nil
}

// layoutFlowNodes positions flow nodes using a level-based layout algorithm.
// Parallel nodes are positioned at the same Y level, sequential nodes at deeper levels.
func (wb *WorkspaceBundle) layoutFlowNodes(flowID idwrap.IDWrap) error {
	// Build node map for this flow
	nodeMap := make(map[idwrap.IDWrap]*mflow.Node)
	for i := range wb.FlowNodes {
		if wb.FlowNodes[i].FlowID.Compare(flowID) == 0 {
			nodeMap[wb.FlowNodes[i].ID] = &wb.FlowNodes[i]
		}
	}

	if len(nodeMap) == 0 {
		return nil // No nodes to layout
	}

	// Find start node for this flow
	var startNode *mflow.Node
	for i := range wb.FlowNoopNodes {
		if wb.FlowNoopNodes[i].Type == mflow.NODE_NO_OP_KIND_START {
			if node := nodeMap[wb.FlowNoopNodes[i].FlowNodeID]; node != nil {
				startNode = node
				break
			}
		}
	}

	if startNode == nil {
		return fmt.Errorf("start node not found for flow")
	}

	// Build adjacency lists from edges for this flow
	outgoingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	incomingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range wb.FlowEdges {
		if e.FlowID.Compare(flowID) == 0 {
			outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e.TargetID)
			incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e.SourceID)
		}
	}

	// Calculate dependency levels using BFS
	nodeLevels := make(map[idwrap.IDWrap]int)
	levelNodes := make(map[int][]idwrap.IDWrap) // level -> nodes at that level

	// BFS to assign levels
	queue := []idwrap.IDWrap{startNode.ID}
	nodeLevels[startNode.ID] = 0
	levelNodes[0] = []idwrap.IDWrap{startNode.ID}

	for len(queue) > 0 {
		currentNodeID := queue[0]
		queue = queue[1:]

		// Process all children
		for _, childID := range outgoingEdges[currentNodeID] {
			// Calculate the maximum level of all parents + 1
			maxParentLevel := -1
			for _, parentID := range incomingEdges[childID] {
				if parentLevel, exists := nodeLevels[parentID]; exists {
					if parentLevel > maxParentLevel {
						maxParentLevel = parentLevel
					}
				}
			}

			childLevel := maxParentLevel + 1

			// Only update if this is a new node or we found a deeper level
			if existingLevel, exists := nodeLevels[childID]; !exists || childLevel > existingLevel {
				// Remove from old level if it existed
				if exists {
					oldLevelNodes := levelNodes[existingLevel]
					for i, nodeID := range oldLevelNodes {
						if nodeID == childID {
							levelNodes[existingLevel] = append(oldLevelNodes[:i], oldLevelNodes[i+1:]...)
							break
						}
					}
				}

				// Add to new level
				nodeLevels[childID] = childLevel
				levelNodes[childLevel] = append(levelNodes[childLevel], childID)
				queue = append(queue, childID)
			}
		}
	}

	// Find max level
	maxLevel := 0
	for level := range levelNodes {
		if level > maxLevel {
			maxLevel = level
		}
	}

	// Position nodes level by level
	for level := 0; level <= maxLevel; level++ {
		nodes := levelNodes[level]
		if len(nodes) == 0 {
			continue
		}

		// Calculate Y position for this level
		yPos := float64(StartY + level*NodeSpacingY)

		// Calculate starting X position to center the nodes at this level
		totalWidth := float64((len(nodes) - 1) * NodeSpacingX)
		startXForLevel := float64(StartX) - totalWidth/2

		// Position each node in this level
		for i, nodeID := range nodes {
			if node := nodeMap[nodeID]; node != nil {
				node.PositionX = startXForLevel + float64(i*NodeSpacingX)
				node.PositionY = yPos
			}
		}
	}

	return nil
}
