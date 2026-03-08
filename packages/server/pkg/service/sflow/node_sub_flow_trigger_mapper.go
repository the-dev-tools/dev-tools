package sflow

import (
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertDBToNodeSubFlowTrigger(row gen.FlowNodeSubFlowTrigger) *mflow.NodeSubFlowTrigger {
	var params []mflow.SubFlowParam
	if len(row.Params) > 0 {
		_ = json.Unmarshal(row.Params, &params)
	}
	return &mflow.NodeSubFlowTrigger{
		FlowNodeID: row.FlowNodeID,
		Params:     params,
	}
}

func ConvertNodeSubFlowTriggerToDB(m mflow.NodeSubFlowTrigger) gen.FlowNodeSubFlowTrigger {
	params, _ := json.Marshal(m.Params)
	if params == nil {
		params = []byte("[]")
	}
	return gen.FlowNodeSubFlowTrigger{
		FlowNodeID: m.FlowNodeID,
		Params:     params,
	}
}
