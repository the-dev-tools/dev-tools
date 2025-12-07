//nolint:revive // exported
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Service struct {
	Handler http.Handler
	Path    string
}

type ServerStreamAdHoc[Res any] interface {
	Send(*Res) error
}

type ClientStreamAdHoc[Req any] interface {
	Receive() (*Req, error)
}

type FullStreamAdHoc[Req, Res any] interface {
	Send(*Res) error
	Receive() (*Req, error)
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
	mux := http.NewServeMux()

	for _, service := range services {
		slog.Info("Registering service", "path", service.Path)
		mux.Handle(service.Path, service.Handler)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		Handler: h2c.NewHandler(newCORS().Handler(mux), &http2.Server{
			IdleTimeout:          0,
			MaxConcurrentStreams: 100000,
			MaxHandlers:          0,
		}),
	}

	return srv.ListenAndServe()
}
