package sedge

import (
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type EdgeService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) EdgeService {
	return EdgeService{queries: queries}
}

func ConvertToDBEdge(e edge.Edge) gen.FlowEdge {
	return gen.FlowEdge{
		ID:           e.ID,
		FlowID:       e.FlowID,
		SourceID:     e.SourceID,
		TargetID:     e.TargetID,
		SourceHandle: e.SourceHandler,
	}
}
