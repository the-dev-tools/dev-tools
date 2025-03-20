package mexamplequery

import (
	"the-dev-tools/backend/pkg/idwrap"
)

type Query struct {
	QueryKey      string
	Description   string
	Value         string
	Enable        bool
	DeltaParentID *idwrap.IDWrap
	ID            idwrap.IDWrap
	ExampleID     idwrap.IDWrap
}
