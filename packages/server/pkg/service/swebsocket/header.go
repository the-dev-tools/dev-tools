package swebsocket

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mwebsocket"
)

type WebSocketHeaderService struct {
	queries *gen.Queries
}

func NewWebSocketHeaderService(queries *gen.Queries) WebSocketHeaderService {
	return WebSocketHeaderService{queries: queries}
}

func (s WebSocketHeaderService) TX(tx *sql.Tx) WebSocketHeaderService {
	return WebSocketHeaderService{queries: s.queries.WithTx(tx)}
}

func (s WebSocketHeaderService) GetByID(ctx context.Context, id idwrap.IDWrap) (mwebsocket.WebSocketHeader, error) {
	h, err := s.queries.GetWebSocketHeaderByID(ctx, id)
	if err != nil {
		return mwebsocket.WebSocketHeader{}, err
	}
	return convertToModelHeader(h), nil
}

func (s WebSocketHeaderService) GetByWebSocketID(ctx context.Context, wsID idwrap.IDWrap) ([]mwebsocket.WebSocketHeader, error) {
	headers, err := s.queries.GetWebSocketHeaders(ctx, wsID)
	if err != nil {
		return nil, err
	}
	result := make([]mwebsocket.WebSocketHeader, len(headers))
	for i, h := range headers {
		result[i] = convertToModelHeader(h)
	}
	return result, nil
}

func (s WebSocketHeaderService) Create(ctx context.Context, h mwebsocket.WebSocketHeader) error {
	return s.queries.CreateWebSocketHeader(ctx, gen.CreateWebSocketHeaderParams{
		ID:           h.ID,
		WebsocketID:  h.WebSocketID,
		HeaderKey:    h.Key,
		HeaderValue:  h.Value,
		Description:  h.Description,
		Enabled:      h.Enabled,
		DisplayOrder: float64(h.DisplayOrder),
		CreatedAt:    h.CreatedAt,
		UpdatedAt:    h.UpdatedAt,
	})
}

func (s WebSocketHeaderService) Update(ctx context.Context, h mwebsocket.WebSocketHeader) error {
	return s.queries.UpdateWebSocketHeader(ctx, gen.UpdateWebSocketHeaderParams{
		ID:           h.ID,
		HeaderKey:    h.Key,
		HeaderValue:  h.Value,
		Description:  h.Description,
		Enabled:      h.Enabled,
		DisplayOrder: float64(h.DisplayOrder),
	})
}

func (s WebSocketHeaderService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteWebSocketHeader(ctx, id)
}

func (s WebSocketHeaderService) DeleteByWebSocketID(ctx context.Context, wsID idwrap.IDWrap) error {
	return s.queries.DeleteWebSocketHeadersByWebSocketID(ctx, wsID)
}
