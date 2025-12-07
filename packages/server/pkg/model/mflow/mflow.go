//nolint:revive // exported
package mflow

import "the-dev-tools/server/pkg/idwrap"

type Flow struct {
	ID              idwrap.IDWrap
	WorkspaceID     idwrap.IDWrap
	VersionParentID *idwrap.IDWrap
	Name            string
	Duration        int32
	Running         bool
}