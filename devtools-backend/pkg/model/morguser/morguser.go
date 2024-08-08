package morguser

import "github.com/oklog/ulid/v2"

type OrgUser struct {
	ID     ulid.ULID
	OrgID  ulid.ULID
	UserID ulid.ULID
}
