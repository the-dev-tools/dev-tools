package snoderequest

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
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

func (w *Writer) CreateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	nodeHTTP, ok := ConvertToDBNodeHTTP(nr)
	if !ok {
		return nil
	}
	return w.queries.CreateFlowNodeHTTP(ctx, gen.CreateFlowNodeHTTPParams(nodeHTTP))
}

func (w *Writer) CreateNodeRequestBulk(ctx context.Context, nodes []mnrequest.MNRequest) error {
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

func (w *Writer) UpdateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
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

func (w *Writer) DeleteNodeRequest(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeHTTP(ctx, id)
}
