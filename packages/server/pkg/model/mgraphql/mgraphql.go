package mgraphql

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

type GraphQL struct {
	ID               idwrap.IDWrap  `json:"id"`
	WorkspaceID      idwrap.IDWrap  `json:"workspace_id"`
	FolderID         *idwrap.IDWrap `json:"folder_id,omitempty"`
	Name             string         `json:"name"`
	Url              string         `json:"url"`
	Query            string         `json:"query"`
	Variables        string         `json:"variables"`
	Description      string         `json:"description"`
	ParentGraphQLID  *idwrap.IDWrap `json:"parent_graphql_id,omitempty"`
	IsDelta          bool           `json:"is_delta"`
	IsSnapshot       bool           `json:"is_snapshot"`
	DeltaName        *string        `json:"delta_name,omitempty"`
	DeltaUrl         *string        `json:"delta_url,omitempty"`
	DeltaQuery       *string        `json:"delta_query,omitempty"`
	DeltaVariables   *string        `json:"delta_variables,omitempty"`
	DeltaDescription *string        `json:"delta_description,omitempty"`
	LastRunAt        *int64         `json:"last_run_at,omitempty"`
	CreatedAt        int64          `json:"created_at"`
	UpdatedAt        int64          `json:"updated_at"`
}

type GraphQLHeader struct {
	ID                     idwrap.IDWrap  `json:"id"`
	GraphQLID              idwrap.IDWrap  `json:"graphql_id"`
	Key                    string         `json:"key"`
	Value                  string         `json:"value"`
	Enabled                bool           `json:"enabled"`
	Description            string         `json:"description"`
	DisplayOrder           float32        `json:"order"`
	ParentGraphQLHeaderID  *idwrap.IDWrap `json:"parent_graphql_header_id,omitempty"`
	IsDelta                bool           `json:"is_delta"`
	DeltaKey               *string        `json:"delta_key,omitempty"`
	DeltaValue             *string        `json:"delta_value,omitempty"`
	DeltaEnabled           *bool          `json:"delta_enabled,omitempty"`
	DeltaDescription       *string        `json:"delta_description,omitempty"`
	DeltaDisplayOrder      *float32       `json:"delta_order,omitempty"`
	CreatedAt              int64          `json:"created_at"`
	UpdatedAt              int64          `json:"updated_at"`
}

type GraphQLAssert struct {
	ID                      idwrap.IDWrap  `json:"id"`
	GraphQLID               idwrap.IDWrap  `json:"graphql_id"`
	Value                   string         `json:"value"`
	Enabled                 bool           `json:"enabled"`
	Description             string         `json:"description"`
	DisplayOrder            float32        `json:"order"`
	ParentGraphQLAssertID   *idwrap.IDWrap `json:"parent_graphql_assert_id,omitempty"`
	IsDelta                 bool           `json:"is_delta"`
	DeltaValue              *string        `json:"delta_value,omitempty"`
	DeltaEnabled            *bool          `json:"delta_enabled,omitempty"`
	DeltaDescription        *string        `json:"delta_description,omitempty"`
	DeltaDisplayOrder       *float32       `json:"delta_order,omitempty"`
	CreatedAt               int64          `json:"created_at"`
	UpdatedAt               int64          `json:"updated_at"`
}

func (a GraphQLAssert) IsEnabled() bool {
	return a.Enabled
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

type GraphQLResponseAssert struct {
	ID         idwrap.IDWrap `json:"id"`
	ResponseID idwrap.IDWrap `json:"response_id"`
	Value      string        `json:"value"`
	Success    bool          `json:"success"`
	CreatedAt  int64         `json:"created_at"`
}

type GraphQLVersion struct {
	ID                 idwrap.IDWrap  `json:"id"`
	GraphQLID          idwrap.IDWrap  `json:"graphql_id"`
	VersionName        string         `json:"version_name"`
	VersionDescription string         `json:"version_description"`
	IsActive           bool           `json:"is_active"`
	CreatedAt          int64          `json:"created_at"`
	CreatedBy          *idwrap.IDWrap `json:"created_by,omitempty"`
}
