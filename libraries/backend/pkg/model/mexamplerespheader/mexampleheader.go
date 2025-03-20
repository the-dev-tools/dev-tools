package mexamplerespheader

import "the-dev-tools/backend/pkg/idwrap"

type ExampleRespHeader struct {
	ID            idwrap.IDWrap
	ExampleRespID idwrap.IDWrap
	HeaderKey     string
	Value         string
}
