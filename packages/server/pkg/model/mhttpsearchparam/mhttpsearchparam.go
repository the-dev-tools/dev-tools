package mhttpsearchparam

import (
	"the-dev-tools/server/pkg/idwrap"
)

type HttpSearchParam struct {
	ID                      idwrap.IDWrap
	HttpID                  idwrap.IDWrap
	Key                     string
	Value                   string
	Enabled                 bool
	Description             string
	Order                   float64
	ParentHttpSearchParamID *idwrap.IDWrap
	IsDelta                 bool
	DeltaKey                *string
	DeltaValue              *string
	DeltaEnabled            *bool
	DeltaDescription        *string
	DeltaOrder              *float64
	CreatedAt               int64
	UpdatedAt               int64
}

func (h HttpSearchParam) IsEnabled() bool {
	return h.Enabled
}
