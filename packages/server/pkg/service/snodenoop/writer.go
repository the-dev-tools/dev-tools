package snodenoop

import (
	"context"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
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

func (w *Writer) CreateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	convertedNode := ConvertToDBNodeStart(nf)
	return w.queries.CreateFlowNodeNoop(ctx, gen.CreateFlowNodeNoopParams(convertedNode))
}

func (w *Writer) CreateNodeNoopBulk(ctx context.Context, nf []mnnoop.NoopNode) error {
	for _, n := range nf {
		convertedNode := ConvertToDBNodeStart(n)
		err := w.queries.CreateFlowNodeNoop(ctx, gen.CreateFlowNodeNoopParams(convertedNode))
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) UpdateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	// Since there's no UpdateFlowNodeNoop query, we'll use delete + create pattern
	// Note: In strict segregation, using w.DeleteNodeNoop directly is fine as it's within the Writer struct
	if err := w.DeleteNodeNoop(ctx, nf.FlowNodeID); err != nil && !errors.Is(err, ErrNoNodeForFound) {
		return err
	}
	return w.CreateNodeNoop(ctx, nf)
}

func (w *Writer) DeleteNodeNoop(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowNodeNoop(ctx, id)
	return err // In Writer context, we might return DB errors directly or handle NoRows if needed
}
