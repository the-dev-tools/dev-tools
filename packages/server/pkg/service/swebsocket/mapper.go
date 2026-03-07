package swebsocket

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mwebsocket"
)

func convertToModelWebSocket(db gen.Websocket) *mwebsocket.WebSocket {
	ws := &mwebsocket.WebSocket{
		ID:          db.ID,
		WorkspaceID: db.WorkspaceID,
		FolderID:    db.FolderID,
		Name:        db.Name,
		Url:         db.Url,
		Description: db.Description,
		CreatedAt:   db.CreatedAt,
		UpdatedAt:   db.UpdatedAt,
	}

	if db.LastRunAt != nil {
		if v, ok := db.LastRunAt.(int64); ok {
			ws.LastRunAt = &v
		}
	}

	return ws
}

func convertToDBCreateWebSocket(ws mwebsocket.WebSocket) gen.CreateWebSocketParams {
	p := gen.CreateWebSocketParams{
		ID:          ws.ID,
		WorkspaceID: ws.WorkspaceID,
		FolderID:    ws.FolderID,
		Name:        ws.Name,
		Url:         ws.Url,
		Description: ws.Description,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
	if ws.LastRunAt != nil {
		p.LastRunAt = *ws.LastRunAt
	}
	return p
}

func convertToModelHeader(db gen.WebsocketHeader) mwebsocket.WebSocketHeader {
	return mwebsocket.WebSocketHeader{
		ID:           db.ID,
		WebSocketID:  db.WebsocketID,
		Key:          db.HeaderKey,
		Value:        db.HeaderValue,
		Enabled:      db.Enabled,
		Description:  db.Description,
		DisplayOrder: float32(db.DisplayOrder),
		CreatedAt:    db.CreatedAt,
		UpdatedAt:    db.UpdatedAt,
	}
}
