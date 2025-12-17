package snodefor

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
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

func (w *Writer) CreateNodeFor(ctx context.Context, nf mnfor.MNFor) error {
	// Preserve UNSPECIFIED to allow default "throw" semantics in flow runner.
	nodeFor := ConvertToDBNodeFor(nf)
	return w.queries.CreateFlowNodeFor(ctx, gen.CreateFlowNodeForParams(nodeFor))
}

func (w *Writer) CreateNodeForBulk(ctx context.Context, nf []mnfor.MNFor) error {
	var err error
	for _, n := range nf {
		err = w.CreateNodeFor(ctx, n)
		if err != nil {
			break
		}
	}
	return err
}

func (w *Writer) UpdateNodeFor(ctx context.Context, nf mnfor.MNFor) error {
	nodeFor := ConvertToDBNodeFor(nf)
	return w.queries.UpdateFlowNodeFor(ctx, gen.UpdateFlowNodeForParams{
		FlowNodeID:    nodeFor.FlowNodeID,
		IterCount:     nodeFor.IterCount,
		ErrorHandling: nodeFor.ErrorHandling,
		Expression:    nodeFor.Expression,
	})
}

func (w *Writer) DeleteNodeFor(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeFor(ctx, id)
}
