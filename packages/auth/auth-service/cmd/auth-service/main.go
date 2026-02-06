// Package main is the entry point for the auth-service.
// It connects to the BetterAuth internal service and starts the Connect RPC server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/the-dev-tools/dev-tools/packages/auth/auth-service/pkg/client"
	"github.com/the-dev-tools/dev-tools/packages/auth/auth-service/pkg/handler"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth/v1/authv1connect"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	betterAuthURL := getEnv("BETTERAUTH_URL", "http://localhost:50051")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}

	slog.Info("Starting auth-service",
		"betterauth_url", betterAuthURL,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	betterAuthClient := client.NewBetterAuthClient(betterAuthURL)

	authHandler, err := handler.NewAuthHandler(betterAuthClient, []byte(jwtSecret))
	if err != nil {
		slog.Error("Failed to create auth handler", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	interceptors := connect.WithInterceptors(authHandler.AuthInterceptor())

	path, httpHandler := authv1connect.NewAuthServiceHandler(authHandler, interceptors)
	mux.Handle(path, httpHandler)

	addr := getEnv("AUTH_SERVICE_ADDR", ":8081")

	server := &http.Server{
		Addr:         addr,
		Handler:      h2c.NewHandler(mux, &http2.Server{}),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		slog.Info("Auth service listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			cancel()
		}
	}()

	select {
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig)
	case <-ctx.Done():
		slog.Info("Context cancelled")
	}

	slog.Info("Shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	slog.Info("Auth service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var _ authv1connect.AuthServiceHandler = (*handler.AuthHandler)(nil)
