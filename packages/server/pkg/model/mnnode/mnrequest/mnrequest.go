package mnrequest

import "the-dev-tools/server/pkg/idwrap"

type MNRequest struct {
	FlowNodeID idwrap.IDWrap
	HttpID     *idwrap.IDWrap

	DeltaHttpID *idwrap.IDWrap

	HasRequestConfig bool
}
