//nolint:revive // exported
package mnjs

import (
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
)

type MNJS struct {
	FlowNodeID       idwrap.IDWrap
	Code             []byte
	CodeCompressType compress.CompressType
}
