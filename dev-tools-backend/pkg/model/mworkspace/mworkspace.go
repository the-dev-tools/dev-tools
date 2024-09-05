package mworkspace

import (
	"time"

	"github.com/oklog/ulid/v2"
)

type Workspace struct {
	ID      ulid.ULID
	Name    string
	Updated time.Time
}

func (w Workspace) GetCreatedTime() time.Time {
	return time.UnixMilli(int64(w.ID.Time()))
}
