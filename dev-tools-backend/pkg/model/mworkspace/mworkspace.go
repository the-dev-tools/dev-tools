package mworkspace

import (
	"time"

	"github.com/oklog/ulid/v2"
)

type Workspace struct {
	ID      ulid.ULID
	Name    string
	Created time.Time
	Updated time.Time
}
