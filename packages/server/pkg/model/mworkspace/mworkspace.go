//nolint:revive // exported
package mworkspace

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
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
	Order           float64
	// Folder sync fields
	SyncPath    *string // nil = no sync, else absolute path to folder
	SyncFormat  *string // "open_yaml" or nil
	SyncEnabled bool    // Whether sync is currently active
}

func (w Workspace) GetCreatedTime() time.Time {
	return w.ID.Time()
}
