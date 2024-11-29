package mbodyform

import "dev-tools-backend/pkg/idwrap"

type BodyForm struct {
	BodyKey     string        `json:"body_key"`
	Description string        `json:"description"`
	Value       string        `json:"value"`
	Enable      bool          `json:"enable"`
	ID          idwrap.IDWrap `json:"id"`
	ExampleID   idwrap.IDWrap `json:"example_id"`
}
