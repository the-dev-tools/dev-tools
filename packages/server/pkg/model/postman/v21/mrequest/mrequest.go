//nolint:revive // exported
package mrequest

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/postman/v21/mauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/postman/v21/mbody"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/postman/v21/mheader"
)

type Request struct {
	// URL can be string or *murl.URL
	URL         interface{}      `json:"url"`
	Auth        *mauth.Auth      `json:"auth,omitempty"`
	Proxy       interface{}      `json:"proxy,omitempty"`
	Certificate interface{}      `json:"certificate,omitempty"`
	Method      string           `json:"method"`
	Description string           `json:"description,omitempty"`
	Header      []mheader.Header `json:"header,omitempty"`
	Body        *mbody.Body      `json:"body,omitempty"`
}
