package mworkspace

import (
	"the-dev-tools/backend/pkg/idwrap"
	"time"
)

type Workspace struct {
	Updated time.Time
	Name    string
	ID      idwrap.IDWrap
}

func (w Workspace) GetCreatedTime() time.Time {
	return w.ID.Time()
}
