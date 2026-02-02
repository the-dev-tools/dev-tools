//nolint:revive // exported
package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeMemoryWriter writes Memory nodes to the database.
type NodeMemoryWriter struct {
	queries *gen.Queries
}

// NewNodeMemoryWriterFromQueries creates a writer from existing queries.
func NewNodeMemoryWriterFromQueries(queries *gen.Queries) *NodeMemoryWriter {
	return &NodeMemoryWriter{queries: queries}
}

// CreateNodeMemory creates a new Memory node.
func (w *NodeMemoryWriter) CreateNodeMemory(ctx context.Context, n mflow.NodeMemory) error {
	dbNode := ConvertNodeMemoryToDB(n)
	return w.queries.CreateFlowNodeMemory(ctx, gen.CreateFlowNodeMemoryParams(dbNode))
}

// UpdateNodeMemory updates an existing Memory node.
func (w *NodeMemoryWriter) UpdateNodeMemory(ctx context.Context, n mflow.NodeMemory) error {
	dbNode := ConvertNodeMemoryToDB(n)
	return w.queries.UpdateFlowNodeMemory(ctx, gen.UpdateFlowNodeMemoryParams{
		FlowNodeID: dbNode.FlowNodeID,
		MemoryType: dbNode.MemoryType,
		WindowSize: dbNode.WindowSize,
	})
}

// DeleteNodeMemory deletes a Memory node by its flow node ID.
func (w *NodeMemoryWriter) DeleteNodeMemory(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeMemory(ctx, id.Bytes())
}
