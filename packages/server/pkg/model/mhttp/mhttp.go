//nolint:revive // exported
package mhttp

import (
	"the-dev-tools/server/pkg/idwrap"
)

type HTTP struct {
	ID               idwrap.IDWrap  `json:"id"`
	WorkspaceID      idwrap.IDWrap  `json:"workspace_id"`
	FolderID         *idwrap.IDWrap `json:"folder_id,omitempty"`
	Name             string         `json:"name"`
	Url              string         `json:"url"`
	Method           string         `json:"method"`
	Description      string         `json:"description"`
	BodyKind         HttpBodyKind   `json:"body_kind"`
	ParentHttpID     *idwrap.IDWrap `json:"parent_http_id,omitempty"`
	IsDelta          bool           `json:"is_delta"`
	DeltaName        *string        `json:"delta_name,omitempty"`
	DeltaUrl         *string        `json:"delta_url,omitempty"`
	DeltaMethod      *string        `json:"delta_method,omitempty"`
	DeltaDescription *string        `json:"delta_description,omitempty"`
	DeltaBodyKind    *HttpBodyKind  `json:"delta_body_kind,omitempty"`
	LastRunAt        *int64         `json:"last_run_at,omitempty"`
	CreatedAt        int64          `json:"created_at"`
	UpdatedAt        int64          `json:"updated_at"`
}

type HttpBodyKind int8

const (
	HttpBodyKindNone       HttpBodyKind = 0
	HttpBodyKindFormData   HttpBodyKind = 1
	HttpBodyKindUrlEncoded HttpBodyKind = 2
	HttpBodyKindRaw        HttpBodyKind = 3
)

type HTTPSearchParam struct {
	ID                      idwrap.IDWrap  `json:"id"`
	HttpID                  idwrap.IDWrap  `json:"http_id"`
	Key                     string         `json:"key"`
	Value                   string         `json:"value"`
	Description             string         `json:"description"`
	Enabled                 bool           `json:"enabled"`
	DisplayOrder            float64        `json:"order"`
	ParentHttpSearchParamID *idwrap.IDWrap `json:"parent_http_search_param_id,omitempty"`
	IsDelta                 bool           `json:"is_delta"`
	DeltaKey                *string        `json:"delta_key,omitempty"`
	DeltaValue              *string        `json:"delta_value,omitempty"`
	DeltaDescription        *string        `json:"delta_description,omitempty"`
	DeltaEnabled            *bool          `json:"delta_enabled,omitempty"`
	DeltaDisplayOrder       *float64       `json:"delta_order,omitempty"`
	CreatedAt               int64          `json:"created_at"`
	UpdatedAt               int64          `json:"updated_at"`
}

func (p HTTPSearchParam) IsEnabled() bool {
	return p.Enabled
}

type HTTPHeader struct {
	ID                 idwrap.IDWrap  `json:"id"`
	HttpID             idwrap.IDWrap  `json:"http_id"`
	Key                string         `json:"key"`
	Value              string         `json:"value"`
	Enabled            bool           `json:"enabled"`
	Description        string         `json:"description"`
	DisplayOrder       float32        `json:"order"`
	ParentHttpHeaderID *idwrap.IDWrap `json:"parent_http_header_id,omitempty"`
	IsDelta            bool           `json:"is_delta"`
	DeltaKey           *string        `json:"delta_key,omitempty"`
	DeltaValue         *string        `json:"delta_value,omitempty"`
	DeltaEnabled       *bool          `json:"delta_enabled,omitempty"`
	DeltaDescription   *string        `json:"delta_description,omitempty"`
	DeltaDisplayOrder  *float32       `json:"delta_order,omitempty"`
	CreatedAt          int64          `json:"created_at"`
	UpdatedAt          int64          `json:"updated_at"`
}

func (h HTTPHeader) IsEnabled() bool {
	return h.Enabled
}

type HTTPBodyForm struct {
	ID                   idwrap.IDWrap  `json:"id"`
	HttpID               idwrap.IDWrap  `json:"http_id"`
	Key                  string         `json:"key"`
	Value                string         `json:"value"`
	Description          string         `json:"description"`
	Enabled              bool           `json:"enabled"`
	DisplayOrder         float32        `json:"order"`
	ParentHttpBodyFormID *idwrap.IDWrap `json:"parent_http_body_form_id,omitempty"`
	IsDelta              bool           `json:"is_delta"`
	DeltaKey             *string        `json:"delta_key,omitempty"`
	DeltaValue           *string        `json:"delta_value,omitempty"`
	DeltaDescription     *string        `json:"delta_description,omitempty"`
	DeltaEnabled         *bool          `json:"delta_enabled,omitempty"`
	DeltaDisplayOrder    *float32       `json:"delta_order,omitempty"`
	CreatedAt            int64          `json:"created_at"`
	UpdatedAt            int64          `json:"updated_at"`
}

func (f HTTPBodyForm) IsEnabled() bool {
	return f.Enabled
}

type HTTPBodyUrlencoded struct {
	ID                         idwrap.IDWrap  `json:"id"`
	HttpID                     idwrap.IDWrap  `json:"http_id"`
	Key                        string         `json:"key"`
	Value                      string         `json:"value"`
	Enabled                    bool           `json:"enabled"`
	Description                string         `json:"description"`
	DisplayOrder               float32        `json:"order"`
	ParentHttpBodyUrlEncodedID *idwrap.IDWrap `json:"parent_http_body_url_encoded_id,omitempty"`
	IsDelta                    bool           `json:"is_delta"`
	DeltaKey                   *string        `json:"delta_key,omitempty"`
	DeltaValue                 *string        `json:"delta_value,omitempty"`
	DeltaEnabled               *bool          `json:"delta_enabled,omitempty"`
	DeltaDescription           *string        `json:"delta_description,omitempty"`
	DeltaDisplayOrder          *float32       `json:"delta_order,omitempty"`
	CreatedAt                  int64          `json:"created_at"`
	UpdatedAt                  int64          `json:"updated_at"`
}

func (u HTTPBodyUrlencoded) IsEnabled() bool {
	return u.Enabled
}

type HTTPBodyRaw struct {
	ID                   idwrap.IDWrap  `json:"id"`
	HttpID               idwrap.IDWrap  `json:"http_id"`
	RawData              []byte         `json:"raw_data"`
	CompressionType      int8           `json:"compression_type"`
	ParentBodyRawID      *idwrap.IDWrap `json:"parent_body_raw_id,omitempty"`
	IsDelta              bool           `json:"is_delta"`
	DeltaRawData         []byte         `json:"delta_raw_data,omitempty"`
	DeltaCompressionType interface{}    `json:"delta_compression_type,omitempty"`
	CreatedAt            int64          `json:"created_at"`
	UpdatedAt            int64          `json:"updated_at"`
}

type HTTPAssert struct {
	ID                 idwrap.IDWrap  `json:"id"`
	HttpID             idwrap.IDWrap  `json:"http_id"`
	Value              string         `json:"value"`
	Enabled            bool           `json:"enabled"`
	Description        string         `json:"description"`
	DisplayOrder       float32        `json:"order"`
	ParentHttpAssertID *idwrap.IDWrap `json:"parent_http_assert_id,omitempty"`
	IsDelta            bool           `json:"is_delta"`
	DeltaValue         *string        `json:"delta_value,omitempty"`
	DeltaEnabled       *bool          `json:"delta_enabled,omitempty"`
	DeltaDescription   *string        `json:"delta_description,omitempty"`
	DeltaDisplayOrder  *float32       `json:"delta_order,omitempty"`
	CreatedAt          int64          `json:"created_at"`
	UpdatedAt          int64          `json:"updated_at"`
}

func (a HTTPAssert) IsEnabled() bool {
	return a.Enabled
}

type HTTPResponse struct {
	ID        idwrap.IDWrap `json:"id"`
	HttpID    idwrap.IDWrap `json:"http_id"`
	Status    int32         `json:"status"`
	Body      []byte        `json:"body"`
	Time      int64         `json:"time"`
	Duration  int32         `json:"duration"`
	Size      int32         `json:"size"`
	CreatedAt int64         `json:"created_at"`
}

type HTTPResponseHeader struct {
	ID          idwrap.IDWrap `json:"id"`
	ResponseID  idwrap.IDWrap `json:"response_id"`
	HeaderKey   string        `json:"header_key"`
	HeaderValue string        `json:"header_value"`
	CreatedAt   int64         `json:"created_at"`
}

type HTTPResponseAssert struct {
	ID         idwrap.IDWrap `json:"id"`
	ResponseID idwrap.IDWrap `json:"response_id"`
	Value      string        `json:"value"`
	Success    bool          `json:"success"`
	CreatedAt  int64         `json:"created_at"`
}

type HttpVersion struct {
	ID                 idwrap.IDWrap  `json:"id"`
	HttpID             idwrap.IDWrap  `json:"http_id"`
	VersionName        string         `json:"version_name"`
	VersionDescription string         `json:"version_description"`
	IsActive           bool           `json:"is_active"`
	CreatedAt          int64          `json:"created_at"`
	CreatedBy          *idwrap.IDWrap `json:"created_by,omitempty"`
}
