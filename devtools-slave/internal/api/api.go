package api

import (
	"context"
	"devtools-nodes/pkg/convert"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/nodemaster"
	"devtools-nodes/pkg/resolver"
	nodeslavev1 "devtools-services/gen/nodeslave/v1"
	"devtools-services/gen/nodeslave/v1/nodeslavev1connect"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type SlaveNodeServer struct{}

func (m SlaveNodeServer) Run(ctx context.Context, req *connect.Request[nodeslavev1.NodeSlaveServiceRunRequest]) (*connect.Response[nodeslavev1.NodeSlaveServiceRunResponse], error) {
	node := req.Msg.Node

	msg, err := anypb.UnmarshalNew(req.Msg.Node.Data, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}

	castedData, err := resolver.ConvertProtoMsg(msg)
	if err != nil {
		return nil, err
	}

	tempNode := mnode.Node{ID: node.Id, Type: node.Type, Data: castedData, OwnerID: node.OwnerId, GroupID: node.GroupId, Edges: medge.Edges{OutNodes: node.Edges.OutNodes}}
	nodes := map[string]mnode.Node{node.Id: tempNode}

	nm, err := nodemaster.NewNodeMaster(node.Id, nodes, resolver.ResolveNodeFunc, nodemaster.ExecuteNode, nil, http.DefaultClient)
	if err != nil {
		return nil, err
	}
	if nm == nil {
		return nil, err
	}
	nm.CurrentNode = &tempNode

	err = nodemaster.ExecuteNode(ctx, nm, resolver.ResolveNodeFunc)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return nil, err
	}
	anyPbArray := make(map[string]*anypb.Any, len(nm.Vars))

	for key, v := range nm.Vars {
		msgMapElement, err := convert.ConvertPrimitiveInterfaceToWrapper(v)
		if err != nil {
			// TODO: if cannot find type send as bytes of something
			continue
		}
		anyPbArray[key] = msgMapElement
	}

	resp := connect.NewResponse(&nodeslavev1.NodeSlaveServiceRunResponse{NextNodeId: nm.NextNodeID})
	return resp, nil
}

func ListenMasterNodeService(port string) error {
	server := &SlaveNodeServer{}
	mux := http.NewServeMux()
	path, handler := nodeslavev1connect.NewNodeSlaveServiceHandler(server)
	mux.Handle(path, handler)
	return http.ListenAndServe(
		":"+port,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
