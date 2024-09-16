package mexamplebodyform

import "dev-tools-backend/pkg/idwrap"

type BodyForm struct {
	ID          idwrap.IDWrap
	ExampleID   idwrap.IDWrap
	BodyKey     string
	Enable      bool
	Description string
	Value       string
}
