package rwebsocket

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// WebSocketProxyHandler returns an HTTP handler that proxies WebSocket connections.
// The client connects to this endpoint, which loads headers from the database,
// dials the target WebSocket server with those headers, and relays messages
// bidirectionally between client and target.
func (s *WebSocketRPC) WebSocketProxyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.URL.Query().Get("id")
		if idStr == "" {
			http.Error(w, "missing id query parameter", http.StatusBadRequest)
			return
		}

		wsID, err := idwrap.NewText(idStr)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		ctx := mwauth.CreateAuthedContext(r.Context(), mwauth.LocalDummyID)

		// Fetch WebSocket entity
		wsEntity, err := s.ws.Get(ctx, wsID)
		if err != nil {
			http.Error(w, "websocket not found", http.StatusNotFound)
			return
		}

		// Fetch enabled headers
		headers, err := s.wsh.GetByWebSocketID(ctx, wsEntity.ID)
		if err != nil {
			slog.Error("failed to fetch ws headers", "error", err)
		}

		targetHeaders := http.Header{}
		for _, h := range headers {
			if h.Enabled {
				targetHeaders.Set(h.Key, h.Value)
			}
		}

		// Dial target before accepting client upgrade so failures return HTTP errors
		targetConn, resp, err := websocket.Dial(ctx, wsEntity.Url, &websocket.DialOptions{
			HTTPHeader: targetHeaders,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to connect to target: %v", err), http.StatusBadGateway)
			return
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		defer targetConn.CloseNow() //nolint:errcheck // best-effort cleanup
		targetConn.SetReadLimit(32 * 1024 * 1024) // 32 MB

		// Accept client WebSocket upgrade
		clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			slog.Error("failed to accept client ws upgrade", "error", err)
			return
		}
		defer clientConn.CloseNow() //nolint:errcheck // best-effort cleanup
		clientConn.SetReadLimit(32 * 1024 * 1024) // 32 MB

		// Bidirectional relay
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Client -> Target
		go func() {
			defer cancel()
			for {
				typ, msg, err := clientConn.Read(ctx)
				if err != nil {
					return
				}
				if err := targetConn.Write(ctx, typ, msg); err != nil {
					return
				}
			}
		}()

		// Target -> Client
		for {
			typ, msg, err := targetConn.Read(ctx)
			if err != nil {
				return
			}
			if err := clientConn.Write(ctx, typ, msg); err != nil {
				return
			}
		}
	})
}
