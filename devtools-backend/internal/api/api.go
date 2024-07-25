package api

import (
	"devtools-services/gen/nodemaster/v1/nodemasterv1connect"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/bufbuild/httplb"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Service struct {
	Path    string
	Handler http.Handler
}

func ListenServices(services []Service, port string) error {
	upstream := os.Getenv("MASTER_NODE_ENDPOINT")
	if upstream == "" {
		return errors.New("MASTER_NODE_IP env var is required")
	}

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer httpClient.Close()

	client := nodemasterv1connect.NewNodeMasterServiceClient(httpClient, upstream)
	if client == nil {
		return errors.New("failed to create client")
	}

	mux := http.NewServeMux()

	for _, service := range services {
		mux.Handle(service.Path, service.Handler)
	}

	return http.ListenAndServe(
		":"+port,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{
			IdleTimeout:          0,
			MaxConcurrentStreams: 100000,
			MaxHandlers:          0,
		}),
	)
}
