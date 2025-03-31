package mbodyraw

import "the-dev-tools/server/pkg/idwrap"

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

type CompressType int8

const (
	CompressTypeNone CompressType = 0
	CompressTypeGzip CompressType = 1
	CompressTypeZstd CompressType = 2
)

type ExampleBodyRaw struct {
	Data          []byte
	VisualizeMode VisualizeMode
	CompressType  CompressType
	ID            idwrap.IDWrap
	ExampleID     idwrap.IDWrap
}
