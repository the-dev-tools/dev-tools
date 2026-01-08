package sflow

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeRequestWriter struct {
	queries *gen.Queries
}

func NewNodeRequestWriter(tx gen.DBTX) *NodeRequestWriter {
	return &NodeRequestWriter{queries: gen.New(tx)}
}

func NewNodeRequestWriterFromQueries(queries *gen.Queries) *NodeRequestWriter {
	return &NodeRequestWriter{queries: queries}
}

func (w *NodeRequestWriter) CreateNodeRequest(ctx context.Context, nr mflow.NodeRequest) error {
	nodeHTTP, ok := ConvertToDBNodeHTTP(nr)
	if !ok {
		return nil
	}
	return w.queries.CreateFlowNodeHTTP(ctx, gen.CreateFlowNodeHTTPParams(nodeHTTP))
}

func (w *NodeRequestWriter) CreateNodeRequestBulk(ctx context.Context, nodes []mflow.NodeRequest) error {
	for _, node := range nodes {
		nodeHTTP, ok := ConvertToDBNodeHTTP(node)
		if !ok {
			continue
		}

		if err := w.queries.CreateFlowNodeHTTP(ctx, gen.CreateFlowNodeHTTPParams(nodeHTTP)); err != nil {
			return err
		}
	}
	return nil
}

func (w *NodeRequestWriter) UpdateNodeRequest(ctx context.Context, nr mflow.NodeRequest) error {
	nodeHTTP, ok := ConvertToDBNodeHTTP(nr)
	if !ok {
		// Treat removal of HttpID as request to delete any existing binding.
		if err := w.queries.DeleteFlowNodeHTTP(ctx, nr.FlowNodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return nil
	}
	return w.queries.UpdateFlowNodeHTTP(ctx, gen.UpdateFlowNodeHTTPParams(nodeHTTP))
}

func (w *NodeRequestWriter) DeleteNodeRequest(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeHTTP(ctx, id)
}
