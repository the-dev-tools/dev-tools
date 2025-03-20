package mnrequest

import "the-dev-tools/backend/pkg/idwrap"

type MNRequest struct {
	FlowNodeID     idwrap.IDWrap
	DeltaExampleID *idwrap.IDWrap
	EndpointID     *idwrap.IDWrap
	ExampleID      *idwrap.IDWrap
}
