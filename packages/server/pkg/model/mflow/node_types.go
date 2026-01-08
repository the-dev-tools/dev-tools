//nolint:revive // exported
package mflow

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
)

// --- Request Node ---

type NodeRequest struct {
	FlowNodeID idwrap.IDWrap
	HttpID     *idwrap.IDWrap

	DeltaHttpID *idwrap.IDWrap

	HasRequestConfig bool
}

// --- JS Node ---

type NodeJS struct {
	FlowNodeID       idwrap.IDWrap
	Code             []byte
	CodeCompressType compress.CompressType
}

// --- ManualStart Node ---

type NodeManualStart struct {
	FlowNodeID idwrap.IDWrap
}

// --- If/Condition Node ---
type NodeIf struct {
	FlowNodeID idwrap.IDWrap
	Condition  mcondition.Condition
	// TODO: Condition type
}

// --- For/ForEach Node ---

type ErrorHandling int8

const (
	ErrorHandling_ERROR_HANDLING_UNSPECIFIED ErrorHandling = 0
	ErrorHandling_ERROR_HANDLING_IGNORE      ErrorHandling = 1
	ErrorHandling_ERROR_HANDLING_BREAK       ErrorHandling = 2
)

type NodeFor struct {
	FlowNodeID    idwrap.IDWrap
	IterCount     int64
	Condition     mcondition.Condition
	ErrorHandling ErrorHandling
}

type NodeForEach struct {
	FlowNodeID     idwrap.IDWrap
	IterExpression string
	Condition      mcondition.Condition
	ErrorHandling  ErrorHandling
}
