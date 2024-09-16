package mbodyform

import "dev-tools-backend/pkg/idwrap"

type BodyForm struct {
	ID          idwrap.IDWrap `json:"id"`
	ExampleID   idwrap.IDWrap `json:"example_id"`
	BodyKey     string        `json:"body_key"`
	Enable      bool          `json:"enable"`
	Description string        `json:"description"`
	Value       string        `json:"value"`
}
