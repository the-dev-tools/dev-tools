package morg

import "github.com/oklog/ulid/v2"

type Org struct {
	ID   ulid.ULID
	Name string
}
