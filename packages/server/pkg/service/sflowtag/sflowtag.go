//nolint:revive // exported
package sflowtag

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type FlowTagService struct {
	reader  *Reader
	queries *gen.Queries
}

var ErrNoFlowTag error = sql.ErrNoRows

func New(queries *gen.Queries) FlowTagService {
	return FlowTagService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*FlowTagService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &FlowTagService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (s *FlowTagService) GetFlowTag(ctx context.Context, id idwrap.IDWrap) (mflow.FlowTag, error) {
	return s.reader.GetFlowTag(ctx, id)
}

func (s *FlowTagService) GetFlowTagsByTagID(ctx context.Context, tagID idwrap.IDWrap) ([]mflow.FlowTag, error) {
	return s.reader.GetFlowTagsByTagID(ctx, tagID)
}

func (s *FlowTagService) CreateFlowTag(ctx context.Context, ftag mflow.FlowTag) error {
	return NewWriterFromQueries(s.queries).CreateFlowTag(ctx, ftag)
}

func (s *FlowTagService) DeleteFlowTag(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).DeleteFlowTag(ctx, id)
}
