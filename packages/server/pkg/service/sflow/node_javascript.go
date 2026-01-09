//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

var ErrNoNodeJsFound = sql.ErrNoRows

type NodeJsService struct {
	reader  *NodeJsReader
	queries *gen.Queries
}

func NewNodeJsService(queries *gen.Queries) NodeJsService {
	return NodeJsService{
		reader:  NewNodeJsReaderFromQueries(queries),
		queries: queries,
	}
}

func (nfs NodeJsService) TX(tx *sql.Tx) NodeJsService {
	newQueries := nfs.queries.WithTx(tx)
	return NodeJsService{
		reader:  NewNodeJsReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeJsServiceTX(ctx context.Context, tx *sql.Tx) (*NodeJsService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeJsService{
		reader:  NewNodeJsReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nfs NodeJsService) GetNodeJS(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeJS, error) {
	return nfs.reader.GetNodeJS(ctx, id)
}

func (nfs NodeJsService) CreateNodeJS(ctx context.Context, mn mflow.NodeJS) error {
	return NewNodeJsWriterFromQueries(nfs.queries).CreateNodeJS(ctx, mn)
}

func (nfs NodeJsService) CreateNodeJSBulk(ctx context.Context, jsNodes []mflow.NodeJS) error {
	return NewNodeJsWriterFromQueries(nfs.queries).CreateNodeJSBulk(ctx, jsNodes)
}

func (nfs NodeJsService) UpdateNodeJS(ctx context.Context, mn mflow.NodeJS) error {
	return NewNodeJsWriterFromQueries(nfs.queries).UpdateNodeJS(ctx, mn)
}

func (nfs NodeJsService) DeleteNodeJS(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeJsWriterFromQueries(nfs.queries).DeleteNodeJS(ctx, id)
}

func (s NodeJsService) Reader() *NodeJsReader { return s.reader }
