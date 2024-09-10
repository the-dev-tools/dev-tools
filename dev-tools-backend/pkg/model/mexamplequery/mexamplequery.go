package mexamplequery

import "github.com/oklog/ulid/v2"

type Query struct {
	id          ulid.ULID
	example_id  ulid.ULID
	query_key   string
	enable      bool
	description string
	value       string
}
