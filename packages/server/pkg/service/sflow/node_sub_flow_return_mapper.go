package sflow

import (
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeSubFlowReturn(row gen.FlowNodeSubFlowReturn) *mflow.NodeSubFlowReturn {
	var outputs []mflow.SubFlowOutput
	if len(row.Outputs) > 0 {
		_ = json.Unmarshal(row.Outputs, &outputs)
	}
	return &mflow.NodeSubFlowReturn{
		FlowNodeID: row.FlowNodeID,
		Outputs:    outputs,
	}
}

func ConvertNodeSubFlowReturnToDB(m mflow.NodeSubFlowReturn) gen.FlowNodeSubFlowReturn {
	outputs, _ := json.Marshal(m.Outputs)
	if outputs == nil {
		outputs = []byte("[]")
	}
	return gen.FlowNodeSubFlowReturn{
		FlowNodeID: m.FlowNodeID,
		Outputs:    outputs,
	}
}
