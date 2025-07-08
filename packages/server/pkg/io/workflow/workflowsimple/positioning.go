package workflowsimple

import (
	"errors"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
)

// positionNodes arranges nodes using a level-based layout algorithm
func positionNodes(data *WorkflowData) error {
	const (
		nodeSpacingX = 400 // Horizontal spacing between parallel nodes
		nodeSpacingY = 300 // Vertical spacing between levels
		startX       = 0   // Starting X position
		startY       = 0   // Starting Y position
	)

	// Map for quick node lookup
	nodeMap := make(map[idwrap.IDWrap]*mnnode.MNode)
	for i := range data.Nodes {
		nodeMap[data.Nodes[i].ID] = &data.Nodes[i]
	}

	// Find start node
	var startNode *mnnode.MNode
	for i := range data.NoopNodes {
		if data.NoopNodes[i].Type == mnnoop.NODE_NO_OP_KIND_START {
			startNode = nodeMap[data.NoopNodes[i].FlowNodeID]
			break
		}
	}
	if startNode == nil {
		return errors.New("start node not found")
	}

	// Build adjacency lists from edges
	outgoingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	incomingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range data.Edges {
		outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e.TargetID)
		incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e.SourceID)
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

	// Position nodes level by level
	for level := 0; level <= len(levelNodes)-1; level++ {
		nodes := levelNodes[level]
		if len(nodes) == 0 {
			continue
		}

		// Calculate Y position for this level
		yPos := float64(startY + level*nodeSpacingY)

		// Calculate starting X position to center the nodes at this level
		totalWidth := float64((len(nodes) - 1) * nodeSpacingX)
		startXForLevel := float64(startX) - totalWidth/2

		// Position each node in this level
		for i, nodeID := range nodes {
			if node := nodeMap[nodeID]; node != nil {
				node.PositionX = startXForLevel + float64(i*nodeSpacingX)
				node.PositionY = yPos
			}
		}
	}

	return nil
}
