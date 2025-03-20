package mnjs

import (
	"the-dev-tools/backend/pkg/compress"
	"the-dev-tools/backend/pkg/idwrap"
)

type MNJS struct {
	FlowNodeID       idwrap.IDWrap
	Code             []byte
	CodeCompressType compress.CompressType
}
