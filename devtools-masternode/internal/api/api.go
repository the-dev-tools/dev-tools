package api

import (
	"context"
	"crypto/tls"
	"devtools-nodes/pkg/convert"
	"devtools-nodes/pkg/model/medge"
	"devtools-nodes/pkg/model/mnode"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/nodemaster"
	"devtools-nodes/pkg/resolver"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	nodeslavev1 "devtools-services/gen/nodeslave/v1"
	"devtools-services/gen/nodeslave/v1/nodeslavev1connect"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type MasterNodeServer struct {
	upstream string
}

func (m MasterNodeServer) ExecuteNode(ctx context.Context, nm *mnodemaster.NodeMaster, resolverFunc mnodemaster.Resolver) error {
	err := nodemaster.ExecuteNode(ctx, nm, resolverFunc)
	if err != nil {
		return err
	}

	log.Printf("Executing node %s\n", nm.CurrentNode.ID)

	castedData, err := convert.ConvertStructToMsg(nm.CurrentNode.Data)
	if err != nil {
		return err
	}
	log.Printf("Converted data: %v\n", castedData)

	currentNode := &nodemasterv1.Node{
		Id:      nm.CurrentNode.ID,
		Type:    nm.CurrentNode.Type,
		Data:    castedData,
		OwnerId: nm.CurrentNode.OwnerID,
		GroupId: nm.CurrentNode.GroupID,
		Edges: &nodemasterv1.Edges{
			OutNodes: nm.CurrentNode.Edges.OutNodes,
			InNodes:  nm.CurrentNode.Edges.InNodes,
		},
	}

	reqMsg := nodeslavev1.NodeSlaveServiceRunRequest{
		Node: currentNode,
	}

	httpClient := httplb.NewClient()
	if httpClient == nil {
		return errors.New("failed to create http client")
	}
	defer httpClient.Close()

	req := connect.NewRequest(&reqMsg)
	if req == nil {
		return errors.New("failed to create request")
	}

	client := nodeslavev1connect.NewNodeSlaveServiceClient(httpClient, m.upstream)
	if client == nil {
		return errors.New("failed to create client")
	}

	// TODO: convert this to a loop
	resp, err := client.Run(ctx, req)
	if err != nil {
		return err
	}

	if resp == nil {
		return errors.New("response is nil")
	}

	if resp.Msg == nil {
		return errors.New("response message is nil")
	}

	fmt.Printf("NextNodeID: %s \n", resp.Msg.NextNodeId)
	nm.NextNodeID = resp.Msg.NextNodeId

	// TODO: convert this into normal value not anypb type
	for key, v := range resp.Msg.Vars {
		nm.Vars[key] = v
	}

	return nil
}

func (m MasterNodeServer) Run(ctx context.Context, req *connect.Request[nodemasterv1.NodeMasterServiceRunRequest]) (*connect.Response[nodemasterv1.NodeMasterServiceRunResponse], error) {
	nodes := req.Msg.Nodes

	// INFO: Experimental change
	convertedNodes := make(map[string]mnode.Node, len(nodes))

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

	resolverFunc := mnodemaster.Resolver(resolver.ResolveNodeFunc)
	executeNodeFunc := mnodemaster.ExcuteNodeFunc(m.ExecuteNode)

	nodeMaster, err := nodemaster.NewNodeMaster(req.Msg.StartNodeId, convertedNodes, resolverFunc, executeNodeFunc, nil, http.DefaultClient)
	if err != nil {
		return nil, err
	}

	err = nodemaster.Run(nodeMaster, ctx)
	if err != nil {
		return nil, err
	}

	respBase := nodemasterv1.NodeMasterServiceRunResponse{Msg: "Ok"}

	resp := connect.NewResponse(&respBase)

	return resp, nil
}

func ListenMasterNodeService(port string) error {
	upstream := os.Getenv("SLAVE_NODE_ENDPOINT")
	if upstream == "" {
		return errors.New("SLAVE_NODE_ENDPOINT env var is required")
	}

	server := &MasterNodeServer{
		upstream: upstream,
	}
	mux := http.NewServeMux()

	path, handler := nodemasterv1connect.NewNodeMasterServiceHandler(server)
	mux.Handle(path, handler)
	serverAddr := ":" + port
	err := http.ListenAndServe(
		serverAddr,
		h2c.NewHandler(mux, &http2.Server{}),
	)
	return err
}

func newInsecureClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
			// Don't forget timeouts!
		},
	}
}
