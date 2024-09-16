package mexamplequery

import (
	"dev-tools-backend/pkg/idwrap"
)

type Query struct {
	ID          idwrap.IDWrap
	ExampleID   idwrap.IDWrap
	QueryKey    string
	Enable      bool
	Description string
	Value       string
}
