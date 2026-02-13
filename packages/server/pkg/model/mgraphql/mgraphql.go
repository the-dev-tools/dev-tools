package mgraphql

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

type GraphQL struct {
	ID          idwrap.IDWrap  `json:"id"`
	WorkspaceID idwrap.IDWrap  `json:"workspace_id"`
	FolderID    *idwrap.IDWrap `json:"folder_id,omitempty"`
	Name        string         `json:"name"`
	Url         string         `json:"url"`
	Query       string         `json:"query"`
	Variables   string         `json:"variables"`
	Description string         `json:"description"`
	LastRunAt   *int64         `json:"last_run_at,omitempty"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

type GraphQLHeader struct {
	ID           idwrap.IDWrap `json:"id"`
	GraphQLID    idwrap.IDWrap `json:"graphql_id"`
	Key          string        `json:"key"`
	Value        string        `json:"value"`
	Enabled      bool          `json:"enabled"`
	Description  string        `json:"description"`
	DisplayOrder float32       `json:"order"`
	CreatedAt    int64         `json:"created_at"`
	UpdatedAt    int64         `json:"updated_at"`
}

type GraphQLResponse struct {
	ID        idwrap.IDWrap `json:"id"`
	GraphQLID idwrap.IDWrap `json:"graphql_id"`
	Status    int32         `json:"status"`
	Body      []byte        `json:"body"`
	Time      int64         `json:"time"`
	Duration  int32         `json:"duration"`
	Size      int32         `json:"size"`
	CreatedAt int64         `json:"created_at"`
}

type GraphQLResponseHeader struct {
	ID          idwrap.IDWrap `json:"id"`
	ResponseID  idwrap.IDWrap `json:"response_id"`
	HeaderKey   string        `json:"header_key"`
	HeaderValue string        `json:"header_value"`
	CreatedAt   int64         `json:"created_at"`
}
