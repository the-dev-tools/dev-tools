package mocknode

import (
	"context"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
)

type MockNode struct {
	ID    idwrap.IDWrap
	Next  []idwrap.IDWrap
	OnRun func()
}

func NewMockNode(id idwrap.IDWrap, next []idwrap.IDWrap, onRun func()) *MockNode {
	return &MockNode{
		ID:    id,
		Next:  next,
		OnRun: onRun,
	}
}

func (mn *MockNode) GetID() idwrap.IDWrap {
	return mn.ID
}

func (mn *MockNode) SetID(id idwrap.IDWrap) {
	mn.ID = id
}

func (mn *MockNode) GetName() string {
	return "mock"
}

func (mn *MockNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	mn.OnRun()
	return node.FlowNodeResult{
		NextNodeID: mn.Next,
		Err:        nil,
	}
}

func (mn *MockNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	mn.OnRun()
	resultChan <- node.FlowNodeResult{
		NextNodeID: mn.Next,
		Err:        nil,
	}
}
