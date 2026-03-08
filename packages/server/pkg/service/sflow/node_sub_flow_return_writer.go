package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeSubFlowReturnWriter struct {
	queries *gen.Queries
}

func NewNodeSubFlowReturnWriter(tx gen.DBTX) *NodeSubFlowReturnWriter {
	return &NodeSubFlowReturnWriter{queries: gen.New(tx)}
}

func NewNodeSubFlowReturnWriterFromQueries(queries *gen.Queries) *NodeSubFlowReturnWriter {
	return &NodeSubFlowReturnWriter{queries: queries}
}

func (w *NodeSubFlowReturnWriter) CreateNodeSubFlowReturn(ctx context.Context, m mflow.NodeSubFlowReturn) error {
	row := ConvertNodeSubFlowReturnToDB(m)
	return w.queries.CreateFlowNodeSubFlowReturn(ctx, gen.CreateFlowNodeSubFlowReturnParams(row))
}

func (w *NodeSubFlowReturnWriter) UpdateNodeSubFlowReturn(ctx context.Context, m mflow.NodeSubFlowReturn) error {
	row := ConvertNodeSubFlowReturnToDB(m)
	return w.queries.UpdateFlowNodeSubFlowReturn(ctx, gen.UpdateFlowNodeSubFlowReturnParams{
		Outputs:    row.Outputs,
		FlowNodeID: row.FlowNodeID,
	})
}

func (w *NodeSubFlowReturnWriter) DeleteNodeSubFlowReturn(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeSubFlowReturn(ctx, id)
}
