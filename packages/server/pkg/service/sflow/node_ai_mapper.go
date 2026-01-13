package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeAi(nf gen.FlowNodeAi) *mflow.NodeAI {
	credID, _ := idwrap.NewFromBytes(nf.CredentialID)
	return &mflow.NodeAI{
		FlowNodeID:    nf.FlowNodeID,
		Model:         mflow.AiModel(nf.Model),
		CredentialID:  credID,
		Prompt:        nf.Prompt,
		MaxIterations: nf.MaxIterations,
	}
}

func ConvertNodeAiToDB(mn mflow.NodeAI) gen.FlowNodeAi {
	return gen.FlowNodeAi{
		FlowNodeID:    mn.FlowNodeID,
		Model:         int8(mn.Model),
		CredentialID:  mn.CredentialID.Bytes(),
		Prompt:        mn.Prompt,
		MaxIterations: mn.MaxIterations,
	}
}
