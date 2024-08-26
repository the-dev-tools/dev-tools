package mworkspaceuser

import "github.com/oklog/ulid/v2"

type Role int32

const (
	RoleOwner Role = 1
	RoleAdmin Role = 2
	RoleUser  Role = 3
)

type WorkspaceUser struct {
	ID          ulid.ULID
	WorkspaceID ulid.ULID
	UserID      ulid.ULID
	Role        Role
}
