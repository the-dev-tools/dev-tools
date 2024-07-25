package api

import (
	"context"
	"crypto/tls"
	"devtools-nodes/pkg/convert"
	"devtools-nodes/pkg/model/mnodemaster"
	"devtools-nodes/pkg/model/mstatus"
	"devtools-nodes/pkg/nodemaster"
	"devtools-nodes/pkg/resolver"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	nodeslavev1 "devtools-services/gen/nodeslave/v1"
	"devtools-services/gen/nodeslave/v1/nodeslavev1connect"
	nodestatusv1 "devtools-services/gen/nodestatus/v1"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type MasterNodeServer struct {
	Upstream string
}

func (m MasterNodeServer) ExecuteNode(ctx context.Context, nm *mnodemaster.NodeMaster, resolverFunc mnodemaster.Resolver) error {
	castedData, err := convert.ConvertStructToMsg(nm.CurrentNode.Data)
	if err != nil {
		return err
	}

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

	/*
		if currentNode.Type == resolver.NodeTypeLoopRemote {
			err := nodemaster.ExecuteNode(ctx, nm, resolverFunc)
			return err
		}
	*/
	if currentNode.Type == resolver.NodeTypeLoopRemote {
		err := nodemaster.ExecuteNode(ctx, nm, resolverFunc)
		if err != nil {
			log.Printf("Error: %v", err)
			return err
		}
		return nil
	}

	reqMsg := nodeslavev1.NodeSlaveServiceRunRequest{
		Node: currentNode,
	}

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	if httpClient == nil {
		return errors.New("failed to create http client")
	}
	defer httpClient.Close()

	req := connect.NewRequest(&reqMsg)
	if req == nil {
		return errors.New("failed to create request")
	}

	client := nodeslavev1connect.NewNodeSlaveServiceClient(httpClient, m.Upstream)
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

	nodemaster.SetNextNode(nm, resp.Msg.NextNodeId)

	// TODO: convert this into normal value not anypb type
	for key, v := range resp.Msg.Vars {
		nm.Vars[key] = v
	}

	return nil
}

func (m MasterNodeServer) Run(ctx context.Context, req *connect.Request[nodemasterv1.NodeMasterServiceRunRequest], stream *connect.ServerStream[nodemasterv1.NodeMasterServiceRunResponse]) error {
	nodes := req.Msg.Nodes

	// INFO: Experimental change

	convertedNodes, err := convert.ConvertMsgNodesToNodes(nodes, resolver.ConvertProtoMsg)
	if err != nil {
		return err
	}

	resolverFunc := mnodemaster.Resolver(resolver.ResolveNodeFunc)
	executeNodeFunc := mnodemaster.ExcuteNodeFunc(m.ExecuteNode)

	stateChan := make(chan mstatus.NodeStatus)
	defer close(stateChan)
	nodeMaster, err := nodemaster.NewNodeMaster(req.Msg.StartNodeId, convertedNodes, resolverFunc, executeNodeFunc, stateChan, http.DefaultClient)
	if err != nil {
		log.Fatal(err)
		return err
	}

	finished := make(chan bool)
	defer close(finished)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-finished:
				return
			case statusUpdate := <-stateChan:

				statusData, err := convert.ConvertNodeStatusToMsg(statusUpdate.Data)
				if err != nil {
					log.Printf("Error: %v", err)
					continue
				}

				err = stream.Send(&nodemasterv1.NodeMasterServiceRunResponse{
					// TODO: Convert to a normal value not anypb type
					Msg: fmt.Sprintf("Type: %s", statusUpdate.Type),
					NodeUpdate: &nodestatusv1.NodeStatus{
						NodeId: statusUpdate.NodeID,
						Data:   statusData,
					},
				})
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}()

	err = nodemaster.Run(nodeMaster, ctx)
	if err != nil {
		if err != context.Canceled {
			return nil
		}
		log.Fatal(err)
		return err
	}
	finished <- true

	return nil
}

func ListenMasterNodeService(port string) error {
	upstream := os.Getenv("SLAVE_NODE_ENDPOINT")
	if upstream == "" {
		return errors.New("SLAVE_NODE_ENDPOINT env var is required")
	}

	server := &MasterNodeServer{
		Upstream: upstream,
	}
	mux := http.NewServeMux()

	path, handler := nodemasterv1connect.NewNodeMasterServiceHandler(server)
	mux.Handle(path, handler)
	serverAddr := ":" + port
	err := http.ListenAndServe(
		serverAddr,
		h2c.NewHandler(mux, &http2.Server{
			IdleTimeout:          0,
			MaxConcurrentStreams: 100000,
		}),
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
