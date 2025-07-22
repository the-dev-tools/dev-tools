package mnodeexecution

import "the-dev-tools/server/pkg/idwrap"

type NodeExecution struct {
	ID                     idwrap.IDWrap
	NodeID                 idwrap.IDWrap
	State                  int8
	InputData              []byte
	InputDataCompressType  int8
	OutputData             []byte
	OutputDataCompressType int8
	Error                  *string
}