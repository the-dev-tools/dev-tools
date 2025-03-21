package njs

import (
	"context"
	"fmt"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/idwrap"
	nodejs_executorv1 "the-dev-tools/spec/dist/buf/go/nodejs_executor/v1"
	"the-dev-tools/spec/dist/buf/go/nodejs_executor/v1/nodejs_executorv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
)

type NodeJS struct {
	FlowNodeID   idwrap.IDWrap
	Name         string
	jsCode       string
	nodejsClient nodejs_executorv1connect.NodeJSExecutorServiceClient
}

func New(id idwrap.IDWrap, name, jsCode string, nodejsv1Cleint nodejs_executorv1connect.NodeJSExecutorServiceClient) *NodeJS {
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

	rpcReqRaw := nodejs_executorv1.ExecuteNodeJSRequest{
		Context: val,
		Code:    n.jsCode,
	}
	rpcReq := connect.NewRequest(&rpcReqRaw)

	rpcResp, err := n.nodejsClient.ExecuteNodeJS(ctx, rpcReq)
	if err != nil {
		result.Err = fmt.Errorf("failed to execute nodejs: %w", err)
		fmt.Println(result.Err)
		return result
	}

	InterfaceRaw := rpcResp.Msg.Result.AsInterface()

	fmt.Println("return type:", rpcResp.Msg.Result)

	err = node.WriteNodeVarRaw(req, n.Name, InterfaceRaw)
	if err != nil {
		result.Err = fmt.Errorf("failed to write node var bulk: %w", err)
		fmt.Println(result.Err)
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

	rpcReqRaw := nodejs_executorv1.ExecuteNodeJSRequest{
		Context: val,
		Code:    n.jsCode,
	}
	rpcReq := connect.NewRequest(&rpcReqRaw)

	rpcResp, err := n.nodejsClient.ExecuteNodeJS(ctx, rpcReq)
	if err != nil {
		result.Err = fmt.Errorf("failed to execute nodejs: %w", err)
		resultChan <- result
		return
	}
	fmt.Println("return type:", rpcResp.Msg.Result)

	InterfaceRaw := rpcResp.Msg.Result.AsInterface()
	err = node.WriteNodeVarRaw(req, n.Name, InterfaceRaw)
	if err != nil {
		result.Err = fmt.Errorf("failed to write node var bulk: %w", err)
	}

	resultChan <- result
}
