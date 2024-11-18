package api

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bufbuild/httplb"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Service struct {
	Handler http.Handler
	Path    string
}

func newCORS() *cors.Cors {
	return cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{
			"Accept",
			"Accept-Encoding",
			"Accept-Post",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Content-Encoding",
			"Grpc-Accept-Encoding",
			"Grpc-Encoding",
			"Grpc-Message",
			"Grpc-Status",
			"Grpc-Status-Details-Bin",
		},
		MaxAge: int(time.Second),
	})
}

func ListenServices(services []Service, port string) error {
	upstream := os.Getenv("MASTER_NODE_ENDPOINT")
	if upstream == "" {
		return errors.New("MASTER_NODE_ENDPOINT env var is required")
	}

	httpClient := httplb.NewClient(httplb.WithDefaultTimeout(time.Hour))
	defer httpClient.Close()

	mux := http.NewServeMux()

	for _, service := range services {
		log.Printf("Registering service %s", service.Path)
		mux.Handle(service.Path, service.Handler)
	}

	return http.ListenAndServe(
		":"+port,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(newCORS().Handler(mux), &http2.Server{
			IdleTimeout:          0,
			MaxConcurrentStreams: 100000,
			MaxHandlers:          0,
		}),
	)
}
