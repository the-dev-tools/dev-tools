//nolint:revive // exported
package mflow

import (
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcondition"
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

// --- NoOp Node ---

type NoopTypes int16

const (
	NODE_NO_OP_KIND_UNSPECIFIED NoopTypes = iota
	NODE_NO_OP_KIND_START
	NODE_NO_OP_KIND_CREATE
	NODE_NO_OP_KIND_THEN
	NODE_NO_OP_KIND_ELSE
	NODE_NO_OP_KIND_LOOP
)

type NodeNoop struct {
	FlowNodeID idwrap.IDWrap
	Type       NoopTypes
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
