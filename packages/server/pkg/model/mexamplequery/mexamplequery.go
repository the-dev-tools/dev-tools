package mexamplequery

import (
	"the-dev-tools/server/pkg/idwrap"
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
