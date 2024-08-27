package mworkspaceuser

import "github.com/oklog/ulid/v2"

type Role uint16

const (
	RoleUnknown Role = 0
	RoleUser    Role = 1
	RoleAdmin   Role = 2
	RoleOwner   Role = 3
)

type WorkspaceUser struct {
	ID          ulid.ULID
	WorkspaceID ulid.ULID
	UserID      ulid.ULID
	Role        Role
}
