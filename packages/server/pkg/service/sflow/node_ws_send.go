//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWsSendService struct {
	reader  *NodeWsSendReader
	queries *gen.Queries
}

func NewNodeWsSendService(queries *gen.Queries) NodeWsSendService {
	return NodeWsSendService{
		reader:  NewNodeWsSendReaderFromQueries(queries),
		queries: queries,
	}
}

func (s NodeWsSendService) TX(tx *sql.Tx) NodeWsSendService {
	newQueries := s.queries.WithTx(tx)
	return NodeWsSendService{
		reader:  NewNodeWsSendReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s NodeWsSendService) GetNodeWsSend(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeWsSend, error) {
	return s.reader.GetNodeWsSend(ctx, id)
}

func (s NodeWsSendService) CreateNodeWsSend(ctx context.Context, n mflow.NodeWsSend) error {
	return NewNodeWsSendWriterFromQueries(s.queries).CreateNodeWsSend(ctx, n)
}

func (s NodeWsSendService) UpdateNodeWsSend(ctx context.Context, n mflow.NodeWsSend) error {
	return NewNodeWsSendWriterFromQueries(s.queries).UpdateNodeWsSend(ctx, n)
}

func (s NodeWsSendService) DeleteNodeWsSend(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeWsSendWriterFromQueries(s.queries).DeleteNodeWsSend(ctx, id)
}

func (s NodeWsSendService) Reader() *NodeWsSendReader { return s.reader }
