package mworkspace

import (
	"the-dev-tools/server/pkg/idwrap"
	"time"
)

type Workspace struct {
	FlowCount       int32
	CollectionCount int32
	Updated         time.Time
	Name            string
	ActiveEnv       idwrap.IDWrap
	GlobalEnv       idwrap.IDWrap
	ID              idwrap.IDWrap
}

func (w Workspace) GetCreatedTime() time.Time {
	return w.ID.Time()
}
