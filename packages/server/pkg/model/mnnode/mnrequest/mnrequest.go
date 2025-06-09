package mnrequest

import "the-dev-tools/server/pkg/idwrap"

type MNRequest struct {
	FlowNodeID      idwrap.IDWrap
	DeltaExampleID  *idwrap.IDWrap
	DeltaEndpointID *idwrap.IDWrap
	EndpointID      *idwrap.IDWrap
	ExampleID       *idwrap.IDWrap
}
