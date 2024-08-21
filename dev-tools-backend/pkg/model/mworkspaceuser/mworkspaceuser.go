package mworkspaceuser

import "github.com/oklog/ulid/v2"

type WorkspaceUser struct {
	ID          ulid.ULID
	WorkspaceID ulid.ULID
	UserID      ulid.ULID
}
