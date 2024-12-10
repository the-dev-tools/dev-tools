package tflow

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
)

func SeralizeModelToRPCItem(e mflow.Flow) *flowv1.FlowListItem {
	return &flowv1.FlowListItem{
		FlowId: e.ID.Bytes(),
		Name:   e.Name,
	}
}

func SeralizeModelToRPC(e mflow.Flow) *flowv1.Flow {
	return &flowv1.Flow{
		FlowId: e.ID.Bytes(),
		Name:   e.Name,
	}
}

func SeralizeRpcToModel(e *flowv1.Flow, wsID idwrap.IDWrap) (*mflow.Flow, error) {
	flow := SeralizeRpcToModelWithoutID(e, wsID)
	id, err := idwrap.NewFromBytes(e.GetFlowId())
	if err != nil {
		return nil, err
	}
	flow.ID = id
	return flow, nil
}

func SeralizeRpcToModelWithoutID(e *flowv1.Flow, wsID idwrap.IDWrap) *mflow.Flow {
	return &mflow.Flow{
		Name:        e.Name,
		WorkspaceID: wsID,
	}
}
