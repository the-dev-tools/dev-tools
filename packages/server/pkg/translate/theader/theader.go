package theader

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
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

func SerializeHeaderModelToRPCDeltaItem(header mexampleheader.Header) *requestv1.HeaderDeltaListItem {
	return &requestv1.HeaderDeltaListItem{
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
	headerId, err := idwrap.NewFromBytes(header.GetHeaderId())
	if err != nil {
		return mexampleheader.Header{}, err
	}
	var deltaParentID *idwrap.IDWrap
	h := SerlializeHeaderRPCtoModelNoID(header, exampleID, deltaParentID)
	h.ID = headerId
	return h, nil
}

func SerlializeHeaderRPCtoModelNoID(header *requestv1.Header, exampleID idwrap.IDWrap, parentID *idwrap.IDWrap) mexampleheader.Header {
	return mexampleheader.Header{
		ExampleID:     exampleID,
		HeaderKey:     header.GetKey(),
		Description:   header.GetDescription(),
		Enable:        header.GetEnabled(),
		Value:         header.GetValue(),
		DeltaParentID: parentID,
	}
}
