//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

var ErrNoNodeSubFlowTriggerFound = sql.ErrNoRows

type NodeSubFlowTriggerService struct {
	reader  *NodeSubFlowTriggerReader
	queries *gen.Queries
}

func NewNodeSubFlowTriggerService(queries *gen.Queries) NodeSubFlowTriggerService {
	return NodeSubFlowTriggerService{
		reader:  NewNodeSubFlowTriggerReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeSubFlowTriggerService) TX(tx *sql.Tx) NodeSubFlowTriggerService {
	newQueries := s.queries.WithTx(tx)
	return NodeSubFlowTriggerService{
		reader:  NewNodeSubFlowTriggerReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s NodeSubFlowTriggerService) GetNodeSubFlowTrigger(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeSubFlowTrigger, error) {
	return s.reader.GetNodeSubFlowTrigger(ctx, id)
}

func (s NodeSubFlowTriggerService) CreateNodeSubFlowTrigger(ctx context.Context, m mflow.NodeSubFlowTrigger) error {
	return NewNodeSubFlowTriggerWriterFromQueries(s.queries).CreateNodeSubFlowTrigger(ctx, m)
}

func (s NodeSubFlowTriggerService) UpdateNodeSubFlowTrigger(ctx context.Context, m mflow.NodeSubFlowTrigger) error {
	return NewNodeSubFlowTriggerWriterFromQueries(s.queries).UpdateNodeSubFlowTrigger(ctx, m)
}

func (s NodeSubFlowTriggerService) DeleteNodeSubFlowTrigger(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeSubFlowTriggerWriterFromQueries(s.queries).DeleteNodeSubFlowTrigger(ctx, id)
}

func (s NodeSubFlowTriggerService) Reader() *NodeSubFlowTriggerReader { return s.reader }
