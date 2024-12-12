package mnrequest

import "the-dev-tools/backend/pkg/idwrap"

type MNRequest struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	ExampleID  idwrap.IDWrap
}
