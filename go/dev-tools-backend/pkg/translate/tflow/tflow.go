package tflow

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mflow"
	flowv1 "dev-tools-spec/dist/buf/go/flow/v1"
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

func SeralizeRpcToModel(e *flowv1.Flow) (*mflow.Flow, error) {
	flow := SeralizeRpcToModelWithoutID(e)
	id, err := idwrap.NewFromBytes(e.GetFlowId())
	if err != nil {
		return nil, err
	}
	flow.ID = id
	return flow, nil
}

func SeralizeRpcToModelWithoutID(e *flowv1.Flow) *mflow.Flow {
	return &mflow.Flow{
		Name: e.Name,
	}
}
