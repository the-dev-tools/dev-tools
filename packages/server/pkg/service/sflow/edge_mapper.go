package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertToDBEdge(e mflow.Edge) gen.FlowEdge {
	return gen.FlowEdge{
		ID:           e.ID,
		FlowID:       e.FlowID,
		SourceID:     e.SourceID,
		TargetID:     e.TargetID,
		SourceHandle: e.SourceHandler,
		State:        e.State,
	}
}

func ConvertToModelEdge(e gen.FlowEdge) *mflow.Edge {
	return &mflow.Edge{
		ID:            e.ID,
		FlowID:        e.FlowID,
		SourceID:      e.SourceID,
		TargetID:      e.TargetID,
		SourceHandler: e.SourceHandle,
		State:         e.State,
	}
}
