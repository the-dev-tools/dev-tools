package theader

import (
	"dev-tools-backend/pkg/model/mexampleheader"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
	"errors"

	"github.com/oklog/ulid/v2"
)

func SerializeHeaderModelToRPC(header mexampleheader.Header) *itemapiexamplev1.Header {
	return &itemapiexamplev1.Header{
		Id:          header.ID.String(),
		ExampleId:   header.ExampleID.String(),
		Key:         header.HeaderKey,
		Enabled:     header.Enable,
		Description: header.Description,
		Value:       header.Value,
	}
}

func SerlializeHeaderRPCtoModel(header *itemapiexamplev1.Header) (mexampleheader.Header, error) {
	if header == nil {
		return mexampleheader.Header{}, errors.New("header is nil")
	}
	headerId, err := ulid.Parse(header.GetId())
	if err != nil {
		return mexampleheader.Header{}, err
	}
	exampleId, err := ulid.Parse(header.GetExampleId())
	if err != nil {
		return mexampleheader.Header{}, err
	}

	return mexampleheader.Header{
		ID:          headerId,
		ExampleID:   exampleId,
		HeaderKey:   header.GetKey(),
		Enable:      header.GetEnabled(),
		Description: header.GetDescription(),
		Value:       header.GetValue(),
	}, nil
}

func SerlializeHeaderRPCtoModelNoID(header *itemapiexamplev1.Header) (mexampleheader.Header, error) {
	if header == nil {
		return mexampleheader.Header{}, errors.New("header is nil")
	}
	exampleUlID, err := ulid.Parse(header.GetExampleId())
	if err != nil {
		return mexampleheader.Header{}, err
	}
	return mexampleheader.Header{
		ExampleID:   exampleUlID,
		HeaderKey:   header.GetKey(),
		Description: header.GetDescription(),
		Enable:      header.GetEnabled(),
		Value:       header.GetValue(),
	}, nil
}
