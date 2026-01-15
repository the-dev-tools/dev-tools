//nolint:revive // exported
package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeModelWriter writes Model nodes to the database.
type NodeModelWriter struct {
	queries *gen.Queries
}

// NewNodeModelWriterFromQueries creates a writer from existing queries.
func NewNodeModelWriterFromQueries(queries *gen.Queries) *NodeModelWriter {
	return &NodeModelWriter{queries: queries}
}

// CreateNodeModel creates a new Model node.
func (w *NodeModelWriter) CreateNodeModel(ctx context.Context, n mflow.NodeModel) error {
	dbNode := ConvertNodeModelToDB(n)
	return w.queries.CreateFlowNodeModel(ctx, gen.CreateFlowNodeModelParams(dbNode))
}

// UpdateNodeModel updates an existing Model node.
func (w *NodeModelWriter) UpdateNodeModel(ctx context.Context, n mflow.NodeModel) error {
	dbNode := ConvertNodeModelToDB(n)
	return w.queries.UpdateFlowNodeModel(ctx, gen.UpdateFlowNodeModelParams{
		FlowNodeID:   dbNode.FlowNodeID,
		CredentialID: dbNode.CredentialID,
		Model:        dbNode.Model,
		Temperature:  dbNode.Temperature,
		MaxTokens:    dbNode.MaxTokens,
	})
}

// DeleteNodeModel deletes a Model node by its flow node ID.
func (w *NodeModelWriter) DeleteNodeModel(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeModel(ctx, id.Bytes())
}
