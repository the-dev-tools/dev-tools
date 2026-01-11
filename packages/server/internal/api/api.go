//nolint:revive // exported
package api

import (
	"log/slog"
	"net/http"
	"os"
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

// Server mode constants
const (
	ServerModeUDS = "uds"
	ServerModeTCP = "tcp"
)

func newH2CServer(mux *http.ServeMux) *http.Server {
	return &http.Server{
		// NOTE: ConnectRPC requires an address even for Unix sockets.
		// Use a placeholder address since actual routing is via socket.
		Addr:              "the-dev-tools:0",
		ReadHeaderTimeout: 10 * time.Second,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		Handler: h2c.NewHandler(newCORS().Handler(mux), &http2.Server{
			IdleTimeout:          0,
			MaxConcurrentStreams: 100000,
			MaxHandlers:          0,
		}),
	}
}

// ListenServices starts the server listening on either a Unix socket or TCP port.
//
// Environment variables:
//   - SERVER_MODE: "uds" (default) or "tcp"
//   - SERVER_SOCKET_PATH: custom socket path (uds mode, defaults to /tmp/the-dev-tools/server.socket)
//   - PORT: port number (tcp mode, defaults to 8080)
func ListenServices(services []Service, port string) error {
	mux := http.NewServeMux()

	for _, service := range services {
		slog.Info("Registering service", "path", service.Path)
		mux.Handle(service.Path, service.Handler)
	}

	mode := os.Getenv("SERVER_MODE")
	if mode == "" {
		mode = ServerModeUDS
	}

	switch mode {
	case ServerModeTCP:
		return listenTCP(mux, port)
	case ServerModeUDS:
		return listenIPC(mux)
	default:
		slog.Warn("Unknown SERVER_MODE, falling back to uds", "mode", mode)
		return listenIPC(mux)
	}
}

func listenTCP(mux *http.ServeMux, port string) error {
	srv := newH2CServer(mux)
	srv.Addr = ":" + port

	slog.Info("Server listening on TCP", "port", port)
	return srv.ListenAndServe()
}