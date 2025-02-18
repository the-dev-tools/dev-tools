package tflow

import (
	"the-dev-tools/backend/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

// TODO: not need to get full root maybe just name etc
func SeralizeModelToRPCItem(flow mflow.Flow) *flowv1.FlowListItem {
	return &flowv1.FlowListItem{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
}

// TODO: not need to get full root maybe just name etc
func SeralizeModelToRPC(flow mflow.Flow) *flowv1.Flow {
	return &flowv1.Flow{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
}
