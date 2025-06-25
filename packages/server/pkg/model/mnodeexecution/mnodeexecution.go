package mnodeexecution

import "the-dev-tools/server/pkg/idwrap"

type NodeExecution struct {
	ID               idwrap.IDWrap
	NodeID           idwrap.IDWrap
	FlowRunID        idwrap.IDWrap
	State            int8
	Data             []byte
	DataCompressType int8
	Error            *string
}