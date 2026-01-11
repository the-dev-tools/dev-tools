//go:build !windows

package api

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// DefaultServerSocketPath returns the default path for the server Unix socket.
func DefaultServerSocketPath() string {
	return filepath.Join(os.TempDir(), "the-dev-tools", "server.socket")
}

// DefaultWorkerSocketPath returns the default path for the worker-js Unix socket.
func DefaultWorkerSocketPath() string {
	return filepath.Join(os.TempDir(), "the-dev-tools", "worker-js.socket")
}

func listenIPC(mux *http.ServeMux) error {
	socketPath := os.Getenv("SERVER_SOCKET_PATH")
	if socketPath == "" {
		socketPath = DefaultServerSocketPath()
	}

	srv := newH2CServer(mux)

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

func DialWorker(ctx context.Context, socketPath string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unix", socketPath)
}