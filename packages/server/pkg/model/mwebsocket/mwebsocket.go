package mwebsocket

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

type WebSocket struct {
	ID          idwrap.IDWrap  `json:"id"`
	WorkspaceID idwrap.IDWrap  `json:"workspace_id"`
	FolderID    *idwrap.IDWrap `json:"folder_id,omitempty"`
	Name        string         `json:"name"`
	Url         string         `json:"url"`
	Description string         `json:"description"`
	LastRunAt   *int64         `json:"last_run_at,omitempty"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

type WebSocketHeader struct {
	ID           idwrap.IDWrap `json:"id"`
	WebSocketID  idwrap.IDWrap `json:"websocket_id"`
	Key          string        `json:"key"`
	Value        string        `json:"value"`
	Enabled      bool          `json:"enabled"`
	Description  string        `json:"description"`
	DisplayOrder float32       `json:"order"`
	CreatedAt    int64         `json:"created_at"`
	UpdatedAt    int64         `json:"updated_at"`
}
