package mitem

import (
	"devtools-backend/pkg/model/postman/v21/mauth"
	"devtools-backend/pkg/model/postman/v21/mevent"
	"devtools-backend/pkg/model/postman/v21/mrequest"
	"devtools-backend/pkg/model/postman/v21/mresponse"
	"devtools-backend/pkg/model/postman/v21/mvariable"
)

// Can be generic for single or group items
type Items struct {
	ID                      string                `json:"id,omitempty"`
	Name                    string                `json:"name"`
	Description             string                `json:"description,omitempty"`
	Variables               []*mvariable.Variable `json:"variable,omitempty"`
	Events                  []*mevent.Event       `json:"event,omitempty"`
	ProtocolProfileBehavior interface{}           `json:"protocolProfileBehavior,omitempty"`
	Request                 *mrequest.Request     `json:"request,omitempty"`
	Responses               []*mresponse.Response `json:"response,omitempty"`
	Items                   []*Items              `json:"item"`
	Auth                    *mauth.Auth           `json:"auth,omitempty"`
}

type Item struct {
	Name                    string                `json:"name"`
	Description             string                `json:"description,omitempty"`
	Variables               []*mvariable.Variable `json:"variable,omitempty"`
	Events                  []*mevent.Event       `json:"event,omitempty"`
	ProtocolProfileBehavior interface{}           `json:"protocolProfileBehavior,omitempty"`
	ID                      string                `json:"id,omitempty"`
	Request                 *mrequest.Request     `json:"request,omitempty"`
	Responses               []*mresponse.Response `json:"response,omitempty"`
}

type ItemGroup struct {
	Name                    string                `json:"name"`
	Description             string                `json:"description,omitempty"`
	Variables               []*mvariable.Variable `json:"variable,omitempty"`
	Events                  []*mevent.Event       `json:"event,omitempty"`
	ProtocolProfileBehavior interface{}           `json:"protocolProfileBehavior,omitempty"`
	Items                   []*Items              `json:"item"`
	Auth                    *mauth.Auth           `json:"auth,omitempty"`
}
