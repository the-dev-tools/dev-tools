package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeRunSubFlowWriter struct {
	queries *gen.Queries
}

func NewNodeRunSubFlowWriter(tx gen.DBTX) *NodeRunSubFlowWriter {
	return &NodeRunSubFlowWriter{queries: gen.New(tx)}
}

func NewNodeRunSubFlowWriterFromQueries(queries *gen.Queries) *NodeRunSubFlowWriter {
	return &NodeRunSubFlowWriter{queries: queries}
}

func (w *NodeRunSubFlowWriter) CreateNodeRunSubFlow(ctx context.Context, m mflow.NodeRunSubFlow) error {
	row := ConvertNodeRunSubFlowToDB(m)
	return w.queries.CreateFlowNodeRunSubFlow(ctx, gen.CreateFlowNodeRunSubFlowParams(row))
}

func (w *NodeRunSubFlowWriter) UpdateNodeRunSubFlow(ctx context.Context, m mflow.NodeRunSubFlow) error {
	row := ConvertNodeRunSubFlowToDB(m)
	return w.queries.UpdateFlowNodeRunSubFlow(ctx, gen.UpdateFlowNodeRunSubFlowParams{
		TargetFlowID:   row.TargetFlowID,
		TargetFlowName: row.TargetFlowName,
		Inputs:         row.Inputs,
		FlowNodeID:     row.FlowNodeID,
	})
}

func (w *NodeRunSubFlowWriter) DeleteNodeRunSubFlow(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeRunSubFlow(ctx, id)
}
