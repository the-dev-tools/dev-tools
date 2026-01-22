package sflow

import (
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeAiProvider(nf gen.FlowNodeAiProvider) *mflow.NodeAiProvider {
	nodeID, _ := idwrap.NewFromBytes(nf.FlowNodeID)

	var credID *idwrap.IDWrap
	if len(nf.CredentialID) > 0 {
		id, err := idwrap.NewFromBytes(nf.CredentialID)
		if err == nil {
			credID = &id
		}
	}

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

	return &mflow.NodeAiProvider{
		FlowNodeID:   nodeID,
		CredentialID: credID,
		Model:        mflow.AiModel(nf.Model),
		Temperature:  temp,
		MaxTokens:    maxTokens,
	}
}

func ConvertNodeAiProviderToDB(mn mflow.NodeAiProvider) gen.FlowNodeAiProvider {
	var temp sql.NullFloat64
	if mn.Temperature != nil {
		temp = sql.NullFloat64{Float64: float64(*mn.Temperature), Valid: true}
	}

	var maxTokens sql.NullInt64
	if mn.MaxTokens != nil {
		maxTokens = sql.NullInt64{Int64: int64(*mn.MaxTokens), Valid: true}
	}

	var credentialID []byte
	if mn.CredentialID != nil {
		credentialID = mn.CredentialID.Bytes()
	}

	return gen.FlowNodeAiProvider{
		FlowNodeID:   mn.FlowNodeID.Bytes(),
		CredentialID: credentialID,
		Model:        int8(mn.Model),
		Temperature:  temp,
		MaxTokens:    maxTokens,
	}
}
