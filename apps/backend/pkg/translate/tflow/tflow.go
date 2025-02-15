package tflow

import (
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mflowroot"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

// TODO: not need to get full root maybe just name etc
func SeralizeModelToRPCItem(flow mflow.Flow, root mflowroot.FlowRoot) *flowv1.FlowListItem {
	return &flowv1.FlowListItem{
		FlowId:        root.ID.Bytes(),
		FlowVersionId: flow.ID.Bytes(),
		Name:          root.Name,
	}
}

// TODO: not need to get full root maybe just name etc
func SeralizeModelToRPC(flow mflow.Flow, root mflowroot.FlowRoot) *flowv1.Flow {
	return &flowv1.Flow{
		FlowId:        root.ID.Bytes(),
		FlowVersionId: flow.ID.Bytes(),
		Name:          root.Name,
	}
}
