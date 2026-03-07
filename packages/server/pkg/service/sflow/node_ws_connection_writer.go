package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWsConnectionWriter struct {
	queries *gen.Queries
}

func NewNodeWsConnectionWriter(tx gen.DBTX) *NodeWsConnectionWriter {
	return &NodeWsConnectionWriter{queries: gen.New(tx)}
}

func NewNodeWsConnectionWriterFromQueries(queries *gen.Queries) *NodeWsConnectionWriter {
	return &NodeWsConnectionWriter{queries: queries}
}

func (w *NodeWsConnectionWriter) CreateNodeWsConnection(ctx context.Context, n mflow.NodeWsConnection) error {
	dbModel, ok := ConvertToDBNodeWsConnection(n)
	if !ok {
		return nil
	}
	return w.queries.CreateFlowNodeWsConnection(ctx, gen.CreateFlowNodeWsConnectionParams(dbModel))
}

func (w *NodeWsConnectionWriter) UpdateNodeWsConnection(ctx context.Context, n mflow.NodeWsConnection) error {
	dbModel, ok := ConvertToDBNodeWsConnection(n)
	if !ok {
		if err := w.queries.DeleteFlowNodeWsConnection(ctx, n.FlowNodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return nil
	}
	return w.queries.UpdateFlowNodeWsConnection(ctx, gen.UpdateFlowNodeWsConnectionParams(dbModel))
}

func (w *NodeWsConnectionWriter) DeleteNodeWsConnection(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeWsConnection(ctx, id)
}
