//nolint:revive // exported
package snodejs

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeJSService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeJSService {
	return NodeJSService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (nfs NodeJSService) TX(tx *sql.Tx) NodeJSService {
	newQueries := nfs.queries.WithTx(tx)
	return NodeJSService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeJSService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeJSService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nfs NodeJSService) GetNodeJS(ctx context.Context, id idwrap.IDWrap) (mnjs.MNJS, error) {
	return nfs.reader.GetNodeJS(ctx, id)
}

func (nfs NodeJSService) CreateNodeJS(ctx context.Context, mn mnjs.MNJS) error {
	return NewWriterFromQueries(nfs.queries).CreateNodeJS(ctx, mn)
}

func (nfs NodeJSService) CreateNodeJSBulk(ctx context.Context, jsNodes []mnjs.MNJS) error {
	return NewWriterFromQueries(nfs.queries).CreateNodeJSBulk(ctx, jsNodes)
}

func (nfs NodeJSService) UpdateNodeJS(ctx context.Context, mn mnjs.MNJS) error {
	return NewWriterFromQueries(nfs.queries).UpdateNodeJS(ctx, mn)
}

func (nfs NodeJSService) DeleteNodeJS(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(nfs.queries).DeleteNodeJS(ctx, id)
}