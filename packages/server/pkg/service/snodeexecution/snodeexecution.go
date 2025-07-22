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
	
	return &gen.NodeExecution{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		State:                  ne.State,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		Error:                  errorSQL,
	}
}

func ConvertNodeExecutionToModel(ne gen.NodeExecution) *mnodeexecution.NodeExecution {
	var errorPtr *string
	if ne.Error.Valid {
		errorPtr = &ne.Error.String
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
	
	return s.queries.CreateNodeExecution(ctx, gen.CreateNodeExecutionParams{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		State:                  ne.State,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		Error:                  errorSQL,
	})
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