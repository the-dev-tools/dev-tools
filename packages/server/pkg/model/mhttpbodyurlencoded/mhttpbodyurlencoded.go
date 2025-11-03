package mhttpbodyurlencoded

import (
	"the-dev-tools/server/pkg/idwrap"
)

type HttpBodyUrlEncoded struct {
	ID                         idwrap.IDWrap  `json:"id"`
	HttpID                     idwrap.IDWrap  `json:"http_id"`
	Key                        string         `json:"key"`
	Value                      string         `json:"value"`
	Enabled                    bool           `json:"enabled"`
	Description                string         `json:"description"`
	Order                      float32        `json:"order"`
	ParentHttpBodyUrlEncodedID *idwrap.IDWrap `json:"parent_http_body_url_encoded_id,omitempty"`
	IsDelta                    bool           `json:"is_delta"`
	DeltaKey                   *string        `json:"delta_key,omitempty"`
	DeltaValue                 *string        `json:"delta_value,omitempty"`
	DeltaEnabled               *bool          `json:"delta_enabled,omitempty"`
	DeltaDescription           *string        `json:"delta_description,omitempty"`
	DeltaOrder                 *float32       `json:"delta_order,omitempty"`
	CreatedAt                  int64          `json:"created_at"`
	UpdatedAt                  int64          `json:"updated_at"`
}
