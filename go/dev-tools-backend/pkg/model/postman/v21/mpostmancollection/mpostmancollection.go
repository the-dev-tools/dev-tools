package mpostmancollection

import (
	"dev-tools-backend/pkg/model/postman/v21/mauth"
	"dev-tools-backend/pkg/model/postman/v21/mevent"
	"dev-tools-backend/pkg/model/postman/v21/mitem"
	"dev-tools-backend/pkg/model/postman/v21/mvariable"
)

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Schema      string `json:"schema"`
}

// Collection represents a Postman Collection.
type Collection struct {
	Auth      *mauth.Auth           `json:"auth,omitempty"`
	Info      Info                  `json:"info"`
	Items     []mitem.Items         `json:"item"`
	Events    []*mevent.Event       `json:"event,omitempty"`
	Variables []*mvariable.Variable `json:"variable,omitempty"`
}
