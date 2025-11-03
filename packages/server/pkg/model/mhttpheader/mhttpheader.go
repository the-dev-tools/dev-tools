package mhttpheader

import "the-dev-tools/server/pkg/idwrap"

type HttpHeader struct {
	ID                 idwrap.IDWrap
	HttpID             idwrap.IDWrap
	Key                string
	Value              string
	Enabled            bool
	Description        string
	Order              float32
	ParentHttpHeaderID *idwrap.IDWrap
	IsDelta            bool
	DeltaKey           *string
	DeltaValue         *string
	DeltaEnabled       *bool
	DeltaDescription   *string
	DeltaOrder         *float32
	CreatedAt          int64
	UpdatedAt          int64
}

func (h HttpHeader) IsEnabled() bool {
	return h.Enabled
}
