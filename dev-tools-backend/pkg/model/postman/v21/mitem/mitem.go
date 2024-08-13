package mitem

import (
	"dev-tools-backend/pkg/model/postman/v21/mauth"
	"dev-tools-backend/pkg/model/postman/v21/mevent"
	"dev-tools-backend/pkg/model/postman/v21/mrequest"
	"dev-tools-backend/pkg/model/postman/v21/mresponse"
	"dev-tools-backend/pkg/model/postman/v21/mvariable"
)

// Can be generic for single or group items
type Items struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Variables can be generic for single or group items | []*mvariable.Variable
	Variables               interface{}           `json:"variable,omitempty"`
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
