package mrequest

import (
	"the-dev-tools/server/pkg/model/postman/v21/mauth"
	"the-dev-tools/server/pkg/model/postman/v21/mbody"
	"the-dev-tools/server/pkg/model/postman/v21/mheader"
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
