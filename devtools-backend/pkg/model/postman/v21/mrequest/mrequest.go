package mrequest

import (
	"devtools-backend/pkg/model/postman/v21/mauth"
	"devtools-backend/pkg/model/postman/v21/mbody"
	"devtools-backend/pkg/model/postman/v21/mheader"
	"devtools-backend/pkg/model/postman/v21/murl"
)

type Request struct {
	URL         *murl.URL        `json:"url"`
	Auth        *mauth.Auth      `json:"auth,omitempty"`
	Proxy       interface{}      `json:"proxy,omitempty"`
	Certificate interface{}      `json:"certificate,omitempty"`
	Method      string           `json:"method"`
	Description string           `json:"description,omitempty"`
	Header      []mheader.Header `json:"header,omitempty"`
	Body        *mbody.Body      `json:"body,omitempty"`
}
