package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeExecutionWriter struct {
	queries *gen.Queries
}

func NewNodeExecutionWriter(tx gen.DBTX) *NodeExecutionWriter {
	return &NodeExecutionWriter{queries: gen.New(tx)}
}

func NewNodeExecutionWriterFromQueries(queries *gen.Queries) *NodeExecutionWriter {
	return &NodeExecutionWriter{queries: queries}
}

func (w *NodeExecutionWriter) CreateNodeExecution(ctx context.Context, ne mflow.NodeExecution) error {
	var errorSQL sql.NullString
	if ne.Error != nil {
		errorSQL = sql.NullString{
			String: *ne.Error,
			Valid:  true,
		}
	}

	var completedAtSQL sql.NullInt64
	if ne.CompletedAt != nil {
		completedAtSQL = sql.NullInt64{
			Int64: *ne.CompletedAt,
			Valid: true,
		}
	}

	_, err := w.queries.CreateNodeExecution(ctx, gen.CreateNodeExecutionParams{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		Name:                   ne.Name,
		State:                  ne.State,
		Error:                  errorSQL,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		HttpResponseID:         ne.ResponseID,
		GraphqlResponseID:      ne.GraphQLResponseID,
		CompletedAt:            completedAtSQL,
	})

	return err
}

func (w *NodeExecutionWriter) UpdateNodeExecution(ctx context.Context, ne mflow.NodeExecution) error {
	var errorSQL sql.NullString
	if ne.Error != nil {
		errorSQL = sql.NullString{
			String: *ne.Error,
			Valid:  true,
		}
	}

	var completedAtSQL sql.NullInt64
	if ne.CompletedAt != nil {
		completedAtSQL = sql.NullInt64{
			Int64: *ne.CompletedAt,
			Valid: true,
		}
	}

	_, err := w.queries.UpdateNodeExecution(ctx, gen.UpdateNodeExecutionParams{
		ID:                     ne.ID,
		State:                  ne.State,
		Error:                  errorSQL,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		HttpResponseID:         ne.ResponseID,
		GraphqlResponseID:      ne.GraphQLResponseID,
		CompletedAt:            completedAtSQL,
	})

	return err
}

func (w *NodeExecutionWriter) UpsertNodeExecution(ctx context.Context, ne mflow.NodeExecution) error {
	var errorSQL sql.NullString
	if ne.Error != nil {
		errorSQL = sql.NullString{
			String: *ne.Error,
			Valid:  true,
		}
	}

	var completedAtSQL sql.NullInt64
	if ne.CompletedAt != nil {
		completedAtSQL = sql.NullInt64{
			Int64: *ne.CompletedAt,
			Valid: true,
		}
	}

	_, err := w.queries.UpsertNodeExecution(ctx, gen.UpsertNodeExecutionParams{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		Name:                   ne.Name,
		State:                  ne.State,
		Error:                  errorSQL,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		HttpResponseID:         ne.ResponseID,
		GraphqlResponseID:      ne.GraphQLResponseID,
		CompletedAt:            completedAtSQL,
	})

	return err
}

func (w *NodeExecutionWriter) DeleteNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) error {
	return w.queries.DeleteNodeExecutionsByNodeID(ctx, nodeID)
}

func (w *NodeExecutionWriter) DeleteNodeExecutionsByNodeIDs(ctx context.Context, nodeIDs []idwrap.IDWrap) error {
	return w.queries.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)
}

// UpdateNodeExecutionNodeID updates the node_id of a node execution
// This is used to move executions from parent nodes to version nodes
func (w *NodeExecutionWriter) UpdateNodeExecutionNodeID(ctx context.Context, execID, newNodeID idwrap.IDWrap) error {
	return w.queries.UpdateNodeExecutionNodeID(ctx, gen.UpdateNodeExecutionNodeIDParams{
		ID:     execID,
		NodeID: newNodeID,
	})
}
