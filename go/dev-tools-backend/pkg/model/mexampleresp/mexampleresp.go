package mexampleresp

import (
	"dev-tools-backend/pkg/idwrap"
)

type BodyCompressType uint8

const (
	BodyCompressTypeNone BodyCompressType = 0
	BodyCompressTypeZstd BodyCompressType = 1
)

type ExampleResp struct {
	ID               idwrap.IDWrap
	ExampleID        idwrap.IDWrap
	Status           uint16
	Body             []byte
	BodyCompressType BodyCompressType
	Duration         int32
}
