// Package main is a standalone test server that exposes the AuthAdapterService
// over a Unix socket backed by an in-memory SQLite database.
//
// The process prints "READY\n" to stdout once it is accepting connections.
// It exits cleanly on SIGTERM or SIGINT.
//
// Environment variable:
//
//	SOCKET_PATH – Unix socket path (default: /tmp/authadapter-test.socket)
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rauthadapter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/authadapter"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	socketPath := os.Getenv("SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/tmp/authadapter-test.socket"
	}

	ctx := context.Background()

	// In-memory SQLite — full schema applied, no disk writes, destroyed on exit.
	db, err := dbtest.GetTestDB(ctx)
	if err != nil {
		return fmt.Errorf("open test DB: %w", err)
	}
	defer db.Close()

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return fmt.Errorf("prepare queries: %w", err)
	}

	adapter := authadapter.New(queries)
	handler := rauthadapter.New(rauthadapter.AuthAdapterRPCDeps{Adapter: adapter})
	path, httpHandler := rauthadapter.CreateService(handler)

	mux := http.NewServeMux()
	mux.Handle(path, httpHandler)

	srv := &http.Server{
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// Remove stale socket file if present.
	_ = os.Remove(socketPath)

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", socketPath, err)
	}

	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("serve error: %v", serveErr)
		}
	}()

	// Signal TypeScript test runner that we are ready.
	fmt.Println("READY")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	return srv.Close()
}
