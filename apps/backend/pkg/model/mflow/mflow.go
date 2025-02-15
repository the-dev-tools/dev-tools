package mflow

import "the-dev-tools/backend/pkg/idwrap"

type Flow struct {
	ID         idwrap.IDWrap
	FlowRootID idwrap.IDWrap
	Name       string
}
