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
	ID                  idwrap.IDWrap  `json:"id"`
	HttpID              idwrap.IDWrap  `json:"http_id"`
	ParamKey            string         `json:"param_key"`
	ParamValue          string         `json:"param_value"`
	Description         string         `json:"description"`
	Enabled             bool           `json:"enabled"`
	ParentSearchParamID *idwrap.IDWrap `json:"parent_search_param_id,omitempty"`
	IsDelta             bool           `json:"is_delta"`
	DeltaParamKey       *string        `json:"delta_param_key,omitempty"`
	DeltaParamValue     *string        `json:"delta_param_value,omitempty"`
	DeltaDescription    *string        `json:"delta_description,omitempty"`
	DeltaEnabled        *bool          `json:"delta_enabled,omitempty"`
	Prev                *idwrap.IDWrap `json:"prev,omitempty"`
	Next                *idwrap.IDWrap `json:"next,omitempty"`
	CreatedAt           int64          `json:"created_at"`
	UpdatedAt           int64          `json:"updated_at"`
}

type HTTPHeader struct {
	ID                 idwrap.IDWrap  `json:"id"`
	HttpID             idwrap.IDWrap  `json:"http_id"`
	Key                string         `json:"key"`
	Value              string         `json:"value"`
	Enabled            bool           `json:"enabled"`
	Description        string         `json:"description"`
	Order              float32        `json:"order"`
	ParentHttpHeaderID *idwrap.IDWrap `json:"parent_http_header_id,omitempty"`
	IsDelta            bool           `json:"is_delta"`
	DeltaKey           *string        `json:"delta_key,omitempty"`
	DeltaValue         *string        `json:"delta_value,omitempty"`
	DeltaEnabled       *bool          `json:"delta_enabled,omitempty"`
	DeltaDescription   *string        `json:"delta_description,omitempty"`
	DeltaOrder         *float32       `json:"delta_order,omitempty"`
	CreatedAt          int64          `json:"created_at"`
	UpdatedAt          int64          `json:"updated_at"`
}

func (h HTTPHeader) IsEnabled() bool {
	return h.Enabled
}

type HTTPBodyForm struct {
	ID               idwrap.IDWrap  `json:"id"`
	HttpID           idwrap.IDWrap  `json:"http_id"`
	FormKey          string         `json:"form_key"`
	FormValue        string         `json:"form_value"`
	Description      string         `json:"description"`
	Enabled          bool           `json:"enabled"`
	ParentBodyFormID *idwrap.IDWrap `json:"parent_body_form_id,omitempty"`
	IsDelta          bool           `json:"is_delta"`
	DeltaFormKey     *string        `json:"delta_form_key,omitempty"`
	DeltaFormValue   *string        `json:"delta_form_value,omitempty"`
	DeltaDescription *string        `json:"delta_description,omitempty"`
	DeltaEnabled     *bool          `json:"delta_enabled,omitempty"`
	Prev             *idwrap.IDWrap `json:"prev,omitempty"`
	Next             *idwrap.IDWrap `json:"next,omitempty"`
	CreatedAt        int64          `json:"created_at"`
	UpdatedAt        int64          `json:"updated_at"`
}

type HTTPBodyUrlencoded struct {
	ID                     idwrap.IDWrap  `json:"id"`
	HttpID                 idwrap.IDWrap  `json:"http_id"`
	UrlencodedKey          string         `json:"urlencoded_key"`
	UrlencodedValue        string         `json:"urlencoded_value"`
	Description            string         `json:"description"`
	Enabled                bool           `json:"enabled"`
	ParentBodyUrlencodedID *idwrap.IDWrap `json:"parent_body_urlencoded_id,omitempty"`
	IsDelta                bool           `json:"is_delta"`
	DeltaUrlencodedKey     *string        `json:"delta_urlencoded_key,omitempty"`
	DeltaUrlencodedValue   *string        `json:"delta_urlencoded_value,omitempty"`
	DeltaDescription       *string        `json:"delta_description,omitempty"`
	DeltaEnabled           *bool          `json:"delta_enabled,omitempty"`
	Prev                   *idwrap.IDWrap `json:"prev,omitempty"`
	Next                   *idwrap.IDWrap `json:"next,omitempty"`
	CreatedAt              int64          `json:"created_at"`
	UpdatedAt              int64          `json:"updated_at"`
}

type HTTPBodyRaw struct {
	ID                   idwrap.IDWrap  `json:"id"`
	HttpID               idwrap.IDWrap  `json:"http_id"`
	RawData              []byte         `json:"raw_data"`
	ContentType          string         `json:"content_type"`
	CompressionType      int8           `json:"compression_type"`
	ParentBodyRawID      *idwrap.IDWrap `json:"parent_body_raw_id,omitempty"`
	IsDelta              bool           `json:"is_delta"`
	DeltaRawData         []byte         `json:"delta_raw_data,omitempty"`
	DeltaContentType     interface{}    `json:"delta_content_type,omitempty"`
	DeltaCompressionType interface{}    `json:"delta_compression_type,omitempty"`
	CreatedAt            int64          `json:"created_at"`
	UpdatedAt            int64          `json:"updated_at"`
}

type HTTPAssert struct {
	ID               idwrap.IDWrap  `json:"id"`
	HttpID           idwrap.IDWrap  `json:"http_id"`
	AssertKey        string         `json:"assert_key"`
	AssertValue      string         `json:"assert_value"`
	Description      string         `json:"description"`
	Enabled          bool           `json:"enabled"`
	ParentAssertID   *idwrap.IDWrap `json:"parent_assert_id,omitempty"`
	IsDelta          bool           `json:"is_delta"`
	DeltaAssertKey   *string        `json:"delta_assert_key,omitempty"`
	DeltaAssertValue *string        `json:"delta_assert_value,omitempty"`
	DeltaDescription *string        `json:"delta_description,omitempty"`
	DeltaEnabled     *bool          `json:"delta_enabled,omitempty"`
	Prev             *idwrap.IDWrap `json:"prev,omitempty"`
	Next             *idwrap.IDWrap `json:"next,omitempty"`
	CreatedAt        int64          `json:"created_at"`
	UpdatedAt        int64          `json:"updated_at"`
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
