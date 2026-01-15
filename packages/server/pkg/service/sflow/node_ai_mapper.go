package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeAi(nf gen.FlowNodeAi) *mflow.NodeAI {
	return &mflow.NodeAI{
		FlowNodeID:    nf.FlowNodeID,
		Prompt:        nf.Prompt,
		MaxIterations: nf.MaxIterations,
	}
}

func ConvertNodeAiToDB(mn mflow.NodeAI) gen.FlowNodeAi {
	return gen.FlowNodeAi{
		FlowNodeID:    mn.FlowNodeID,
		Prompt:        mn.Prompt,
		MaxIterations: mn.MaxIterations,
	}
}
