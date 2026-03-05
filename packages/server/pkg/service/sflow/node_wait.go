//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

var ErrNoNodeWaitFound = sql.ErrNoRows

type NodeWaitService struct {
	reader  *NodeWaitReader
	queries *gen.Queries
}

func NewNodeWaitService(queries *gen.Queries) NodeWaitService {
	return NodeWaitService{
		reader:  NewNodeWaitReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeWaitService) TX(tx *sql.Tx) NodeWaitService {
	newQueries := s.queries.WithTx(tx)
	return NodeWaitService{
		reader:  NewNodeWaitReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s NodeWaitService) GetNodeWait(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeWait, error) {
	return s.reader.GetNodeWait(ctx, id)
}

func (s NodeWaitService) CreateNodeWait(ctx context.Context, mn mflow.NodeWait) error {
	return NewNodeWaitWriterFromQueries(s.queries).CreateNodeWait(ctx, mn)
}

func (s NodeWaitService) UpdateNodeWait(ctx context.Context, mn mflow.NodeWait) error {
	return NewNodeWaitWriterFromQueries(s.queries).UpdateNodeWait(ctx, mn)
}

func (s NodeWaitService) DeleteNodeWait(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeWaitWriterFromQueries(s.queries).DeleteNodeWait(ctx, id)
}

func (s NodeWaitService) Reader() *NodeWaitReader { return s.reader }
