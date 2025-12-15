//nolint:revive // exported
package api

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
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
	ServerModeUnix = "unix"
	ServerModeTCP  = "tcp"
)

// DefaultServerSocketPath returns the default path for the server Unix socket.
func DefaultServerSocketPath() string {
	return filepath.Join(os.TempDir(), "the-dev-tools", "server.socket")
}

// DefaultWorkerSocketPath returns the default path for the worker-js Unix socket.
func DefaultWorkerSocketPath() string {
	return filepath.Join(os.TempDir(), "the-dev-tools", "worker-js.socket")
}

// ListenServices starts the server listening on either a Unix socket or TCP port.
//
// Environment variables:
//   - SERVER_MODE: "unix" (default) or "tcp"
//   - SERVER_SOCKET_PATH: custom socket path (unix mode, defaults to /tmp/the-dev-tools/server.socket)
//   - PORT: port number (tcp mode, defaults to 8080)
func ListenServices(services []Service, port string) error {
	mux := http.NewServeMux()

	for _, service := range services {
		slog.Info("Registering service", "path", service.Path)
		mux.Handle(service.Path, service.Handler)
	}

	mode := os.Getenv("SERVER_MODE")
	if mode == "" {
		mode = ServerModeUnix
	}

	switch mode {
	case ServerModeTCP:
		return listenTCP(mux, port)
	case ServerModeUnix:
		return listenUnix(mux)
	default:
		slog.Warn("Unknown SERVER_MODE, falling back to unix", "mode", mode)
		return listenUnix(mux)
	}
}

func listenTCP(mux *http.ServeMux, port string) error {
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

	slog.Info("Server listening on TCP", "port", port)
	return srv.ListenAndServe()
}

func listenUnix(mux *http.ServeMux) error {
	socketPath := os.Getenv("SERVER_SOCKET_PATH")
	if socketPath == "" {
		socketPath = DefaultServerSocketPath()
	}

	srv := &http.Server{
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

	socketDir := filepath.Dir(socketPath)

	// Create socket directory if it doesn't exist
	if err := os.MkdirAll(socketDir, 0o750); err != nil {
		return err
	}

	// Remove stale socket file if present (e.g., from a previous crash)
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to remove stale socket", "path", socketPath, "error", err)
	}

	// Create Unix socket listener
	lc := net.ListenConfig{}
	socket, err := lc.Listen(context.Background(), "unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}

	slog.Info("Server listening on Unix socket", "path", socketPath)

	// Ensure socket cleanup on server close
	srv.RegisterOnShutdown(func() {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to remove socket on shutdown", "path", socketPath, "error", err)
		}
	})

	return srv.Serve(socket)
}
