package sflow

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeJsWriter struct {
	queries *gen.Queries
}

func NewNodeJsWriter(tx gen.DBTX) *NodeJsWriter {
	return &NodeJsWriter{queries: gen.New(tx)}
}

func NewNodeJsWriterFromQueries(queries *gen.Queries) *NodeJsWriter {
	return &NodeJsWriter{queries: queries}
}

func (w *NodeJsWriter) CreateNodeJS(ctx context.Context, mn mflow.NodeJS) error {
	nodeJS := ConvertNodeJsToDB(mn)
	return w.queries.CreateFlowNodeJs(ctx, gen.CreateFlowNodeJsParams(nodeJS))
}

func (w *NodeJsWriter) CreateNodeJSBulk(ctx context.Context, jsNodes []mflow.NodeJS) error {
	var err error
	for _, jsNode := range jsNodes {
		err = w.CreateNodeJS(ctx, jsNode)
		if err != nil {
			break
		}
	}
	return err
}

func (w *NodeJsWriter) UpdateNodeJS(ctx context.Context, mn mflow.NodeJS) error {
	nodeJS := ConvertNodeJsToDB(mn)
	return w.queries.UpdateFlowNodeJs(ctx, gen.UpdateFlowNodeJsParams{
		FlowNodeID:       nodeJS.FlowNodeID,
		Code:             nodeJS.Code,
		CodeCompressType: nodeJS.CodeCompressType,
	})
}

func (w *NodeJsWriter) DeleteNodeJS(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeJs(ctx, id)
}
