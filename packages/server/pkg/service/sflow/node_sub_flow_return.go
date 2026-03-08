//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

var ErrNoNodeSubFlowReturnFound = sql.ErrNoRows

type NodeSubFlowReturnService struct {
	reader  *NodeSubFlowReturnReader
	queries *gen.Queries
}

func NewNodeSubFlowReturnService(queries *gen.Queries) NodeSubFlowReturnService {
	return NodeSubFlowReturnService{
		reader:  NewNodeSubFlowReturnReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeSubFlowReturnService) TX(tx *sql.Tx) NodeSubFlowReturnService {
	newQueries := s.queries.WithTx(tx)
	return NodeSubFlowReturnService{
		reader:  NewNodeSubFlowReturnReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s NodeSubFlowReturnService) GetNodeSubFlowReturn(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeSubFlowReturn, error) {
	return s.reader.GetNodeSubFlowReturn(ctx, id)
}

func (s NodeSubFlowReturnService) CreateNodeSubFlowReturn(ctx context.Context, m mflow.NodeSubFlowReturn) error {
	return NewNodeSubFlowReturnWriterFromQueries(s.queries).CreateNodeSubFlowReturn(ctx, m)
}

func (s NodeSubFlowReturnService) UpdateNodeSubFlowReturn(ctx context.Context, m mflow.NodeSubFlowReturn) error {
	return NewNodeSubFlowReturnWriterFromQueries(s.queries).UpdateNodeSubFlowReturn(ctx, m)
}

func (s NodeSubFlowReturnService) DeleteNodeSubFlowReturn(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeSubFlowReturnWriterFromQueries(s.queries).DeleteNodeSubFlowReturn(ctx, id)
}

func (s NodeSubFlowReturnService) Reader() *NodeSubFlowReturnReader { return s.reader }
