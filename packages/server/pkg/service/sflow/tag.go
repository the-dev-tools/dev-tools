//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type FlowTagService struct {
	reader  *FlowTagReader
	queries *gen.Queries
}

var ErrNoFlowTag error = sql.ErrNoRows

func NewFlowTagService(queries *gen.Queries) FlowTagService {
	return FlowTagService{
		reader:  NewFlowTagReaderFromQueries(queries),
		queries: queries,
	}
}

func NewFlowTagServiceTX(ctx context.Context, tx *sql.Tx) (*FlowTagService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &FlowTagService{
		reader:  NewFlowTagReaderFromQueries(queries),
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
	return NewFlowTagWriterFromQueries(s.queries).CreateFlowTag(ctx, ftag)
}

func (s *FlowTagService) DeleteFlowTag(ctx context.Context, id idwrap.IDWrap) error {
	return NewFlowTagWriterFromQueries(s.queries).DeleteFlowTag(ctx, id)
}

func (s FlowTagService) Reader() *FlowTagReader { return s.reader }
