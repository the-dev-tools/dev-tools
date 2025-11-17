package texampleresp

import (
	"errors"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/zstdcompress"
	httpv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"

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

func SeralizeModelToRPC(e mexampleresp.ExampleResp) (*httpv1.HttpResponse, error) {
	body := e.Body
	if e.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
		var err error
		body, err = zstdcompress.Decompress(body)
		if err != nil {
			return nil, errors.Join(ErrDecompress, err)
		}
	}

	return &httpv1.HttpResponse{
		ResponseId: e.ID.Bytes(),
		Status:     int32(e.Status),
		Body:       body,
		Time:       timestamppb.New(e.ID.Time()),
		Duration:   e.Duration,
	}, nil
}

// ResponseGetResponse represents a response with additional metadata.
// TODO: Replace with actual protobuf type when available
type ResponseGetResponse struct {
	ResponseId []byte                 `protobuf:"bytes,1,opt,name=response_id,json=responseId,proto3" json:"response_id,omitempty"`
	Status     int32                  `protobuf:"varint,2,opt,name=status,proto3" json:"status,omitempty"`
	Body       []byte                 `protobuf:"bytes,3,opt,name=body,proto3" json:"body,omitempty"`
	Time       *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=time,proto3" json:"time,omitempty"`
	Duration   int64                  `protobuf:"varint,5,opt,name=duration,proto3" json:"duration,omitempty"`
	Size       int32                  `protobuf:"varint,6,opt,name=size,proto3" json:"size,omitempty"`
}

func SeralizeModelToRPCGetResponse(e mexampleresp.ExampleResp) (*ResponseGetResponse, error) {
	body := e.Body
	if e.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
		var err error
		body, err = zstdcompress.Decompress(body)
		if err != nil {
			return nil, errors.Join(ErrDecompress, err)
		}
	}

	return &ResponseGetResponse{
		ResponseId: e.ID.Bytes(),
		Status:     int32(e.Status),
		Body:       body,
		Time:       timestamppb.New(e.ID.Time()),
		Duration:   e.Duration,
		Size:       int32(len(body)),
	}, nil
}
