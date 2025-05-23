package tflowversion

import (
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

func ModelToRPC(flow mflow.Flow) *flowv1.FlowVersionsItem {
	return &flowv1.FlowVersionsItem{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
}

func RPCToModel(flow mflow.Flow) *flowv1.FlowVersionsItem {
	return &flowv1.FlowVersionsItem{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
}
