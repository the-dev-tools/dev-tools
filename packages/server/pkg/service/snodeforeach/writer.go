package snodeforeach

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{queries: gen.New(tx)}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{queries: queries}
}

func (w *Writer) CreateNodeForEach(ctx context.Context, nf mnforeach.MNForEach) error {
	nodeForEach := ConvertToDBNodeFor(nf)
	return w.queries.CreateFlowNodeForEach(ctx, gen.CreateFlowNodeForEachParams(nodeForEach))
}

func (w *Writer) CreateNodeForEachBulk(ctx context.Context, forEachNodes []mnforeach.MNForEach) error {
	var err error
	for _, forEachNode := range forEachNodes {
		err = w.CreateNodeForEach(ctx, forEachNode)
		if err != nil {
			break
		}
	}
	return err
}

func (w *Writer) UpdateNodeForEach(ctx context.Context, nf mnforeach.MNForEach) error {
	nodeForEach := ConvertToDBNodeFor(nf)
	return w.queries.UpdateFlowNodeForEach(ctx, gen.UpdateFlowNodeForEachParams{
		FlowNodeID:     nodeForEach.FlowNodeID,
		IterExpression: nodeForEach.IterExpression,
		ErrorHandling:  nodeForEach.ErrorHandling,
		Expression:     nodeForEach.Expression,
	})
}

func (w *Writer) DeleteNodeForEach(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeForEach(ctx, id)
}
