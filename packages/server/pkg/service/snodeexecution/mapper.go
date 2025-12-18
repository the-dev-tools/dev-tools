package snodeexecution

import (
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertNodeExecutionToDB(ne mflow.NodeExecution) *gen.NodeExecution {
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

	return &gen.NodeExecution{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		Name:                   ne.Name,
		State:                  ne.State,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		Error:                  errorSQL,
		HttpResponseID:         ne.ResponseID,
		CompletedAt:            completedAtSQL,
	}
}

func ConvertNodeExecutionToModel(ne gen.NodeExecution) *mflow.NodeExecution {
	var errorPtr *string
	if ne.Error.Valid {
		errorPtr = &ne.Error.String
	}

	responseIDPtr := ne.HttpResponseID

	var completedAtPtr *int64
	if ne.CompletedAt.Valid {
		completedAtPtr = &ne.CompletedAt.Int64
	}

	return &mflow.NodeExecution{
		ID:                     ne.ID,
		NodeID:                 ne.NodeID,
		Name:                   ne.Name,
		State:                  ne.State,
		InputData:              ne.InputData,
		InputDataCompressType:  ne.InputDataCompressType,
		OutputData:             ne.OutputData,
		OutputDataCompressType: ne.OutputDataCompressType,
		Error:                  errorPtr,
		ResponseID:             responseIDPtr,
		CompletedAt:            completedAtPtr,
	}
}
