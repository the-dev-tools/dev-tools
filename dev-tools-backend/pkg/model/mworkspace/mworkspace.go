package mworkspace

import (
	"dev-tools-backend/pkg/idwrap"
	"time"
)

type Workspace struct {
	ID      idwrap.IDWrap
	Name    string
	Updated time.Time
}

func (w Workspace) GetCreatedTime() time.Time {
	return w.ID.Time()
}
