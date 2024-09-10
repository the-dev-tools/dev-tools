package theader

import (
	"dev-tools-backend/pkg/model/mexampleheader"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"

	"github.com/oklog/ulid/v2"
)

func SerializeHeaderModelToRPC(header mexampleheader.Header) *itemapiexamplev1.Header {
	return &itemapiexamplev1.Header{
		Id:          header.ID.String(),
		Key:         header.HeaderKey,
		Enabled:     header.Enable,
		Description: header.Description,
		Value:       header.Value,
	}
}

func SerlializeHeaderRPCtoModel(header *itemapiexamplev1.Header) (mexampleheader.Header, error) {
	headerId, err := ulid.Parse(header.GetId())
	if err != nil {
		return mexampleheader.Header{}, err
	}

	return mexampleheader.Header{
		ID:          headerId,
		HeaderKey:   header.GetKey(),
		Enable:      header.GetEnabled(),
		Description: header.GetDescription(),
		Value:       header.GetValue(),
	}, nil
}
