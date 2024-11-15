package mflow

import "dev-tools-backend/pkg/idwrap"

type Flow struct {
	ID   idwrap.IDWrap
	Name string
}

type FlowTag struct {
	ID   idwrap.IDWrap
	Name string
}

type FlowTags struct {
	ID     idwrap.IDWrap
	FlowID idwrap.IDWrap
	TagID  idwrap.IDWrap
}
