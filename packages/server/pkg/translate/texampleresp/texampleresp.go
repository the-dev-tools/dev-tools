package texampleresp

import (
	"errors"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/zstdcompress"
	responsev1 "the-dev-tools/spec/dist/buf/go/collection/item/response/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

/*
func SeralizeHeaderModelToRPC(h mexamplerespheader.ExampleRespHeader) *itemapiexamplev1.ResponseHeader {
	return &itemapiexamplev1.ResponseHeader{
		Id:    h.ID.String(),
		Key:   h.HeaderKey,
		Value: h.Value,
	}
}
*/

var ErrDecompress error = errors.New("failed to decompress body")

func SeralizeModelToRPC(e mexampleresp.ExampleResp) (*responsev1.Response, error) {
	body := e.Body
	if e.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
		var err error
		body, err = zstdcompress.Decompress(body)
		if err != nil {
			return nil, errors.Join(ErrDecompress, err)
		}
	}

	return &responsev1.Response{
		ResponseId: e.ID.Bytes(),
		Status:     int32(e.Status),
		Body:       body,
		Time:       timestamppb.New(e.ID.Time()),
		Duration:   e.Duration,
	}, nil
}

func SeralizeModelToRPCGetResponse(e mexampleresp.ExampleResp) (*responsev1.ResponseGetResponse, error) {
	body := e.Body
	if e.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
		var err error
		body, err = zstdcompress.Decompress(body)
		if err != nil {
			return nil, errors.Join(ErrDecompress, err)
		}
	}

	return &responsev1.ResponseGetResponse{
		ResponseId: e.ID.Bytes(),
		Status:     int32(e.Status),
		Body:       body,
		Time:       timestamppb.New(e.ID.Time()),
		Duration:   e.Duration,
		Size:       int32(len(body)),
	}, nil
}
