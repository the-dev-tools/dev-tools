package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	connectcors "connectrpc.com/cors"
	"github.com/bufbuild/httplb"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Service struct {
	Path    string
	Handler http.Handler
}

func withCORS(h http.Handler) http.Handler {
	middleware := cors.New(cors.Options{
		AllowedOrigins: []string{"localhost:3000", "localhost", "*"},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	})
	return middleware.Handler(h)
}

func ListenServices(services []Service, port string) error {
	upstream := os.Getenv("MASTER_NODE_ENDPOINT")
	if upstream == "" {
		return errors.New("MASTER_NODE_IP env var is required")
	}

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer httpClient.Close()

	mux := http.NewServeMux()

	for _, service := range services {
		log.Printf("Registering service %s", service.Path)
		handler := withCORS(service.Handler)
		mux.Handle(service.Path, handler)
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
