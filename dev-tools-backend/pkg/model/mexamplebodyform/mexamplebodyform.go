package mexamplebodyform

import "dev-tools-backend/pkg/ulidwrap"

type BodyForm struct {
	ID          ulidwrap.ULIDWrap
	ExampleID   ulidwrap.ULIDWrap
	BodyKey     string
	Enable      bool
	Description string
	Value       string
}
