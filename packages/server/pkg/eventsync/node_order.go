package eventsync

import (
	"sort"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

// NodePriority defines the base priority for each node kind.
// Lower values = higher priority (published first).
// Container nodes have lower priority so they exist before children.
const (
	PriorityManualStart = 0   // Entry point - always first
	PriorityFor         = 100 // Loop container
	PriorityForEach     = 100 // Loop container
	PriorityCondition   = 100 // Branch container
	PriorityRequest     = 200 // Leaf node
	PriorityJS          = 200 // Leaf node
	PriorityUnspecified = 999 // Unknown - last
)

// NodeKindPriority maps node kinds to their base priority.
var NodeKindPriority = map[mflow.NodeKind]int{
	mflow.NODE_KIND_MANUAL_START: PriorityManualStart,
	mflow.NODE_KIND_FOR:          PriorityFor,
	mflow.NODE_KIND_FOR_EACH:     PriorityForEach,
	mflow.NODE_KIND_CONDITION:    PriorityCondition,
	mflow.NODE_KIND_REQUEST:      PriorityRequest,
	mflow.NODE_KIND_JS:           PriorityJS,
	mflow.NODE_KIND_UNSPECIFIED:  PriorityUnspecified,
}

// GetNodeKindPriority returns the base priority for a node kind.
func GetNodeKindPriority(kind mflow.NodeKind) int {
	if p, ok := NodeKindPriority[kind]; ok {
		return p
	}
	return PriorityUnspecified
}

// NodeOrderInfo contains ordering information for a node.
type NodeOrderInfo struct {
	NodeID   idwrap.IDWrap
	Level    int // Graph depth (0 = start node)
	Priority int // Base priority from node kind
}

// ComputeNodeOrder computes the publishing order for nodes based on:
// 1. Graph topology (BFS level from start node)
// 2. Node kind priority (containers before leaves)
//
// Returns node IDs in the order they should be published.
func ComputeNodeOrder(nodes []mflow.Node, edges []mflow.Edge) []idwrap.IDWrap {
	if len(nodes) == 0 {
		return nil
	}

	// Build node map and adjacency
	nodeMap := make(map[idwrap.IDWrap]*mflow.Node)
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	outgoing := buildOutgoing(edges)

	// Find start node
	var startID idwrap.IDWrap
	for _, node := range nodes {
		if node.NodeKind == mflow.NODE_KIND_MANUAL_START {
			startID = node.ID
			break
		}
	}

	// Compute BFS levels from start node
	levels := make(map[idwrap.IDWrap]int)
	var emptyID idwrap.IDWrap
	if startID.Compare(emptyID) != 0 {
		computeLevels(startID, outgoing, levels)
	}

	// Assign levels to orphan nodes (not reachable from start)
	maxLevel := 0
	for _, l := range levels {
		if l > maxLevel {
			maxLevel = l
		}
	}
	for _, node := range nodes {
		if _, hasLevel := levels[node.ID]; !hasLevel {
			// Orphan nodes go after all reachable nodes
			levels[node.ID] = maxLevel + 1
		}
	}

	// Build order info for each node
	orderInfos := make([]NodeOrderInfo, len(nodes))
	for i, node := range nodes {
		orderInfos[i] = NodeOrderInfo{
			NodeID:   node.ID,
			Level:    levels[node.ID],
			Priority: GetNodeKindPriority(node.NodeKind),
		}
	}

	// Sort by: level first, then priority, then ID for determinism
	sort.Slice(orderInfos, func(i, j int) bool {
		if orderInfos[i].Level != orderInfos[j].Level {
			return orderInfos[i].Level < orderInfos[j].Level
		}
		if orderInfos[i].Priority != orderInfos[j].Priority {
			return orderInfos[i].Priority < orderInfos[j].Priority
		}
		return orderInfos[i].NodeID.Compare(orderInfos[j].NodeID) < 0
	})

	// Extract ordered IDs
	result := make([]idwrap.IDWrap, len(orderInfos))
	for i, info := range orderInfos {
		result[i] = info.NodeID
	}

	return result
}

// buildOutgoing builds adjacency list for outgoing edges.
func buildOutgoing(edges []mflow.Edge) map[idwrap.IDWrap][]idwrap.IDWrap {
	adj := make(map[idwrap.IDWrap][]idwrap.IDWrap)
	for _, e := range edges {
		adj[e.SourceID] = append(adj[e.SourceID], e.TargetID)
	}
	return adj
}

// computeLevels uses BFS to assign levels to nodes.
func computeLevels(startID idwrap.IDWrap, outgoing map[idwrap.IDWrap][]idwrap.IDWrap, levels map[idwrap.IDWrap]int) {
	queue := []idwrap.IDWrap{startID}
	levels[startID] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentLevel := levels[current]

		for _, target := range outgoing[current] {
			if _, visited := levels[target]; !visited {
				levels[target] = currentLevel + 1
				queue = append(queue, target)
			}
		}
	}
}

// SortNodesByOrder sorts nodes in-place according to ComputeNodeOrder.
func SortNodesByOrder(nodes []mflow.Node, edges []mflow.Edge) {
	if len(nodes) == 0 {
		return
	}

	// Get the ordered IDs
	orderedIDs := ComputeNodeOrder(nodes, edges)

	// Create ID to position map
	idToPos := make(map[idwrap.IDWrap]int)
	for i, id := range orderedIDs {
		idToPos[id] = i
	}

	// Sort nodes by their position in orderedIDs
	sort.Slice(nodes, func(i, j int) bool {
		posI := idToPos[nodes[i].ID]
		posJ := idToPos[nodes[j].ID]
		return posI < posJ
	})
}
