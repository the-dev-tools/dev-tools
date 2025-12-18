package sflow

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeForWriter struct {
	queries *gen.Queries
}

func NewNodeForWriter(tx gen.DBTX) *NodeForWriter {
	return &NodeForWriter{queries: gen.New(tx)}
}

func NewNodeForWriterFromQueries(queries *gen.Queries) *NodeForWriter {
	return &NodeForWriter{queries: queries}
}

func (w *NodeForWriter) CreateNodeFor(ctx context.Context, nf mflow.NodeFor) error {
	// Preserve UNSPECIFIED to allow default "throw" semantics in flow runner.
	nodeFor := ConvertNodeForToDB(nf)
	return w.queries.CreateFlowNodeFor(ctx, gen.CreateFlowNodeForParams(nodeFor))
}

func (w *NodeForWriter) CreateNodeForBulk(ctx context.Context, nf []mflow.NodeFor) error {
	var err error
	for _, n := range nf {
		err = w.CreateNodeFor(ctx, n)
		if err != nil {
			break
		}
	}
	return err
}

func (w *NodeForWriter) UpdateNodeFor(ctx context.Context, nf mflow.NodeFor) error {
	nodeFor := ConvertNodeForToDB(nf)
	return w.queries.UpdateFlowNodeFor(ctx, gen.UpdateFlowNodeForParams{
		FlowNodeID:    nodeFor.FlowNodeID,
		IterCount:     nodeFor.IterCount,
		ErrorHandling: nodeFor.ErrorHandling,
		Expression:    nodeFor.Expression,
	})
}

func (w *NodeForWriter) DeleteNodeFor(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeFor(ctx, id)
}
