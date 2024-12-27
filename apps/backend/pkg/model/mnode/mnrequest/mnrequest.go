package mnrequest

import "the-dev-tools/backend/pkg/idwrap"

type MNRequest struct {
	FlowNodeID     idwrap.IDWrap
	Name           string
	DeltaExampleID *idwrap.IDWrap
	EndpointID     *idwrap.IDWrap
	ExampleID      *idwrap.IDWrap
}
