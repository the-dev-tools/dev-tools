package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeSubFlowTriggerWriter struct {
	queries *gen.Queries
}

func NewNodeSubFlowTriggerWriter(tx gen.DBTX) *NodeSubFlowTriggerWriter {
	return &NodeSubFlowTriggerWriter{queries: gen.New(tx)}
}

func NewNodeSubFlowTriggerWriterFromQueries(queries *gen.Queries) *NodeSubFlowTriggerWriter {
	return &NodeSubFlowTriggerWriter{queries: queries}
}

func (w *NodeSubFlowTriggerWriter) CreateNodeSubFlowTrigger(ctx context.Context, m mflow.NodeSubFlowTrigger) error {
	row := ConvertNodeSubFlowTriggerToDB(m)
	return w.queries.CreateFlowNodeSubFlowTrigger(ctx, gen.CreateFlowNodeSubFlowTriggerParams(row))
}

func (w *NodeSubFlowTriggerWriter) UpdateNodeSubFlowTrigger(ctx context.Context, m mflow.NodeSubFlowTrigger) error {
	row := ConvertNodeSubFlowTriggerToDB(m)
	return w.queries.UpdateFlowNodeSubFlowTrigger(ctx, gen.UpdateFlowNodeSubFlowTriggerParams{
		Params:     row.Params,
		FlowNodeID: row.FlowNodeID,
	})
}

func (w *NodeSubFlowTriggerWriter) DeleteNodeSubFlowTrigger(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeSubFlowTrigger(ctx, id)
}
