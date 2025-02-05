package mnnoop

import "the-dev-tools/backend/pkg/idwrap"

type NoopTypes int16

const (
	NODE_NO_OP_KIND_UNSPECIFIED NoopTypes = iota
	NODE_NO_OP_KIND_START
	NODE_NO_OP_KIND_CREATE
	NODE_NO_OP_KIND_THEN
	NODE_NO_OP_KIND_ELSE
	NODE_NO_OP_KIND_LOOP
)

type NoopNode struct {
	FlowNodeID idwrap.IDWrap
	Type       NoopTypes
}
