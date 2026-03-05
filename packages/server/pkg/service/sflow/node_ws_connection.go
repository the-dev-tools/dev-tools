//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWsConnectionService struct {
	reader  *NodeWsConnectionReader
	queries *gen.Queries
}

func NewNodeWsConnectionService(queries *gen.Queries) NodeWsConnectionService {
	return NodeWsConnectionService{
		reader:  NewNodeWsConnectionReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeWsConnectionService) TX(tx *sql.Tx) NodeWsConnectionService {
	newQueries := s.queries.WithTx(tx)
	return NodeWsConnectionService{
		reader:  NewNodeWsConnectionReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s NodeWsConnectionService) GetNodeWsConnection(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeWsConnection, error) {
	return s.reader.GetNodeWsConnection(ctx, id)
}

func (s NodeWsConnectionService) CreateNodeWsConnection(ctx context.Context, n mflow.NodeWsConnection) error {
	return NewNodeWsConnectionWriterFromQueries(s.queries).CreateNodeWsConnection(ctx, n)
}

func (s NodeWsConnectionService) UpdateNodeWsConnection(ctx context.Context, n mflow.NodeWsConnection) error {
	return NewNodeWsConnectionWriterFromQueries(s.queries).UpdateNodeWsConnection(ctx, n)
}

func (s NodeWsConnectionService) DeleteNodeWsConnection(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeWsConnectionWriterFromQueries(s.queries).DeleteNodeWsConnection(ctx, id)
}

func (s NodeWsConnectionService) Reader() *NodeWsConnectionReader { return s.reader }
