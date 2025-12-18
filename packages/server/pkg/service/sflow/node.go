//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

var ErrNoNodeFound error = sql.ErrNoRows

type NodeService struct {
	reader  *NodeReader
	queries *gen.Queries
}

func NewNodeService(queries *gen.Queries) NodeService {
	return NodeService{
		reader:  NewNodeReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeService) TX(tx *sql.Tx) NodeService {
	newQueries := s.queries.WithTx(tx)
	return NodeService{
		reader:  NewNodeReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeServiceTX(ctx context.Context, tx *sql.Tx) (*NodeService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeService{
		reader:  NewNodeReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (ns NodeService) GetNode(ctx context.Context, id idwrap.IDWrap) (*mflow.Node, error) {
	return ns.reader.GetNode(ctx, id)
}

func (ns NodeService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.Node, error) {
	return ns.reader.GetNodesByFlowID(ctx, flowID)
}

func (ns NodeService) CreateNode(ctx context.Context, n mflow.Node) error {
	return NewNodeWriterFromQueries(ns.queries).CreateNode(ctx, n)
}

func (ns NodeService) CreateNodeBulk(ctx context.Context, nodes []mflow.Node) error {
	return NewNodeWriterFromQueries(ns.queries).CreateNodeBulk(ctx, nodes)
}

func (ns NodeService) UpdateNode(ctx context.Context, n mflow.Node) error {
	return NewNodeWriterFromQueries(ns.queries).UpdateNode(ctx, n)
}

func (ns NodeService) DeleteNode(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeWriterFromQueries(ns.queries).DeleteNode(ctx, id)
}

func (s NodeService) Reader() *NodeReader { return s.reader }
