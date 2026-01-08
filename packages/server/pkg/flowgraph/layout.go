package flowgraph

import (
	"fmt"
	"sort"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// Layout computes node positions using BFS-based level assignment.
// Each node's level is max(parent_levels) + 1, ensuring proper dependency ordering.
func Layout(nodes []mflow.Node, edges []mflow.Edge, startNodeID idwrap.IDWrap, config LayoutConfig) (*LayoutResult, error) {
	if len(nodes) == 0 {
		return &LayoutResult{
			Positions: make(map[idwrap.IDWrap]Position),
			Levels:    make(map[idwrap.IDWrap]int),
			MaxLevel:  0,
		}, nil
	}

	// Build adjacency lists
	outgoingEdges := BuildOutgoingAdjacency(edges)
	incomingEdges := BuildIncomingAdjacency(edges)

	// Calculate dependency levels using BFS
	nodeLevels := make(map[idwrap.IDWrap]int)
	levelNodes := make(map[int][]idwrap.IDWrap)

	// Start BFS from start node
	queue := []idwrap.IDWrap{startNodeID}
	nodeLevels[startNodeID] = 0
	levelNodes[0] = []idwrap.IDWrap{startNodeID}

	// Safety counter to prevent infinite loops on cyclic graphs
	processedCount := 0
	maxProcessed := len(nodes) * len(nodes)
	if maxProcessed < 10000 {
		maxProcessed = 10000
	}

	for len(queue) > 0 {
		if processedCount > maxProcessed {
			break
		}
		processedCount++

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

	// Calculate positions based on orientation
	positions := make(map[idwrap.IDWrap]Position)

	for level := 0; level <= maxLevel; level++ {
		nodesAtLevel := levelNodes[level]
		if len(nodesAtLevel) == 0 {
			continue
		}

		// Calculate primary axis position (depth direction)
		primaryPos := config.StartX
		if config.Orientation == LayoutVertical {
			primaryPos = config.StartY
		}
		primaryPos += float64(level) * config.SpacingPrimary

		// Calculate secondary axis positions (centered around start)
		totalSecondary := float64((len(nodesAtLevel) - 1)) * config.SpacingSecondary
		startSecondary := config.StartY
		if config.Orientation == LayoutVertical {
			startSecondary = config.StartX
		}
		startSecondary -= totalSecondary / 2

		for i, nodeID := range nodesAtLevel {
			secondaryPos := startSecondary + float64(i)*config.SpacingSecondary

			var pos Position
			if config.Orientation == LayoutHorizontal {
				pos = Position{X: primaryPos, Y: secondaryPos}
			} else {
				pos = Position{X: secondaryPos, Y: primaryPos}
			}
			positions[nodeID] = pos
		}
	}

	return &LayoutResult{
		Positions: positions,
		Levels:    nodeLevels,
		MaxLevel:  maxLevel,
	}, nil
}

// ApplyLayout applies the layout result to a slice of nodes.
func ApplyLayout(nodes []mflow.Node, result *LayoutResult) {
	for i := range nodes {
		if pos, ok := result.Positions[nodes[i].ID]; ok {
			nodes[i].PositionX = pos.X
			nodes[i].PositionY = pos.Y
		}
	}
}

// ApplyLayoutToNodePtrs applies the layout result to a map of node pointers.
func ApplyLayoutToNodePtrs(nodeMap map[idwrap.IDWrap]*mflow.Node, result *LayoutResult) {
	for nodeID, pos := range result.Positions {
		if node, ok := nodeMap[nodeID]; ok {
			node.PositionX = pos.X
			node.PositionY = pos.Y
		}
	}
}

// LinearizeNodes returns nodes in BFS traversal order (for YAML export).
// Neighbors are sorted alphabetically by name for deterministic ordering.
// Disconnected nodes are appended at the end, also sorted alphabetically.
func LinearizeNodes(startNodeID idwrap.IDWrap, allNodes []mflow.Node, edges []mflow.Edge) []mflow.Node {
	if len(allNodes) == 0 {
		return nil
	}

	// Build node map for quick lookup
	nodeMap := make(map[idwrap.IDWrap]mflow.Node)
	for _, n := range allNodes {
		nodeMap[n.ID] = n
	}

	// Build edges by source
	edgesBySource := BuildOutgoingAdjacency(edges)

	// BFS traversal
	visited := make(map[idwrap.IDWrap]bool)
	var result []mflow.Node
	queue := []idwrap.IDWrap{startNodeID}
	visited[startNodeID] = true

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if n, ok := nodeMap[currentID]; ok {
			result = append(result, n)
		}

		// Get all outgoing edges from current node
		targetIDs := edgesBySource[currentID]
		var neighbors []mflow.Node

		for _, targetID := range targetIDs {
			if target, ok := nodeMap[targetID]; ok {
				neighbors = append(neighbors, target)
			}
		}

		// Sort neighbors alphabetically by name for deterministic ordering
		sort.Slice(neighbors, func(i, j int) bool {
			return neighbors[i].Name < neighbors[j].Name
		})

		// Add unvisited neighbors to queue
		for _, neighbor := range neighbors {
			if !visited[neighbor.ID] {
				visited[neighbor.ID] = true
				queue = append(queue, neighbor.ID)
			}
		}
	}

	// Handle disconnected nodes (not reachable from start)
	var disconnected []mflow.Node
	for _, n := range allNodes {
		if !visited[n.ID] {
			disconnected = append(disconnected, n)
		}
	}

	// Sort disconnected nodes alphabetically
	sort.Slice(disconnected, func(i, j int) bool {
		return disconnected[i].Name < disconnected[j].Name
	})

	// Append disconnected nodes to result
	result = append(result, disconnected...)

	return result
}

// LayoutNodes is a convenience function that performs layout and applies positions.
// It returns an error if the start node is not found.
func LayoutNodes(nodes []mflow.Node, edges []mflow.Edge, config LayoutConfig) error {
	startNode, found := FindStartNode(nodes)
	if !found {
		return fmt.Errorf("start node not found")
	}

	result, err := Layout(nodes, edges, startNode.ID, config)
	if err != nil {
		return err
	}

	ApplyLayout(nodes, result)
	return nil
}
