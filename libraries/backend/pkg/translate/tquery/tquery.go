package tquery

import (
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mexamplequery"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
)

func SerializeQueryModelToRPC(query mexamplequery.Query) *requestv1.Query {
	return &requestv1.Query{
		QueryId:     query.ID.Bytes(),
		Key:         query.QueryKey,
		Enabled:     query.Enable,
		Description: query.Description,
		Value:       query.Value,
	}
}

func SerializeQueryModelToRPCItem(query mexamplequery.Query) *requestv1.QueryListItem {
	var parentDeltaIDBytes []byte
	if query.DeltaParentID != nil {
		parentDeltaIDBytes = query.DeltaParentID.Bytes()
	}

	return &requestv1.QueryListItem{
		QueryId:       query.ID.Bytes(),
		Key:           query.QueryKey,
		Enabled:       query.Enable,
		Description:   query.Description,
		ParentQueryId: parentDeltaIDBytes,
		Value:         query.Value,
	}
}

func SerlializeQueryRPCtoModel(query *requestv1.Query, exID idwrap.IDWrap) (mexamplequery.Query, error) {
	q, err := SerlializeQueryRPCtoModelNoID(query, exID)
	if err != nil {
		return mexamplequery.Query{}, err
	}
	queryId, err := idwrap.NewFromBytes(query.GetQueryId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	q.ID = queryId
	return q, nil
}

func SerlializeQueryRPCtoModelNoID(query *requestv1.Query, exID idwrap.IDWrap) (mexamplequery.Query, error) {
	var parentDeltaIDPtr *idwrap.IDWrap
	if len(query.ParentQueryId) > 0 {
		parentDeltaID, err := idwrap.NewFromBytes(query.ParentQueryId)
		if err != nil {
			return mexamplequery.Query{}, err
		}
		parentDeltaIDPtr = &parentDeltaID
	}

	return mexamplequery.Query{
		QueryKey:      query.GetKey(),
		ExampleID:     exID,
		Enable:        query.GetEnabled(),
		Description:   query.GetDescription(),
		DeltaParentID: parentDeltaIDPtr,
		Value:         query.GetValue(),
	}, nil
}
