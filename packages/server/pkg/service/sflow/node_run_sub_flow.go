//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

var ErrNoNodeRunSubFlowFound = sql.ErrNoRows

type NodeRunSubFlowService struct {
	reader  *NodeRunSubFlowReader
	queries *gen.Queries
}

func NewNodeRunSubFlowService(queries *gen.Queries) NodeRunSubFlowService {
	return NodeRunSubFlowService{
		reader:  NewNodeRunSubFlowReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeRunSubFlowService) TX(tx *sql.Tx) NodeRunSubFlowService {
	newQueries := s.queries.WithTx(tx)
	return NodeRunSubFlowService{
		reader:  NewNodeRunSubFlowReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s NodeRunSubFlowService) GetNodeRunSubFlow(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeRunSubFlow, error) {
	return s.reader.GetNodeRunSubFlow(ctx, id)
}

func (s NodeRunSubFlowService) CreateNodeRunSubFlow(ctx context.Context, m mflow.NodeRunSubFlow) error {
	return NewNodeRunSubFlowWriterFromQueries(s.queries).CreateNodeRunSubFlow(ctx, m)
}

func (s NodeRunSubFlowService) UpdateNodeRunSubFlow(ctx context.Context, m mflow.NodeRunSubFlow) error {
	return NewNodeRunSubFlowWriterFromQueries(s.queries).UpdateNodeRunSubFlow(ctx, m)
}

func (s NodeRunSubFlowService) DeleteNodeRunSubFlow(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeRunSubFlowWriterFromQueries(s.queries).DeleteNodeRunSubFlow(ctx, id)
}

func (s NodeRunSubFlowService) Reader() *NodeRunSubFlowReader { return s.reader }
