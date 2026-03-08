package sflow

import (
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeRunSubFlow(row gen.FlowNodeRunSubFlow) *mflow.NodeRunSubFlow {
	var inputs []mflow.SubFlowInputMapping
	if len(row.Inputs) > 0 {
		_ = json.Unmarshal(row.Inputs, &inputs)
	}

	return &mflow.NodeRunSubFlow{
		FlowNodeID:     row.FlowNodeID,
		TargetFlowID:   row.TargetFlowID,
		TargetFlowName: row.TargetFlowName,
		Inputs:         inputs,
	}
}

func ConvertNodeRunSubFlowToDB(m mflow.NodeRunSubFlow) gen.FlowNodeRunSubFlow {
	inputs, _ := json.Marshal(m.Inputs)
	if inputs == nil {
		inputs = []byte("[]")
	}

	return gen.FlowNodeRunSubFlow{
		FlowNodeID:     m.FlowNodeID,
		TargetFlowID:   m.TargetFlowID,
		TargetFlowName: m.TargetFlowName,
		Inputs:         inputs,
	}
}
