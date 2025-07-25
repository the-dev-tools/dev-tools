package snodeexecution

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type NodeExecutionService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeExecutionService {
	return NodeExecutionService{queries: queries}
}

func (s NodeExecutionService) TX(tx *sql.Tx) NodeExecutionService {
	return NodeExecutionService{queries: s.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeExecutionService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeExecutionService{
		queries: queries,
	}, nil
}

func ConvertNodeExecutionToDB(ne mnodeexecution.NodeExecution) *gen.NodeExecution {
	var errorSQL sql.NullString
	if ne.Error != nil {
		errorSQL = sql.NullString{
			String: *ne.Error,
			Valid:  true,
		}
	}
	
	var outputKindSQL sql.NullInt64
	if ne.OutputKind != nil {
		outputKindSQL = sql.NullInt64{
			Int64: int64(*ne.OutputKind),
			Valid: true,
		}
	}
	
	var completedAtSQL sql.NullInt64
	if ne.CompletedAt != nil {
		completedAtSQL = sql.NullInt64{
			Int64: *ne.CompletedAt,
			Valid: true,
		}
	}
	
	return &gen.NodeExecution{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		State:                  ne.State,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		Error:                  errorSQL,
		OutputKind:             outputKindSQL,
		CompletedAt:            completedAtSQL,
	}
}

func ConvertNodeExecutionToModel(ne gen.NodeExecution) *mnodeexecution.NodeExecution {
	var errorPtr *string
	if ne.Error.Valid {
		errorPtr = &ne.Error.String
	}
	
	var outputKindPtr *int8
	if ne.OutputKind.Valid {
		outputKind := int8(ne.OutputKind.Int64)
		outputKindPtr = &outputKind
	}
	
	var completedAtPtr *int64
	if ne.CompletedAt.Valid {
		completedAtPtr = &ne.CompletedAt.Int64
	}
	
	return &mnodeexecution.NodeExecution{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		State:                  ne.State,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		Error:                  errorPtr,
		OutputKind:             outputKindPtr,
		CompletedAt:            completedAtPtr,
	}
}

func (s NodeExecutionService) CreateNodeExecution(ctx context.Context, ne mnodeexecution.NodeExecution) error {
	var errorSQL sql.NullString
	if ne.Error != nil {
		errorSQL = sql.NullString{
			String: *ne.Error,
			Valid:  true,
		}
	}
	
	var outputKindSQL sql.NullInt64
	if ne.OutputKind != nil {
		outputKindSQL = sql.NullInt64{
			Int64: int64(*ne.OutputKind),
			Valid: true,
		}
	}
	
	var completedAtSQL sql.NullInt64
	if ne.CompletedAt != nil {
		completedAtSQL = sql.NullInt64{
			Int64: *ne.CompletedAt,
			Valid: true,
		}
	}
	
	_, err := s.queries.CreateNodeExecution(ctx, gen.CreateNodeExecutionParams{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		State:                  ne.State,
		Error:                  errorSQL,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		OutputKind:             outputKindSQL,
		CompletedAt:            completedAtSQL,
	})
	
	return err
}

func (s NodeExecutionService) GetNodeExecution(ctx context.Context, executionID idwrap.IDWrap) (*mnodeexecution.NodeExecution, error) {
	execution, err := s.queries.GetNodeExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return ConvertNodeExecutionToModel(execution), nil
}

func (s NodeExecutionService) GetNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mnodeexecution.NodeExecution, error) {
	executions, err := s.queries.GetNodeExecutionsByNodeID(ctx, nodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mnodeexecution.NodeExecution{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(executions, ConvertNodeExecutionToModel), nil
}

func (s NodeExecutionService) ListNodeExecutionsByNodeID(ctx context.Context, nodeID idwrap.IDWrap) ([]mnodeexecution.NodeExecution, error) {
	// For now, use the existing method - could add pagination later
	return s.GetNodeExecutionsByNodeID(ctx, nodeID)
}

func (s NodeExecutionService) GetLatestNodeExecutionByNodeID(ctx context.Context, nodeID idwrap.IDWrap) (*mnodeexecution.NodeExecution, error) {
	execution, err := s.queries.GetLatestNodeExecutionByNodeID(ctx, nodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return ConvertNodeExecutionToModel(execution), nil
}