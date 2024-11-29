package mexamplequery

import (
	"the-dev-tools/backend/pkg/idwrap"
)

type Query struct {
	QueryKey    string
	Description string
	Value       string
	Enable      bool
	ID          idwrap.IDWrap
	ExampleID   idwrap.IDWrap
}
