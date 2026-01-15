package sflow

import (
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeModel(nf gen.FlowNodeModel) *mflow.NodeModel {
	nodeID, _ := idwrap.NewFromBytes(nf.FlowNodeID)
	credID, _ := idwrap.NewFromBytes(nf.CredentialID)

	var temp *float32
	if nf.Temperature.Valid {
		t := float32(nf.Temperature.Float64)
		temp = &t
	}

	var maxTokens *int32
	if nf.MaxTokens.Valid {
		//nolint:gosec // G115: MaxTokens is bounded by LLM API limits, no realistic overflow
		mt := int32(nf.MaxTokens.Int64)
		maxTokens = &mt
	}

	return &mflow.NodeModel{
		FlowNodeID:   nodeID,
		CredentialID: credID,
		Model:        mflow.AiModel(nf.Model),
		Temperature:  temp,
		MaxTokens:    maxTokens,
	}
}

func ConvertNodeModelToDB(mn mflow.NodeModel) gen.FlowNodeModel {
	var temp sql.NullFloat64
	if mn.Temperature != nil {
		temp = sql.NullFloat64{Float64: float64(*mn.Temperature), Valid: true}
	}

	var maxTokens sql.NullInt64
	if mn.MaxTokens != nil {
		maxTokens = sql.NullInt64{Int64: int64(*mn.MaxTokens), Valid: true}
	}

	return gen.FlowNodeModel{
		FlowNodeID:   mn.FlowNodeID.Bytes(),
		CredentialID: mn.CredentialID.Bytes(),
		Model:        int8(mn.Model),
		Temperature:  temp,
		MaxTokens:    maxTokens,
	}
}
