//nolint:revive // exported
package mresponse

import (
	"the-dev-tools/server/pkg/model/postman/v21/mcookie"
	"the-dev-tools/server/pkg/model/postman/v21/mheader"
	"the-dev-tools/server/pkg/model/postman/v21/mrequest"
)

type Response struct {
	ID              string            `json:"id,omitempty"`
	OriginalRequest *mrequest.Request `json:"originalRequest,omitempty"`
	ResponseTime    interface{}       `json:"responseTime,omitempty"`
	Timings         interface{}       `json:"timings,omitempty"`
	Headers         []mheader.Header  `json:"header,omitempty"`
	Cookies         []*mcookie.Cookie `json:"cookie,omitempty"`
	Body            string            `json:"body,omitempty"`
	Status          string            `json:"status,omitempty"`
	Code            int               `json:"code,omitempty"`
	Name            string            `json:"name,omitempty"`
	PreviewLanguage string            `json:"_postman_previewlanguage,omitempty"`
}
