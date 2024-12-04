package node

import (
	"context"
	"the-dev-tools/backend/pkg/idwrap"
)

type FlowNode interface {
	GetID() idwrap.IDWrap

	// TODO: will implement streaming in the future
	RunSync(ctx context.Context, variableMap map[string]interface{}) FlowNodeResult
	RunAsync(ctx context.Context, variableMap map[string]interface{}, resultChan chan FlowNodeResult)
}

type FlowNodeResult struct {
	NextNodeID *idwrap.IDWrap
	Err        error
}
