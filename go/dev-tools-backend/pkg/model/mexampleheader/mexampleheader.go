package mexampleheader

import (
	"dev-tools-backend/pkg/idwrap"
)

type Header struct {
	HeaderKey   string
	Description string
	Value       string
	Enable      bool
	ID          idwrap.IDWrap
	ExampleID   idwrap.IDWrap
}
