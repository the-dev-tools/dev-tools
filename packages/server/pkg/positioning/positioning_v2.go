package positioning

import (
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"time"
)

// PositionNodesV2 is an improved version that handles cycles better
func (np *NodePositioner) PositionNodesV2(nodes []mnnode.MNode, edges []edge.Edge, noopNodes []mnnoop.NoopNode) error {
	if len(nodes) == 0 {
		return nil
	}

	// Set a reasonable timeout
	done := make(chan error, 1)
	go func() {
		done <- np.positionNodesInternal(nodes, edges, noopNodes)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		// If positioning takes too long, just arrange in a grid
		return np.arrangeInGrid(nodes)
	}
}

func (np *NodePositioner) positionNodesInternal(nodes []mnnode.MNode, edges []edge.Edge, noopNodes []mnnoop.NoopNode) error {
	// Map for quick node lookup
	nodeMap := make(map[idwrap.IDWrap]*mnnode.MNode)
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	// Build adjacency lists
	outgoingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	incomingEdges := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		outgoingEdges[e.SourceID] = append(outgoingEdges[e.SourceID], e.TargetID)
		incomingEdges[e.TargetID] = append(incomingEdges[e.TargetID], e.SourceID)
	}

	// Find start nodes
	startNodes := findStartNodes(nodes, noopNodes, incomingEdges)
	if len(startNodes) == 0 {
		// No clear start, arrange in grid
		return np.arrangeInGrid(nodes)
	}

	// Detect cycles using DFS
	cycleNodes := detectCycles(nodes, outgoingEdges)
	
	// Calculate levels for non-cycle nodes
	nodeLevels := make(map[idwrap.IDWrap]int)
	levelNodes := make(map[int][]idwrap.IDWrap)
	
	// Initialize start nodes
	for _, startID := range startNodes {
		nodeLevels[startID] = 0
		levelNodes[0] = append(levelNodes[0], startID)
	}

	// Process nodes level by level, skipping cycle nodes
	visited := make(map[idwrap.IDWrap]bool)
	queue := append([]idwrap.IDWrap{}, startNodes...)
	
	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]
		
		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		currentLevel := nodeLevels[currentID]
		
		// Process children
		for _, childID := range outgoingEdges[currentID] {
			// Skip if in a cycle
			if cycleNodes[childID] {
				continue
			}
			
			// Calculate child level
			childLevel := currentLevel + 1
			
			// Update level if higher
			if existingLevel, exists := nodeLevels[childID]; !exists || childLevel > existingLevel {
				nodeLevels[childID] = childLevel
				
				// Update level grouping
				if exists {
					// Remove from old level
					removeFromLevel(levelNodes, existingLevel, childID)
				}
				levelNodes[childLevel] = append(levelNodes[childLevel], childID)
				
				if !visited[childID] {
					queue = append(queue, childID)
				}
			}
		}
	}

	// Position non-cycle nodes by levels
	np.positionByLevels(nodeMap, levelNodes)

	// Position cycle nodes separately
	np.positionCycleNodes(nodeMap, cycleNodes, nodeLevels, levelNodes)

	return nil
}

func findStartNodes(nodes []mnnode.MNode, noopNodes []mnnoop.NoopNode, incomingEdges map[idwrap.IDWrap][]idwrap.IDWrap) []idwrap.IDWrap {
	startNodes := []idwrap.IDWrap{}
	
	// Look for explicit start nodes
	for _, noop := range noopNodes {
		if noop.Type == mnnoop.NODE_NO_OP_KIND_START {
			startNodes = append(startNodes, noop.FlowNodeID)
		}
	}
	
	if len(startNodes) > 0 {
		return startNodes
	}
	
	// Find nodes with no incoming edges
	for _, node := range nodes {
		if len(incomingEdges[node.ID]) == 0 {
			startNodes = append(startNodes, node.ID)
		}
	}
	
	return startNodes
}

func detectCycles(nodes []mnnode.MNode, outgoingEdges map[idwrap.IDWrap][]idwrap.IDWrap) map[idwrap.IDWrap]bool {
	cycleNodes := make(map[idwrap.IDWrap]bool)
	visited := make(map[idwrap.IDWrap]bool)
	recStack := make(map[idwrap.IDWrap]bool)
	
	var dfs func(nodeID idwrap.IDWrap) bool
	dfs = func(nodeID idwrap.IDWrap) bool {
		visited[nodeID] = true
		recStack[nodeID] = true
		
		for _, childID := range outgoingEdges[nodeID] {
			if !visited[childID] {
				if dfs(childID) {
					cycleNodes[nodeID] = true
					return true
				}
			} else if recStack[childID] {
				// Found a cycle
				cycleNodes[nodeID] = true
				cycleNodes[childID] = true
				return true
			}
		}
		
		recStack[nodeID] = false
		return false
	}
	
	// Run DFS from all unvisited nodes
	for _, node := range nodes {
		if !visited[node.ID] {
			dfs(node.ID)
		}
	}
	
	return cycleNodes
}

func (np *NodePositioner) positionCycleNodes(nodeMap map[idwrap.IDWrap]*mnnode.MNode, cycleNodes map[idwrap.IDWrap]bool, nodeLevels map[idwrap.IDWrap]int, levelNodes map[int][]idwrap.IDWrap) {
	// Find the maximum level
	maxLevel := 0
	for level := range levelNodes {
		if level > maxLevel {
			maxLevel = level
		}
	}
	
	// Position cycle nodes at the bottom in a horizontal line
	cycleNodesList := []idwrap.IDWrap{}
	for nodeID, inCycle := range cycleNodes {
		if inCycle {
			cycleNodesList = append(cycleNodesList, nodeID)
		}
	}
	
	if len(cycleNodesList) == 0 {
		return
	}
	
	// Position at bottom level
	yPos := float64(np.StartY + (maxLevel+2)*np.NodeSpacingY)
	totalWidth := float64((len(cycleNodesList) - 1) * np.NodeSpacingX)
	startX := float64(np.StartX) - totalWidth/2
	
	for i, nodeID := range cycleNodesList {
		if node := nodeMap[nodeID]; node != nil {
			node.PositionX = startX + float64(i*np.NodeSpacingX)
			node.PositionY = yPos
		}
	}
}

func removeFromLevel(levelNodes map[int][]idwrap.IDWrap, level int, nodeID idwrap.IDWrap) {
	nodes := levelNodes[level]
	for i, id := range nodes {
		if id == nodeID {
			levelNodes[level] = append(nodes[:i], nodes[i+1:]...)
			break
		}
	}
}

// Simplified grid arrangement for fallback
func (np *NodePositioner) arrangeInGridSimple(nodes []mnnode.MNode) {
	cols := 4
	for i := range nodes {
		col := i % cols
		row := i / cols
		nodes[i].PositionX = float64(np.StartX + col*np.NodeSpacingX)
		nodes[i].PositionY = float64(np.StartY + row*np.NodeSpacingY)
	}
}