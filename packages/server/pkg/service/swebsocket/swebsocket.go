package swebsocket

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mwebsocket"
)

var ErrNoWebSocketFound = sql.ErrNoRows

type WebSocketService struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func New(queries *gen.Queries, logger *slog.Logger) WebSocketService {
	return WebSocketService{queries: queries, logger: logger}
}

func (s WebSocketService) TX(tx *sql.Tx) WebSocketService {
	return WebSocketService{queries: s.queries.WithTx(tx), logger: s.logger}
}

func (s WebSocketService) Get(ctx context.Context, id idwrap.IDWrap) (*mwebsocket.WebSocket, error) {
	ws, err := s.queries.GetWebSocket(ctx, id)
	if err != nil {
		return nil, err
	}
	return convertToModelWebSocket(ws), nil
}

func (s WebSocketService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mwebsocket.WebSocket, error) {
	wsList, err := s.queries.GetWebSocketsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]mwebsocket.WebSocket, len(wsList))
	for i, ws := range wsList {
		result[i] = *convertToModelWebSocket(ws)
	}
	return result, nil
}

func (s WebSocketService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	return s.queries.GetWebSocketWorkspaceID(ctx, id)
}

func (s WebSocketService) Create(ctx context.Context, ws *mwebsocket.WebSocket) error {
	return s.queries.CreateWebSocket(ctx, convertToDBCreateWebSocket(*ws))
}

func (s WebSocketService) Update(ctx context.Context, ws *mwebsocket.WebSocket) error {
	var lastRunAt interface{}
	if ws.LastRunAt != nil {
		lastRunAt = *ws.LastRunAt
	}
	return s.queries.UpdateWebSocket(ctx, gen.UpdateWebSocketParams{
		ID:          ws.ID,
		Name:        ws.Name,
		Url:         ws.Url,
		Description: ws.Description,
		LastRunAt:   lastRunAt,
	})
}

func (s WebSocketService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteWebSocket(ctx, id)
}
