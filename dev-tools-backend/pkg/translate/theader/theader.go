package theader

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexampleheader"
	requestv1 "dev-tools-spec/dist/buf/go/collection/item/request/v1"
)

func SerializeHeaderModelToRPC(header mexampleheader.Header) *requestv1.Header {
	return &requestv1.Header{
		HeaderId:    header.ID.Bytes(),
		Key:         header.HeaderKey,
		Enabled:     header.Enable,
		Value:       header.Value,
		Description: header.Description,
	}
}

func SerializeHeaderModelToRPCItem(header mexampleheader.Header) *requestv1.HeaderListItem {
	return &requestv1.HeaderListItem{
		HeaderId:    header.ID.Bytes(),
		Key:         header.HeaderKey,
		Enabled:     header.Enable,
		Value:       header.Value,
		Description: header.Description,
	}
}

func SerlializeHeaderRPCtoModel(header *requestv1.Header, exampleID idwrap.IDWrap) (mexampleheader.Header, error) {
	h := SerlializeHeaderRPCtoModelNoID(header, exampleID)
	headerId, err := idwrap.NewFromBytes(header.GetHeaderId())
	if err != nil {
		return mexampleheader.Header{}, err
	}
	h.ID = headerId
	return h, nil
}

func SerlializeHeaderRPCtoModelNoID(header *requestv1.Header, exampleID idwrap.IDWrap) mexampleheader.Header {
	return mexampleheader.Header{
		ExampleID:   exampleID,
		HeaderKey:   header.GetKey(),
		Description: header.GetDescription(),
		Enable:      header.GetEnabled(),
		Value:       header.GetValue(),
	}
}
