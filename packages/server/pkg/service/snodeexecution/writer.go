package snodeexecution

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnodeexecution"
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

func (w *Writer) CreateNodeExecution(ctx context.Context, ne mnodeexecution.NodeExecution) error {
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
		CompletedAt:            completedAtSQL,
	})

	return err
}

func (w *Writer) UpdateNodeExecution(ctx context.Context, ne mnodeexecution.NodeExecution) error {
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
		CompletedAt:            completedAtSQL,
	})

	return err
}

func (w *Writer) UpsertNodeExecution(ctx context.Context, ne mnodeexecution.NodeExecution) error {
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
		CompletedAt:            completedAtSQL,
	})

	return err
}

func (w *Writer) DeleteNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) error {
	return w.queries.DeleteNodeExecutionsByNodeID(ctx, nodeID)
}

func (w *Writer) DeleteNodeExecutionsByNodeIDs(ctx context.Context, nodeIDs []idwrap.IDWrap) error {
	return w.queries.DeleteNodeExecutionsByNodeIDs(ctx, nodeIDs)
}
