package snodejs

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
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

func (w *Writer) CreateNodeJS(ctx context.Context, mn mflow.NodeJS) error {
	nodeJS := ConvertModelToDB(mn)
	return w.queries.CreateFlowNodeJs(ctx, gen.CreateFlowNodeJsParams(nodeJS))
}

func (w *Writer) CreateNodeJSBulk(ctx context.Context, jsNodes []mflow.NodeJS) error {
	var err error
	for _, jsNode := range jsNodes {
		err = w.CreateNodeJS(ctx, jsNode)
		if err != nil {
			break
		}
	}
	return err
}

func (w *Writer) UpdateNodeJS(ctx context.Context, mn mflow.NodeJS) error {
	nodeJS := ConvertModelToDB(mn)
	return w.queries.UpdateFlowNodeJs(ctx, gen.UpdateFlowNodeJsParams{
		FlowNodeID:       nodeJS.FlowNodeID,
		Code:             nodeJS.Code,
		CodeCompressType: nodeJS.CodeCompressType,
	})
}

func (w *Writer) DeleteNodeJS(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeJs(ctx, id)
}
