package mexampleheader

import "github.com/oklog/ulid/v2"

type Header struct {
	ID           ulid.ULID
	ExampleID    ulid.ULID
	CollectionID ulid.ULID
	HeaderKey    string
	Enable       bool
	Description  string
	Value        string
}
