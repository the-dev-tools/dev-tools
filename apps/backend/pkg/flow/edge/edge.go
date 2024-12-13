package edge

import (
	"errors"
	"the-dev-tools/backend/pkg/idwrap"
)

type EdgeHandle int

/*
  HANDLE_UNSPECIFIED: 0,
  HANDLE_TRUE: 1,
  HANDLE_FALSE: 2,
  HANDLE_LOOP: 3,
*/

const (
	HandleUnspecified EdgeHandle = iota
	HandleTrue
	HandleFalse
	HandleLoop
	HandleLength
)

var ErrEdgeNotFound = errors.New("edge not found")

type Edge struct {
	ID            idwrap.IDWrap
	FlowID        idwrap.IDWrap
	SourceID      idwrap.IDWrap
	TargetID      idwrap.IDWrap
	SourceHandler EdgeHandle
}

type (
	EdgesMap map[idwrap.IDWrap]map[EdgeHandle]idwrap.IDWrap
)

func GetNextNodeID(edgesMap EdgesMap, sourceID idwrap.IDWrap, handle EdgeHandle) *idwrap.IDWrap {
	edges, ok := edgesMap[sourceID]
	if !ok {
		return nil
	}
	edge, ok := edges[handle]
	if !ok {
		return nil
	}

	return &edge
}

func NewEdge(id, sourceID, targetID idwrap.IDWrap, sourceHandlerID EdgeHandle) Edge {
	return Edge{
		ID:            id,
		SourceID:      sourceID,
		TargetID:      targetID,
		SourceHandler: sourceHandlerID,
	}
}

func NewEdges(edges ...Edge) []Edge {
	return edges
}

func NewEdgesMap(edges []Edge) EdgesMap {
	edgesMap := make(EdgesMap)
	for _, edge := range edges {
		if _, ok := edgesMap[edge.SourceID]; !ok {
			edgesMap[edge.SourceID] = make(map[EdgeHandle]idwrap.IDWrap)
		}
		edgesMap[edge.SourceID][edge.SourceHandler] = edge.TargetID
	}
	return edgesMap
}
