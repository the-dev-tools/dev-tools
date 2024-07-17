package api

import (
	"context"
	"crypto/tls"
	nodemasterv1 "devtools-services/gen/nodemaster/v1"
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	"errors"
	"net"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type MasterNodeServer struct {
	upstream string
	client   nodemasterv1connect.NodeMasterServiceClient
}

func (m MasterNodeServer) Run(ctx context.Context, req *connect.Request[nodemasterv1.NodeMasterServiceRunRequest]) (*connect.Response[nodemasterv1.NodeMasterServiceRunResponse], error) {
	client := m.client
	upstreamReq := connect.NewRequest(req.Msg)
	resp, err := client.Run(ctx, upstreamReq)
	if err != nil {
		return nil, err
	}
	respData := connect.NewResponse(resp.Msg)
	return respData, nil
}

func ListenBackendServerProxy(port string) error {
	upstream := os.Getenv("MASTER_NODE_ENDPOINT")
	if upstream == "" {
		return errors.New("MASTER_NODE_IP env var is required")
	}

	client := nodemasterv1connect.NewNodeMasterServiceClient(newInsecureClient(), upstream)
	if client == nil {
		return errors.New("failed to create client")
	}

	server := &MasterNodeServer{
		upstream: upstream,
		client:   client,
	}
	mux := http.NewServeMux()
	path, handler := nodemasterv1connect.NewNodeMasterServiceHandler(server)
	mux.Handle(path, handler)
	http.ListenAndServe(
		":"+port,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{}),
	)
	return nil
}

func newInsecureClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
				// If you're also using this client for non-h2c traffic, you may want
				// to delegate to tls.Dial if the network isn't TCP or the addr isn't
				// in an allowlist.
				return net.Dial(network, addr)
			},
			// Don't forget timeouts!
		},
	}
}
