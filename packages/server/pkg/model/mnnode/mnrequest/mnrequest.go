package mnrequest

import "the-dev-tools/server/pkg/idwrap"

type MNRequest struct {
	FlowNodeID idwrap.IDWrap
	HttpID     idwrap.IDWrap

	// Legacy fields retained temporarily for migration; will be removed once callers
	// rely exclusively on HttpID.
	DeltaExampleID  *idwrap.IDWrap
	DeltaEndpointID *idwrap.IDWrap
	EndpointID      *idwrap.IDWrap
	ExampleID       *idwrap.IDWrap

	HasRequestConfig bool
}
