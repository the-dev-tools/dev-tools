package api

import (
	"context"
	"devtools-nodes/pkg/convert"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/model/mstatus"
	"devtools-nodes/pkg/nodemaster"
	"devtools-nodes/pkg/resolver"
	"devtools-nodes/pkg/status"
	nodeslavev1 "devtools-services/gen/nodeslave/v1"
	"devtools-services/gen/nodeslave/v1/nodeslavev1connect"
	nodestatusv1 "devtools-services/gen/nodestatus/v1"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/anypb"
)

type SlaveNodeServer struct{}

type MultiNodeRunner struct {
	EndNode string
}

func (m SlaveNodeServer) Run(ctx context.Context, req *connect.Request[nodeslavev1.NodeSlaveServiceRunRequest]) (*connect.Response[nodeslavev1.NodeSlaveServiceRunResponse], error) {
	rawNode := req.Msg.Node

	node, err := convert.ConvertMsgNodeToNode(rawNode, resolver.ConvertProtoMsg)
	if err != nil {
		return nil, err
	}

	nm, err := nodemaster.NewNodeMaster(rawNode.Id, nil, resolver.ResolveNodeFunc, nodemaster.ExecuteNode, nil, http.DefaultClient)
	if err != nil {
		return nil, err
	}
	nm.CurrentNode = node

	err = nodemaster.ExecuteNode(ctx, nm, resolver.ResolveNodeFunc)
	if err != nil {
		return nil, err
	}

	updatedVars, err := convert.ConvertVarsToAny(nm.Vars)
	if err != nil {
		return nil, err
	}

	resp := connect.NewResponse(&nodeslavev1.NodeSlaveServiceRunResponse{NextNodeId: nm.NextNodeID, Vars: updatedVars})
	return resp, nil
}

func (m SlaveNodeServer) RunMulti(ctx context.Context, req *connect.Request[nodeslavev1.NodeSlaveServiceRunMultiRequest], stream *connect.ServerStream[nodeslavev1.NodeSlaveServiceRunMultiResponse]) error {
	nodes := req.Msg.Nodes
	convertedNodes, err := convert.ConvertMsgNodesToNodes(nodes, resolver.ConvertProtoMsg)
	if err != nil {
		return err
	}

	resolverFunc := mnodemaster.Resolver(resolver.ResolveNodeFunc)
	multiRunner := MultiNodeRunner{EndNode: req.Msg.StopNodeId}
	executeNodeFunc := mnodemaster.ExcuteNodeFunc(multiRunner.ExecuteNode)

	stateChan := make(chan mstatus.NodeStatus)
	defer close(stateChan)
	nm, err := nodemaster.NewNodeMaster(req.Msg.StartNodeId, convertedNodes, resolverFunc, executeNodeFunc, stateChan, http.DefaultClient)
	if err != nil {
		return err
	}

	finished := make(chan bool)
	defer close(finished)

	funcHandler := func(status mstatus.NodeStatus, anyData *anypb.Any) error {
		err = stream.Send(&nodeslavev1.NodeSlaveServiceRunMultiResponse{
			// TODO: Convert to a normal value not anypb type
			NodeId: status.Type,
			NodeStatus: &nodestatusv1.NodeStatus{
				NodeId: status.NodeID,
				Type:   status.Type,
				Data:   anyData,
			},
		})
		return err
	}

	status.ProxyNotify(ctx, stateChan, convert.ConvertNodeStatusToMsg, funcHandler, finished)

	err = nodemaster.Run(nm, ctx)
	if err != nil {
		log.Println("Error: ", err)
		return err
	}

	finished <- true

	return nil
}

func (m MultiNodeRunner) ExecuteNode(ctx context.Context, nm *mnodemaster.NodeMaster, resolverFunc mnodemaster.Resolver) error {
	err := nodemaster.ExecuteNode(ctx, nm, resolverFunc)
	if err != nil {
		return err
	}

	if nm.NextNodeID == m.EndNode || nm.NextNodeID == "" {
		nm.NextNodeID = ""
		return nil
	}

	/*
		// TODO: move to a function
		statusUpdate := mstatus.NodeStatus{
			Type: mstatus.StatusTypeNextNode,
			Data: mstatus.NodeStatusNextNode{
				NodeID: nm.NextNodeID,
			},
		}

		if nm.StateChan != nil {
			nm.StateChan <- statusUpdate
		}
	*/

	node, err := nodemaster.GetNodeByID(nm, nm.NextNodeID)
	if err != nil {
		return err
	}

	fmt.Printf("NextNodeID: %s \n", nm.NextNodeID)

	nm.CurrentNode = node

	return nil
}

func ListenMasterNodeService(port string) error {
	server := &SlaveNodeServer{}
	mux := http.NewServeMux()
	path, handler := nodeslavev1connect.NewNodeSlaveServiceHandler(server)
	mux.Handle(path, handler)
	return http.ListenAndServe(
		":"+port,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{
			MaxConcurrentStreams: 100000,
			IdleTimeout:          0,
		}),
	)
}
