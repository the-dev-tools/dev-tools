package positioning

import (
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
)

const (
	// Default spacing between nodes
	DefaultNodeSpacingX = 400 // Horizontal spacing between parallel nodes
	DefaultNodeSpacingY = 300 // Vertical spacing between levels
	DefaultStartX       = 0   // Starting X position
	DefaultStartY       = 0   // Starting Y position
)

// NodePositioner provides methods for positioning flow nodes
type NodePositioner struct {
	NodeSpacingX int
	NodeSpacingY int
	StartX       int
	StartY       int
}

// NewNodePositioner creates a new node positioner with default settings
func NewNodePositioner() *NodePositioner {
	return &NodePositioner{
		NodeSpacingX: DefaultNodeSpacingX,
		NodeSpacingY: DefaultNodeSpacingY,
		StartX:       DefaultStartX,
		StartY:       DefaultStartY,
	}
}

// PositionNodes positions flow nodes using a level-based layout.
// Parallel nodes are positioned at the same Y level, sequential nodes at deeper levels.
func (np *NodePositioner) PositionNodes(nodes []mnnode.MNode, edges []edge.Edge, noopNodes []mnnoop.NoopNode) error {
	// Use the improved algorithm that handles cycles better
	return np.PositionNodesV2(nodes, edges, noopNodes)
}

// PositionNodesOld is the original positioning algorithm (kept for reference)
func (np *NodePositioner) PositionNodesOld(nodes []mnnode.MNode, edges []edge.Edge, noopNodes []mnnoop.NoopNode) error {
	// Map for quick node lookup
	nodeMap := make(map[idwrap.IDWrap]*mnnode.MNode)
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	// Find start node
	var startNode *mnnode.MNode
	for i := range noopNodes {
		if noopNodes[i].Type == mnnoop.NODE_NO_OP_KIND_START {
			startNode = nodeMap[noopNodes[i].FlowNodeID]
			break
		}
	}
	if startNode == nil {
		// If no explicit start node, position all nodes without dependencies at level 0
		return np.positionWithoutStartNode(nodes, edges)
	}

	// Build adjacency lists from edges
	outgoingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	incomingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e.TargetID)
		incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e.SourceID)
	}

	// Calculate dependency levels using BFS
	nodeLevels := make(map[idwrap.IDWrap]int)
	levelNodes := make(map[int][]idwrap.IDWrap) // level -> nodes at that level
	visited := make(map[idwrap.IDWrap]bool)
	inQueue := make(map[idwrap.IDWrap]bool)

	// BFS to assign levels
	queue := []idwrap.IDWrap{startNode.ID}
	nodeLevels[startNode.ID] = 0
	levelNodes[0] = []idwrap.IDWrap{startNode.ID}
	inQueue[startNode.ID] = true

	maxIterations := len(nodes) * len(edges) // Safety limit to prevent infinite loops
	iterations := 0

	for len(queue) > 0 && iterations < maxIterations {
		iterations++
		currentNodeID := queue[0]
		queue = queue[1:]
		delete(inQueue, currentNodeID)
		visited[currentNodeID] = true

		// Process all children
		for _, childID := range outgoingEdges[currentNodeID] {
			// Skip if already processing to avoid cycles
			if inQueue[childID] {
				continue
			}

			// Calculate the maximum level of all parents + 1
			maxParentLevel := -1
			allParentsVisited := true
			for _, parentID := range incomingEdges[childID] {
				if !visited[parentID] {
					allParentsVisited = false
					break
				}
				if parentLevel, exists := nodeLevels[parentID]; exists {
					if parentLevel > maxParentLevel {
						maxParentLevel = parentLevel
					}
				}
			}

			// Only process if all parents have been visited (to handle cycles)
			if !allParentsVisited && !visited[childID] {
				continue
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
				if !visited[childID] {
					queue = append(queue, childID)
					inQueue[childID] = true
				}
			}
		}
	}

	// Position nodes level by level
	np.positionByLevels(nodeMap, levelNodes)

	return nil
}

// positionWithoutStartNode handles positioning when there's no explicit start node
func (np *NodePositioner) positionWithoutStartNode(nodes []mnnode.MNode, edges []edge.Edge) error {
	// Build adjacency lists from edges
	incomingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	outgoingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e.SourceID)
		outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e.TargetID)
	}

	// Find all nodes without incoming edges (root nodes)
	rootNodes := []idwrap.IDWrap{}
	for i := range nodes {
		if len(incomingEdges[nodes[i].ID]) == 0 {
			rootNodes = append(rootNodes, nodes[i].ID)
		}
	}

	// If no root nodes found, just arrange all nodes in a grid
	if len(rootNodes) == 0 {
		return np.arrangeInGrid(nodes)
	}

	// Map for quick node lookup
	nodeMap := make(map[idwrap.IDWrap]*mnnode.MNode)
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	// Calculate levels from root nodes
	nodeLevels := make(map[idwrap.IDWrap]int)
	levelNodes := make(map[int][]idwrap.IDWrap)

	// Initialize root nodes at level 0
	for _, rootID := range rootNodes {
		nodeLevels[rootID] = 0
		levelNodes[0] = append(levelNodes[0], rootID)
	}

	// BFS from all root nodes
	queue := append([]idwrap.IDWrap{}, rootNodes...)
	visited := make(map[idwrap.IDWrap]bool)
	inQueue := make(map[idwrap.IDWrap]bool)
	for _, rootID := range rootNodes {
		inQueue[rootID] = true
	}

	maxIterations := len(nodes) * len(edges) // Safety limit
	iterations := 0

	for len(queue) > 0 && iterations < maxIterations {
		iterations++
		currentNodeID := queue[0]
		queue = queue[1:]
		delete(inQueue, currentNodeID)
		visited[currentNodeID] = true

		// Process all children
		for _, childID := range outgoingEdges[currentNodeID] {
			// Skip if already processing to avoid cycles
			if inQueue[childID] {
				continue
			}

			// Calculate the maximum level of all parents + 1
			maxParentLevel := -1
			allParentsProcessed := true
			for _, parentID := range incomingEdges[childID] {
				if parentLevel, exists := nodeLevels[parentID]; exists {
					if parentLevel > maxParentLevel {
						maxParentLevel = parentLevel
					}
				} else {
					allParentsProcessed = false
					break
				}
			}

			// Only process if all parents have been processed
			if !allParentsProcessed {
				continue
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
				if !visited[childID] {
					queue = append(queue, childID)
					inQueue[childID] = true
				}
			}
		}
	}

	// Position nodes by levels
	np.positionByLevels(nodeMap, levelNodes)

	// Handle any unpositioned nodes (disconnected components)
	for i := range nodes {
		if _, exists := nodeLevels[nodes[i].ID]; !exists {
			// Place disconnected nodes at the bottom
			maxLevel := 0
			for level := range levelNodes {
				if level > maxLevel {
					maxLevel = level
				}
			}
			nodes[i].PositionX = float64(np.StartX)
			nodes[i].PositionY = float64(np.StartY + (maxLevel+1)*np.NodeSpacingY)
		}
	}

	return nil
}

// positionByLevels positions nodes according to their calculated levels
func (np *NodePositioner) positionByLevels(nodeMap map[idwrap.IDWrap]*mnnode.MNode, levelNodes map[int][]idwrap.IDWrap) {
	for level := 0; level <= len(levelNodes)-1; level++ {
		nodes := levelNodes[level]
		if len(nodes) == 0 {
			continue
		}

		// Calculate Y position for this level
		yPos := float64(np.StartY + level*np.NodeSpacingY)

		// Calculate starting X position to center the nodes at this level
		totalWidth := float64((len(nodes) - 1) * np.NodeSpacingX)
		startXForLevel := float64(np.StartX) - totalWidth/2

		// Position each node in this level
		for i, nodeID := range nodes {
			if node := nodeMap[nodeID]; node != nil {
				node.PositionX = startXForLevel + float64(i*np.NodeSpacingX)
				node.PositionY = yPos
			}
		}
	}
}

// arrangeInGrid arranges nodes in a grid pattern when no dependency information is available
func (np *NodePositioner) arrangeInGrid(nodes []mnnode.MNode) error {
	if len(nodes) == 0 {
		return nil
	}

	// Calculate grid dimensions
	cols := 3 // Default number of columns
	if len(nodes) < cols {
		cols = len(nodes)
	}

	// Arrange nodes in grid
	for i := range nodes {
		col := i % cols
		row := i / cols
		nodes[i].PositionX = float64(np.StartX + col*np.NodeSpacingX)
		nodes[i].PositionY = float64(np.StartY + row*np.NodeSpacingY)
	}

	return nil
}