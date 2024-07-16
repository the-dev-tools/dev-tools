package api

import (
	"context"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/nodemaster"
	"devtools-nodes/pkg/resolver"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	"net/http"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type MasterNodeServer struct{}

func (m MasterNodeServer) Run(ctx context.Context, req *connect.Request[nodemasterv1.NodeMasterServiceRunRequest]) (*connect.Response[nodemasterv1.NodeMasterServiceRunResponse], error) {
	nodes := req.Msg.Nodes

	convertedNodes := make(map[string]mnode.Node)

	for key, node := range nodes {
		msg, err := anypb.UnmarshalNew(node.Data, proto.UnmarshalOptions{})
		if err != nil {
			return nil, err
		}

		castedData, err := resolver.ConvertProtoMsg(msg)
		if err != nil {
			return nil, err
		}

		tempNode := mnode.Node{ID: node.Id, Type: node.Type, Data: castedData, OwnerID: node.OwnerId, GroupID: node.GroupId, Edges: medge.Edges{OutNodes: node.Edges.OutNodes}}
		convertedNodes[key] = tempNode
	}

	nodeMaster, err := nodemaster.NewNodeMaster(req.Msg.StartNodeId, convertedNodes, resolver.ResolveNodeFunc, nil, http.DefaultClient)
	if err != nil {
		return nil, err
	}

	err = nodemaster.Run(nodeMaster)
	if err != nil {
		return nil, err
	}

	respBase := nodemasterv1.NodeMasterServiceRunResponse{Msg: "Sent"}

	resp := connect.NewResponse(&respBase)

	return resp, nil
}

func ListenMasterNodeService() {
	server := &MasterNodeServer{}
	mux := http.NewServeMux()
	path, handler := nodemasterv1connect.NewNodeMasterServiceHandler(server)
	mux.Handle(path, handler)
	http.ListenAndServe(
		"localhost:8080",
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
