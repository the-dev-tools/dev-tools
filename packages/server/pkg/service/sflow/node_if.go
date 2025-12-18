//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeIfService struct {
	reader  *NodeIfReader
	queries *gen.Queries
}

func NewNodeIfService(queries *gen.Queries) *NodeIfService {
	return &NodeIfService{
		reader:  NewNodeIfReaderFromQueries(queries),
		queries: queries,
	}
}

func (nifs NodeIfService) TX(tx *sql.Tx) *NodeIfService {
	newQueries := nifs.queries.WithTx(tx)
	return &NodeIfService{
		reader:  NewNodeIfReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeIfServiceTX(ctx context.Context, tx *sql.Tx) (*NodeIfService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeIfService{
		reader:  NewNodeIfReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nifs NodeIfService) GetNodeIf(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeIf, error) {
	return nifs.reader.GetNodeIf(ctx, id)
}

func (nifs NodeIfService) CreateNodeIf(ctx context.Context, ni mflow.NodeIf) error {
	return NewNodeIfWriterFromQueries(nifs.queries).CreateNodeIf(ctx, ni)
}

func (nifs NodeIfService) CreateNodeIfBulk(ctx context.Context, conditionNodes []mflow.NodeIf) error {
	return NewNodeIfWriterFromQueries(nifs.queries).CreateNodeIfBulk(ctx, conditionNodes)
}

func (nifs NodeIfService) UpdateNodeIf(ctx context.Context, ni mflow.NodeIf) error {
	return NewNodeIfWriterFromQueries(nifs.queries).UpdateNodeIf(ctx, ni)
}

func (nifs NodeIfService) DeleteNodeIf(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeIfWriterFromQueries(nifs.queries).DeleteNodeIf(ctx, id)
}

func (s NodeIfService) Reader() *NodeIfReader { return s.reader }
