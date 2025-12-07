//nolint:revive // exported
package mbodyraw

import (
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
)

type VisualizeMode int8

const (
	VisualizeModeUndefined  VisualizeMode = 0
	VisualizeModeJSON       VisualizeMode = 1
	VisualizeModeHTML       VisualizeMode = 2
	VisualizeModeXML        VisualizeMode = 3
	VisualizeModeText       VisualizeMode = 4
	VisualizeModeJavascript VisualizeMode = 5
	VisualizeModeBinary     VisualizeMode = 6
)

type ExampleBodyRaw struct {
	Data          []byte
	VisualizeMode VisualizeMode
	CompressType  compress.CompressType
	ID            idwrap.IDWrap
	ExampleID     idwrap.IDWrap
}
