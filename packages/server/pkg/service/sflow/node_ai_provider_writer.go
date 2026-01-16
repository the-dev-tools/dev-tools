//nolint:revive // exported
package sflow

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// NodeAiProviderWriter writes AI Provider nodes to the database.
type NodeAiProviderWriter struct {
	queries *gen.Queries
}

// NewNodeAiProviderWriterFromQueries creates a writer from existing queries.
func NewNodeAiProviderWriterFromQueries(queries *gen.Queries) *NodeAiProviderWriter {
	return &NodeAiProviderWriter{queries: queries}
}

// CreateNodeAiProvider creates a new AI Provider node.
func (w *NodeAiProviderWriter) CreateNodeAiProvider(ctx context.Context, n mflow.NodeAiProvider) error {
	dbNode := ConvertNodeAiProviderToDB(n)
	return w.queries.CreateFlowNodeAiProvider(ctx, gen.CreateFlowNodeAiProviderParams(dbNode))
}

// UpdateNodeAiProvider updates an existing AI Provider node.
func (w *NodeAiProviderWriter) UpdateNodeAiProvider(ctx context.Context, n mflow.NodeAiProvider) error {
	dbNode := ConvertNodeAiProviderToDB(n)
	return w.queries.UpdateFlowNodeAiProvider(ctx, gen.UpdateFlowNodeAiProviderParams{
		FlowNodeID:   dbNode.FlowNodeID,
		CredentialID: dbNode.CredentialID,
		Model:        dbNode.Model,
		Temperature:  dbNode.Temperature,
		MaxTokens:    dbNode.MaxTokens,
	})
}

// DeleteNodeAiProvider deletes an AI Provider node by its flow node ID.
func (w *NodeAiProviderWriter) DeleteNodeAiProvider(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeAiProvider(ctx, id.Bytes())
}
