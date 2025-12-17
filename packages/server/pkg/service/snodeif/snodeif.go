//nolint:revive // exported
package snodeif

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
)

type NodeIfService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) *NodeIfService {
	return &NodeIfService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (nifs NodeIfService) TX(tx *sql.Tx) *NodeIfService {
	newQueries := nifs.queries.WithTx(tx)
	return &NodeIfService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeIfService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeIfService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nifs NodeIfService) GetNodeIf(ctx context.Context, id idwrap.IDWrap) (*mnif.MNIF, error) {
	return nifs.reader.GetNodeIf(ctx, id)
}

func (nifs NodeIfService) CreateNodeIf(ctx context.Context, ni mnif.MNIF) error {
	return NewWriterFromQueries(nifs.queries).CreateNodeIf(ctx, ni)
}

func (nifs NodeIfService) CreateNodeIfBulk(ctx context.Context, conditionNodes []mnif.MNIF) error {
	return NewWriterFromQueries(nifs.queries).CreateNodeIfBulk(ctx, conditionNodes)
}

func (nifs NodeIfService) UpdateNodeIf(ctx context.Context, ni mnif.MNIF) error {
	return NewWriterFromQueries(nifs.queries).UpdateNodeIf(ctx, ni)
}

func (nifs NodeIfService) DeleteNodeIf(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(nifs.queries).DeleteNodeIf(ctx, id)
}