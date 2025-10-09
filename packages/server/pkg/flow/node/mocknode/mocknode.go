package mocknode

import (
	"context"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

type MockNode struct {
	ID    idwrap.IDWrap
	Next  []idwrap.IDWrap
	OnRun func()
	Delay time.Duration
}

func NewMockNode(id idwrap.IDWrap, next []idwrap.IDWrap, onRun func()) *MockNode {
	return &MockNode{
		ID:    id,
		Next:  next,
		OnRun: onRun,
	}
}

func NewDelayedMockNode(id idwrap.IDWrap, next []idwrap.IDWrap, delay time.Duration) *MockNode {
	return &MockNode{
		ID:    id,
		Next:  next,
		Delay: delay,
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
	if mn.OnRun != nil {
		mn.OnRun()
	}
	if mn.Delay > 0 {
		timer := time.NewTimer(mn.Delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return node.FlowNodeResult{Err: ctx.Err()}
		}
	}
	return node.FlowNodeResult{
		NextNodeID: mn.Next,
		Err:        nil,
	}
}

func (mn *MockNode) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	go func() {
		resultChan <- mn.RunSync(ctx, req)
	}()
}
