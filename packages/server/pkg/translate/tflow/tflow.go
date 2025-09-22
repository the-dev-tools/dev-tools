package tflow

import (
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

func SeralizeModelToRPCItem(flow mflow.Flow) *flowv1.FlowListItem {
	item := &flowv1.FlowListItem{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
	duration := flow.Duration
	item.Duration = &duration
	return item
}

func SeralizeModelToRPC(flow mflow.Flow) *flowv1.Flow {
	msg := &flowv1.Flow{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
	duration := flow.Duration
	msg.Duration = &duration
	return msg
}
