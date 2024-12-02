package flowlocalrunner

import (
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/idwrap"
)

type FlowLocalRunner struct {
	ID          idwrap.IDWrap
	FlowID      idwrap.IDWrap
	FlowNodeMap map[idwrap.IDWrap]node.FlowNode
	StartNodeID idwrap.IDWrap
}

func CreateFlowRunner(id, flowID, StartNodeID idwrap.IDWrap, FlowNodeMap map[idwrap.IDWrap]node.FlowNode) *FlowLocalRunner {
	return &FlowLocalRunner{
		ID:          id,
		FlowID:      flowID,
		StartNodeID: StartNodeID,
		FlowNodeMap: FlowNodeMap,
	}
}

func (r FlowLocalRunner) Run(chan runner.FlowStatus) error {
	nextNodeID := &r.StartNodeID
	var err error
	variableMap := make(map[string]interface{})
	for nextNodeID != nil {
		node, ok := r.FlowNodeMap[*nextNodeID]
		if !ok {
			return runner.ErrNodeNotFound
		}

		nextNodeID, err = node.Run(variableMap)
		if err != nil {
			return err
		}
	}
	return nil
}
