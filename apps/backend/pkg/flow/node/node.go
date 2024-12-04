package node

import (
	"context"
	"the-dev-tools/backend/pkg/idwrap"
)

type FlowNode interface {
	GetID() idwrap.IDWrap

	// TODO: will implement streaming in the future
	RunSync(ctx context.Context, variableMap *FlowNodeRequest) FlowNodeResult
	RunAsync(ctx context.Context, variableMap *FlowNodeRequest, resultChan chan FlowNodeResult)
}

type FlowNodeRequest struct {
	VarMap map[string]interface{}
}

type FlowNodeResult struct {
	NextNodeID *idwrap.IDWrap
	Err        error
}
