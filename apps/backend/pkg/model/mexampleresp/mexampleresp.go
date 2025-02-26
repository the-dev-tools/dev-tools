package mexampleresp

import (
	"the-dev-tools/backend/pkg/idwrap"
)

type BodyCompressType = uint8

const (
	BodyCompressTypeNone BodyCompressType = iota
	BodyCompressTypeZstd
)

type ExampleResp struct {
	ID               idwrap.IDWrap
	ExampleID        idwrap.IDWrap
	Status           uint16
	Body             []byte
	BodyCompressType BodyCompressType
	Duration         int32
}
