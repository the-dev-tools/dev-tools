package njs

import (
	"context"
	"errors"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/idwrap"
)

type NodeJS struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	jsCode     string
	// nodejsClient node_js_executorv1connect.NodeJsExecutorServiceClient // TODO: restore when executor proto is available
}

func New(id idwrap.IDWrap, name, jsCode string, _ interface{}) *NodeJS {
	return &NodeJS{
		FlowNodeID: id,
		Name:       name,
		jsCode:     jsCode,
		// nodejsClient: nodejsv1Cleint,
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
		Err:        errors.New("node js execution temporarily unavailable"),
	}

	return result
}

func (n NodeJS) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	next := edge.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, edge.HandleUnspecified)

	result := node.FlowNodeResult{
		NextNodeID: next,
		Err:        errors.New("node js execution temporarily unavailable"),
	}

	resultChan <- result
}

/*
// Legacy implementation retained for reference once node_js_executor protos return:

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

...
*/
