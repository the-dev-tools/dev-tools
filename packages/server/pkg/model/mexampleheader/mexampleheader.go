package mexampleheader

import (
	"the-dev-tools/server/pkg/idwrap"
)

type Header struct {
	HeaderKey     string
	Description   string
	Value         string
	Enable        bool
	ID            idwrap.IDWrap
	DeltaParentID *idwrap.IDWrap
	ExampleID     idwrap.IDWrap
}

func (h Header) IsEnabled() bool {
	return h.Enable
}
