package mexamplequery

import "github.com/oklog/ulid/v2"

type Query struct {
	ID          ulid.ULID
	ExampleID   ulid.ULID
	QueryKey    string
	Enable      bool
	Description string
	Value       string
}
