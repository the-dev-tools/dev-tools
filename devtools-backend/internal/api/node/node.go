package node

import (
	"context"
	"devtools-backend/internal/api"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	"errors"
	"log"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
)

type MasterNodeServer struct {
	upstream string
	client   nodemasterv1connect.NodeMasterServiceClient
}

func (m MasterNodeServer) Run(ctx context.Context, req *connect.Request[nodemasterv1.NodeMasterServiceRunRequest], stream *connect.ServerStream[nodemasterv1.NodeMasterServiceRunResponse]) error {
	client := m.client
	upstreamReq := connect.NewRequest(req.Msg)
	upstream, err := client.Run(ctx, upstreamReq)
	if err != nil {
		return err
	}
	for upstream.Receive() {
		msg := upstream.Msg()
		err = stream.Send(msg)
		if err != nil {
			log.Fatalf("failed to send message: %v", err)
			return err
		}
	}

	if err := upstream.Err(); err != nil {
		return err
	}

	return nil
}

func CreateService() (*api.Service, error) {
	upstream := os.Getenv("MASTER_NODE_ENDPOINT")
	if upstream == "" {
		return nil, errors.New("MASTER_NODE_IP env var is required")
	}

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer httpClient.Close()

	client := nodemasterv1connect.NewNodeMasterServiceClient(httpClient, upstream)
	if client == nil {
		return nil, errors.New("failed to create client")
	}

	server := &MasterNodeServer{
		upstream: upstream,
		client:   client,
	}
	path, handler := nodemasterv1connect.NewNodeMasterServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
