package texampleresp

import (
	"dev-tools-backend/pkg/model/mexampleresp"
	"dev-tools-backend/pkg/model/mexamplerespheader"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/zstdcompress"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func SeralizeHeaderModelToRPC(h mexamplerespheader.ExampleRespHeader) *itemapiexamplev1.ResponseHeader {
	return &itemapiexamplev1.ResponseHeader{
		Id:    h.ID.String(),
		Key:   h.HeaderKey,
		Value: h.Value,
	}
}

func SeralizeModelToRPC(e mexampleresp.ExampleResp, h []mexamplerespheader.ExampleRespHeader) (*itemapiexamplev1.ApiExampleResponse, error) {
	body := e.Body
	if e.BodyCompressType == mexampleresp.BodyCompressTypeZstd {
		var err error
		body, err = zstdcompress.Decompress(body)
		if err != nil {
			return nil, err
		}
	}

	return &itemapiexamplev1.ApiExampleResponse{
		Id:        e.ID.String(),
		ExampleId: e.ExampleID.String(),
		Status:    int32(e.Status),
		Body:      body,
		Time:      timestamppb.New(e.ID.Time()),
		Duration:  e.Duration,
		Headers:   tgeneric.MassConvert(h, SeralizeHeaderModelToRPC),
	}, nil
}
