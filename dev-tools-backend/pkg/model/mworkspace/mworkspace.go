package mworkspace

import "github.com/oklog/ulid/v2"

type Workspace struct {
	ID   ulid.ULID
	Name string
}
