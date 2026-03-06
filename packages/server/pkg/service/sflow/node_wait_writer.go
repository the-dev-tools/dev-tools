package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeWaitWriter struct {
	queries *gen.Queries
}

func NewNodeWaitWriter(tx gen.DBTX) *NodeWaitWriter {
	return &NodeWaitWriter{queries: gen.New(tx)}
}

func NewNodeWaitWriterFromQueries(queries *gen.Queries) *NodeWaitWriter {
	return &NodeWaitWriter{queries: queries}
}

func (w *NodeWaitWriter) CreateNodeWait(ctx context.Context, mn mflow.NodeWait) error {
	nodeWait := ConvertNodeWaitToDB(mn)
	return w.queries.CreateFlowNodeWait(ctx, gen.CreateFlowNodeWaitParams(nodeWait))
}

func (w *NodeWaitWriter) UpdateNodeWait(ctx context.Context, mn mflow.NodeWait) error {
	nodeWait := ConvertNodeWaitToDB(mn)
	return w.queries.UpdateFlowNodeWait(ctx, gen.UpdateFlowNodeWaitParams{
		DurationMs: nodeWait.DurationMs,
		FlowNodeID: nodeWait.FlowNodeID,
	})
}

func (w *NodeWaitWriter) DeleteNodeWait(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeWait(ctx, id)
}
