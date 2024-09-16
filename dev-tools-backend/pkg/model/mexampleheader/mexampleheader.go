package mexampleheader

import (
	"dev-tools-backend/pkg/idwrap"
)

type Header struct {
	ID          idwrap.IDWrap
	ExampleID   idwrap.IDWrap
	HeaderKey   string
	Enable      bool
	Description string
	Value       string
}
