// Package client provides a ConnectRPC client for the BetterAuth internal service.
package client

import (
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
)

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 30 * time.Second

// BetterAuthClient wraps the internal auth service client.
type BetterAuthClient struct {
	auth_internalv1connect.AuthInternalServiceClient
}

// NewBetterAuthClient creates a new client for the BetterAuth internal service.
// Uses a default HTTP timeout of 30 seconds.
func NewBetterAuthClient(baseURL string, opts ...connect.ClientOption) *BetterAuthClient {
	httpClient := &http.Client{
		Timeout: DefaultHTTPTimeout,
	}
	return &BetterAuthClient{
		AuthInternalServiceClient: auth_internalv1connect.NewAuthInternalServiceClient(
			httpClient,
			baseURL,
			opts...,
		),
	}
}

// NewBetterAuthClientWithHTTP creates a new client with a custom HTTP client.
func NewBetterAuthClientWithHTTP(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) *BetterAuthClient {
	return &BetterAuthClient{
		AuthInternalServiceClient: auth_internalv1connect.NewAuthInternalServiceClient(
			httpClient,
			baseURL,
			opts...,
		),
	}
}
