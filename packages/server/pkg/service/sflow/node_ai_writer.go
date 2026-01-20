//nolint:revive // exported
package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeAIWriter writes AI nodes to the database.
type NodeAIWriter struct {
	queries *gen.Queries
}

// NewNodeAIWriter creates a writer from a transaction.
func NewNodeAIWriter(tx gen.DBTX) *NodeAIWriter {
	return &NodeAIWriter{queries: gen.New(tx)}
}

// NewNodeAIWriterFromQueries creates a writer from existing queries.
func NewNodeAIWriterFromQueries(queries *gen.Queries) *NodeAIWriter {
	return &NodeAIWriter{queries: queries}
}

// CreateNodeAI creates a new AI node.
func (w *NodeAIWriter) CreateNodeAI(ctx context.Context, n mflow.NodeAI) error {
	dbNode := ConvertNodeAiToDB(n)
	return w.queries.CreateFlowNodeAI(ctx, gen.CreateFlowNodeAIParams(dbNode))
}

// UpdateNodeAI updates an existing AI node.
func (w *NodeAIWriter) UpdateNodeAI(ctx context.Context, n mflow.NodeAI) error {
	dbNode := ConvertNodeAiToDB(n)
	return w.queries.UpdateFlowNodeAI(ctx, gen.UpdateFlowNodeAIParams{
		Prompt:        dbNode.Prompt,
		MaxIterations: dbNode.MaxIterations,
		FlowNodeID:    dbNode.FlowNodeID,
	})
}

// DeleteNodeAI deletes an AI node by its flow node ID.
func (w *NodeAIWriter) DeleteNodeAI(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeAI(ctx, id)
}
