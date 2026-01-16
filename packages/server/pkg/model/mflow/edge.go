//nolint:revive // exported
package mflow

import (
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

type EdgeHandle = int32

/*
  HANDLE_UNSPECIFIED: 0,
  HANDLE_THEN: 1,
  HANDLE_ELSE: 2,
  HANDLE_LOOP: 3,
*/

const (
	HandleUnspecified EdgeHandle = iota
	HandleThen
	HandleElse
	HandleLoop
	HandleAiProvider
	HandleAiMemory
	HandleAiTools
	HandleLength
)

var ErrEdgeNotFound = errors.New("edge not found")

type Edge struct {
	ID            idwrap.IDWrap
	FlowID        idwrap.IDWrap
	SourceID      idwrap.IDWrap
	TargetID      idwrap.IDWrap
	SourceHandler EdgeHandle
	State         NodeState
}

type (
	EdgesMap map[idwrap.IDWrap]map[EdgeHandle][]idwrap.IDWrap
)

func GetNextNodeID(edgesMap EdgesMap, sourceID idwrap.IDWrap, handle EdgeHandle) []idwrap.IDWrap {
	edges, ok := edgesMap[sourceID]
	if !ok {
		return nil
	}
	edge, ok := edges[handle]
	if !ok {
		return nil
	}

	return edge
}

func NewEdge(id, sourceID, targetID idwrap.IDWrap, sourceHandlerID EdgeHandle) Edge {
	return Edge{
		ID:            id,
		SourceID:      sourceID,
		TargetID:      targetID,
		SourceHandler: sourceHandlerID,
		State:         NODE_STATE_UNSPECIFIED,
	}
}
func NewEdges(edges ...Edge) []Edge {
	return edges
}

func NewEdgesMap(edges []Edge) EdgesMap {
	edgesMap := make(EdgesMap)
	for _, edge := range edges {
		if _, ok := edgesMap[edge.SourceID]; !ok {
			edgesMap[edge.SourceID] = make(map[EdgeHandle][]idwrap.IDWrap)
		}
		a := edgesMap[edge.SourceID][edge.SourceHandler]
		a = append(a, edge.TargetID)
		edgesMap[edge.SourceID][edge.SourceHandler] = a
	}
	return edgesMap
}

// NodePosition represents the relative position of nodes
type NodePosition int

const (
	NodeBefore NodePosition = iota
	NodeAfter
	NodeUnrelated
)

// IsNodeCheckTarget determines if sourceNode is before targetNode in the flow graph
func IsNodeCheckTarget(edgesMap EdgesMap, sourceNode, targetNode idwrap.IDWrap) NodePosition {
	if sourceNode == targetNode {
		return NodeUnrelated
	}

	visited := make(map[idwrap.IDWrap]bool)
	queue := []idwrap.IDWrap{sourceNode}

	// BFS to find if target node is reachable from source
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == targetNode {
			return NodeBefore
		}

		// Check all possible edges from current node
		for handle := HandleUnspecified; handle < HandleLength; handle++ {
			nextNodes := GetNextNodeID(edgesMap, current, handle)
			for _, next := range nextNodes {
				if !visited[next] {
					visited[next] = true
					queue = append(queue, next)
				}
			}
		}
	}

	// Check if source is reachable from target (reverse check)
	visited = make(map[idwrap.IDWrap]bool)
	queue = []idwrap.IDWrap{targetNode}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == sourceNode {
			return NodeAfter
		}

		for handle := HandleUnspecified; handle < HandleLength; handle++ {
			nextNodes := GetNextNodeID(edgesMap, current, handle)
			for _, next := range nextNodes {
				if !visited[next] {
					visited[next] = true
					queue = append(queue, next)
				}
			}
		}
	}

	return NodeUnrelated
}
