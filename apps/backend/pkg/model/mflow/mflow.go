package mflow

import "the-dev-tools/backend/pkg/idwrap"

type Flow struct {
	ID              idwrap.IDWrap
	WorkspaceID     idwrap.IDWrap
	ParentVersionID *idwrap.IDWrap
	Name            string
}
