package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWsSendWriter struct {
	queries *gen.Queries
}

func NewNodeWsSendWriter(tx gen.DBTX) *NodeWsSendWriter {
	return &NodeWsSendWriter{queries: gen.New(tx)}
}

func NewNodeWsSendWriterFromQueries(queries *gen.Queries) *NodeWsSendWriter {
	return &NodeWsSendWriter{queries: queries}
}

func (w *NodeWsSendWriter) CreateNodeWsSend(ctx context.Context, n mflow.NodeWsSend) error {
	dbModel := ConvertToDBNodeWsSend(n)
	return w.queries.CreateFlowNodeWsSend(ctx, gen.CreateFlowNodeWsSendParams(dbModel))
}

func (w *NodeWsSendWriter) UpdateNodeWsSend(ctx context.Context, n mflow.NodeWsSend) error {
	dbModel := ConvertToDBNodeWsSend(n)
	return w.queries.UpdateFlowNodeWsSend(ctx, gen.UpdateFlowNodeWsSendParams(dbModel))
}

func (w *NodeWsSendWriter) DeleteNodeWsSend(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeWsSend(ctx, id)
}
