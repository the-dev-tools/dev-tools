//nolint:revive // exported
package mevent

import "the-dev-tools/server/pkg/model/postman/v21/murl"

type Script struct {
	ID   string   `json:"id,omitempty"`
	Type string   `json:"type,omitempty"`
	Name string   `json:"name,omitempty"`
	Src  murl.URL `json:"src,omitempty"`
	Exec []string `json:"exec,omitempty"`
}

// Not sure we needed but still added just in case
type Event struct {
	ID       string  `json:"id,omitempty"`
	Script   *Script `json:"script,omitempty"`
	Listen   string  `json:"listen,omitempty"`
	Disabled bool    `json:"disabled,omitempty"`
}
