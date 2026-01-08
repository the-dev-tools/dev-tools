//nolint:revive // exported
package mflow

import "github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"

type Flow struct {
	ID              idwrap.IDWrap
	WorkspaceID     idwrap.IDWrap
	VersionParentID *idwrap.IDWrap
	Name            string
	Duration        int32
	Running         bool
	NodeIDMapping   []byte // JSON map of parent node ID -> version node ID
}
