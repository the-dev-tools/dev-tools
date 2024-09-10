package mbodyform

import "github.com/oklog/ulid/v2"

type BodyForm struct {
	ID          ulid.ULID `json:"id"`
	ExampleID   ulid.ULID `json:"example_id"`
	BodyKey     string    `json:"body_key"`
	Enable      bool      `json:"enable"`
	Description string    `json:"description"`
	Value       string    `json:"value"`
}
