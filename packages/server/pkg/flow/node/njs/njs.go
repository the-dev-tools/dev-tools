package njs

import (
	"context"
	"fmt"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
	node_js_executorv1 "the-dev-tools/spec/dist/buf/go/node_js_executor/v1"
	"the-dev-tools/spec/dist/buf/go/node_js_executor/v1/node_js_executorv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

type NodeJS struct {
	FlowNodeID   idwrap.IDWrap
	Name         string
	jsCode       string
	nodejsClient node_js_executorv1connect.NodeJsExecutorServiceClient
}

func New(id idwrap.IDWrap, name, jsCode string, nodejsv1Cleint node_js_executorv1connect.NodeJsExecutorServiceClient) *NodeJS {
	return &NodeJS{
		FlowNodeID:   id,
		Name:         name,
		jsCode:       jsCode,
		nodejsClient: nodejsv1Cleint,
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

	result := node.FlowNodeResult{
		NextNodeID: next,
	}

	val, err := structpb.NewValue(req.VarMap)
	if err != nil {
		result.Err = fmt.Errorf("failed to create structpb.Value: %w", err)
		return result
	}

	rpcReqRaw := node_js_executorv1.NodeJsExecutorRunRequest{
		Context: val,
		Code:    n.jsCode,
	}
	rpcReq := connect.NewRequest(&rpcReqRaw)

	rpcResp, err := n.nodejsClient.NodeJsExecutorRun(ctx, rpcReq)
	if err != nil {
		result.Err = fmt.Errorf("failed to execute nodejs: %w", err)
		return result
	}

	InterfaceRaw := rpcResp.Msg.Result.AsInterface()

	if req.VariableTracker != nil {
		err = node.WriteNodeVarRawWithTracking(req, n.Name, InterfaceRaw, req.VariableTracker)
	} else {
		err = node.WriteNodeVarRaw(req, n.Name, InterfaceRaw)
	}
	if err != nil {
		result.Err = fmt.Errorf("failed to write node var bulk: %w", err)
	}

	return result
}

func (n NodeJS) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	next := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleUnspecified)

	result := node.FlowNodeResult{
		NextNodeID: next,
	}

	val, err := structpb.NewValue(req.VarMap)
	if err != nil {
		result.Err = fmt.Errorf("failed to create structpb.Value: %w", err)
		resultChan <- result
		return
	}

	rpcReqRaw := node_js_executorv1.NodeJsExecutorRunRequest{
		Context: val,
		Code:    n.jsCode,
	}
	rpcReq := connect.NewRequest(&rpcReqRaw)

	rpcResp, err := n.nodejsClient.NodeJsExecutorRun(ctx, rpcReq)
	if err != nil {
		result.Err = fmt.Errorf("failed to execute nodejs: %w", err)
		resultChan <- result
		return
	}
	InterfaceRaw := rpcResp.Msg.Result.AsInterface()
	if req.VariableTracker != nil {
		err = node.WriteNodeVarRawWithTracking(req, n.Name, InterfaceRaw, req.VariableTracker)
	} else {
		err = node.WriteNodeVarRaw(req, n.Name, InterfaceRaw)
	}
	if err != nil {
		result.Err = fmt.Errorf("failed to write node var bulk: %w", err)
	}

	resultChan <- result
}
