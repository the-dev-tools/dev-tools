package tflowversion

import (
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

func ModelToRPC(flow mflow.Flow) *flowv1.FlowVersionListItem {
	return &flowv1.FlowVersionListItem{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
}

func RPCToModel(flow mflow.Flow) *flowv1.FlowVersionListItem {
	return &flowv1.FlowVersionListItem{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
}
