//go:build windows

package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/Microsoft/go-winio"
)

// DefaultServerSocketPath returns the default path for the server Unix socket.
func DefaultServerSocketPath() string {
	return `\\.\pipe\the-dev-tools_server.socket`
}

// DefaultWorkerSocketPath returns the default path for the worker-js Unix socket.
func DefaultWorkerSocketPath() string {
	return `\\.\pipe\the-dev-tools_worker-js.socket`
}

func listenIPC(mux *http.ServeMux) error {
	socketPath := os.Getenv("SERVER_SOCKET_PATH")
	if socketPath == "" {
		socketPath = DefaultServerSocketPath()
	}

	srv := newH2CServer(mux)

	slog.Info("Server listening on Named Pipe", "path", socketPath)

	listener, err := winio.ListenPipe(socketPath, nil)
	if err != nil {
		return err
	}

	return srv.Serve(listener)
}

func DialWorker(ctx context.Context, socketPath string) (net.Conn, error) {
	return winio.DialPipe(socketPath, nil)
}