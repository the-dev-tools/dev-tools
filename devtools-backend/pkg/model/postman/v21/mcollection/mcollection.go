package mcollection

import (
	"github.com/DevToolsGit/devtools-backend/pkg/model/postman/v21/mauth"
	"github.com/DevToolsGit/devtools-backend/pkg/model/postman/v21/mevent"
	"github.com/DevToolsGit/devtools-backend/pkg/model/postman/v21/mitem"
	"github.com/DevToolsGit/devtools-backend/pkg/model/postman/v21/mvariable"
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
	Items     []*mitem.Items        `json:"item"`
	Events    []*mevent.Event       `json:"event,omitempty"`
	Variables []*mvariable.Variable `json:"variable,omitempty"`
}
