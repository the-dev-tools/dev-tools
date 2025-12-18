//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

var ErrNoNodeNoopFound = sql.ErrNoRows

type NodeNoopService struct {
	reader  *NodeNoopReader
	queries *gen.Queries
}

func NewNodeNoopService(queries *gen.Queries) NodeNoopService {
	return NodeNoopService{
		reader:  NewNodeNoopReaderFromQueries(queries),
		queries: queries,
	}
}

func (nns NodeNoopService) TX(tx *sql.Tx) NodeNoopService {
	newQueries := nns.queries.WithTx(tx)
	return NodeNoopService{
		reader:  NewNodeNoopReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeNoopServiceTX(ctx context.Context, tx *sql.Tx) (*NodeNoopService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeNoopService{
		reader:  NewNodeNoopReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nfs NodeNoopService) GetNodeNoop(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeNoop, error) {
	return nfs.reader.GetNodeNoop(ctx, id)
}

func (nfs NodeNoopService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.NodeNoop, error) {
	return nfs.reader.GetNodesByFlowID(ctx, flowID)
}

func (nfs NodeNoopService) CreateNodeNoop(ctx context.Context, nf mflow.NodeNoop) error {
	return NewNodeNoopWriterFromQueries(nfs.queries).CreateNodeNoop(ctx, nf)
}

func (nfs NodeNoopService) CreateNodeNoopBulk(ctx context.Context, nf []mflow.NodeNoop) error {
	return NewNodeNoopWriterFromQueries(nfs.queries).CreateNodeNoopBulk(ctx, nf)
}

func (nfs NodeNoopService) UpdateNodeNoop(ctx context.Context, nf mflow.NodeNoop) error {
	return NewNodeNoopWriterFromQueries(nfs.queries).UpdateNodeNoop(ctx, nf)
}

func (nfs NodeNoopService) DeleteNodeNoop(ctx context.Context, id idwrap.IDWrap) error {
	err := NewNodeNoopWriterFromQueries(nfs.queries).DeleteNodeNoop(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeNoopFound
	}
	return err
}

func (s NodeNoopService) Reader() *NodeNoopReader { return s.reader }
