package mflow

import "the-dev-tools/server/pkg/idwrap"

type Flow struct {
	ID              idwrap.IDWrap
	WorkspaceID     idwrap.IDWrap
	VersionParentID *idwrap.IDWrap
	Name            string
}
