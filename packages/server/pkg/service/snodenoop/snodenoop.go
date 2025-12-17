//nolint:revive // exported
package snodenoop

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeNoopService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeNoopService {
	return NodeNoopService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (nns NodeNoopService) TX(tx *sql.Tx) NodeNoopService {
	newQueries := nns.queries.WithTx(tx)
	return NodeNoopService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeNoopService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeNoopService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nfs NodeNoopService) GetNodeNoop(ctx context.Context, id idwrap.IDWrap) (*mnnoop.NoopNode, error) {
	return nfs.reader.GetNodeNoop(ctx, id)
}

func (nfs NodeNoopService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnnoop.NoopNode, error) {
	return nfs.reader.GetNodesByFlowID(ctx, flowID)
}

func (nfs NodeNoopService) CreateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	return NewWriterFromQueries(nfs.queries).CreateNodeNoop(ctx, nf)
}

func (nfs NodeNoopService) CreateNodeNoopBulk(ctx context.Context, nf []mnnoop.NoopNode) error {
	return NewWriterFromQueries(nfs.queries).CreateNodeNoopBulk(ctx, nf)
}

func (nfs NodeNoopService) UpdateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	return NewWriterFromQueries(nfs.queries).UpdateNodeNoop(ctx, nf)
}

func (nfs NodeNoopService) DeleteNodeNoop(ctx context.Context, id idwrap.IDWrap) error {
	err := NewWriterFromQueries(nfs.queries).DeleteNodeNoop(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeForFound
	}
	return err
}