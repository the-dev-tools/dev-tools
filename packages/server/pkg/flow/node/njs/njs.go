package njs

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	node_js_executorv1 "the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1"
	"the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
)

type NodeJS struct {
	FlowNodeID   idwrap.IDWrap
	Name         string
	jsCode       string
	nodejsClient node_js_executorv1connect.NodeJsExecutorServiceClient
}

// New creates a new NodeJS instance. If nodejsClient is nil, execution will return an error.
func New(id idwrap.IDWrap, name, jsCode string, nodejsClient node_js_executorv1connect.NodeJsExecutorServiceClient) *NodeJS {
	return &NodeJS{
		FlowNodeID:   id,
		Name:         name,
		jsCode:       jsCode,
		nodejsClient: nodejsClient,
	}
}

func (n NodeJS) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n *NodeJS) SetID(id idwrap.IDWrap) {
	n.FlowNodeID = id
}

func (n NodeJS) GetName() string {
	return n.Name
}

func (n NodeJS) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	next := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleUnspecified)

	if n.nodejsClient == nil {
		return node.FlowNodeResult{
			NextNodeID: next,
			Err:        errors.New("JS executor not available - Node.js worker not running"),
		}
	}

	// Build context from variables
	contextValue, err := node.BuildContextValue(req.VarMap)
	if err != nil {
		return node.FlowNodeResult{
			NextNodeID: next,
			Err:        fmt.Errorf("failed to build context for JS execution: %w", err),
		}
	}

	// Execute JS code via RPC
	resp, err := n.nodejsClient.NodeJsExecutorRun(ctx, connect.NewRequest(&node_js_executorv1.NodeJsExecutorRunRequest{
		Code:    n.jsCode,
		Context: contextValue,
	}))
	if err != nil {
		return node.FlowNodeResult{
			NextNodeID: next,
			Err:        fmt.Errorf("JS execution failed: %w", err),
		}
	}

	// Store result in variables if available
	if resp.Msg.Result != nil {
		resultMap, err := node.ParseResultValue(resp.Msg.Result)
		if err == nil {
			for k, v := range resultMap {
				node.WriteVar(req, k, v)
			}
		}
	}

	return node.FlowNodeResult{
		NextNodeID: next,
		Err:        nil,
	}
}

func (n NodeJS) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	resultChan <- result
}
