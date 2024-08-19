package mworkspaceuser

import "github.com/oklog/ulid/v2"

type WorkspaceUser struct {
	ID     ulid.ULID
	OrgID  ulid.ULID
	UserID ulid.ULID
}
