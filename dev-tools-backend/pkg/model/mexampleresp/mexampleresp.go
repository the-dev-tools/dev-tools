package mexampleresp

import "dev-tools-backend/pkg/idwrap"

type ExampleResp struct {
	ID           idwrap.IDWrap
	ExampleID    idwrap.IDWrap
	RespStatus   uint16
	RespBody     []byte
	RespTime     int64
	RespDuration int32
}
