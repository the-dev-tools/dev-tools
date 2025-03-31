package mexamplerespheader

import "the-dev-tools/server/pkg/idwrap"

type ExampleRespHeader struct {
	ID            idwrap.IDWrap
	ExampleRespID idwrap.IDWrap
	HeaderKey     string
	Value         string
}
