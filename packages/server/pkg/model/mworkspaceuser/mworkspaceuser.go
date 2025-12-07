//nolint:revive // exported
package mworkspaceuser

import "the-dev-tools/server/pkg/idwrap"

type Role uint16

const (
	RoleUnknown Role = 0
	RoleUser    Role = 1
	RoleAdmin   Role = 2
	RoleOwner   Role = 3
)

type WorkspaceUser struct {
	ID          idwrap.IDWrap
	WorkspaceID idwrap.IDWrap
	UserID      idwrap.IDWrap
	Role        Role
}
