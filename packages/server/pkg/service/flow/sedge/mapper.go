package sedge

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/flow/edge"
)

func ConvertToDBEdge(e edge.Edge) gen.FlowEdge {
	return gen.FlowEdge{
		ID:           e.ID,
		FlowID:       e.FlowID,
		SourceID:     e.SourceID,
		TargetID:     e.TargetID,
		SourceHandle: e.SourceHandler,
		EdgeKind:     e.Kind,
	}
}

func ConvertToModelEdge(e gen.FlowEdge) *edge.Edge {
	return &edge.Edge{
		ID:            e.ID,
		FlowID:        e.FlowID,
		SourceID:      e.SourceID,
		TargetID:      e.TargetID,
		SourceHandler: e.SourceHandle,
		Kind:          e.EdgeKind,
	}
}
