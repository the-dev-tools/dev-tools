package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertToDBEdge(e mflow.Edge) gen.FlowEdge {
	return gen.FlowEdge{
		ID:           e.ID,
		FlowID:       e.FlowID,
		SourceID:     e.SourceID,
		TargetID:     e.TargetID,
		SourceHandle: e.SourceHandler,
		State:        int8(e.State),
	}
}

func ConvertToModelEdge(e gen.FlowEdge) *mflow.Edge {
	return &mflow.Edge{
		ID:            e.ID,
		FlowID:        e.FlowID,
		SourceID:      e.SourceID,
		TargetID:      e.TargetID,
		SourceHandler: e.SourceHandle,
		State:         mflow.NodeState(e.State),
	}
}
