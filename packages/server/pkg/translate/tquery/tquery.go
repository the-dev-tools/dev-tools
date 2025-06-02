package tquery

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplequery"
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
	return &requestv1.QueryListItem{
		QueryId:     query.ID.Bytes(),
		Key:         query.QueryKey,
		Enabled:     query.Enable,
		Description: query.Description,
		Value:       query.Value,
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

	return mexamplequery.Query{
		QueryKey:      query.GetKey(),
		ExampleID:     exID,
		Enable:        query.GetEnabled(),
		Description:   query.GetDescription(),
		DeltaParentID: parentDeltaIDPtr,
		Value:         query.GetValue(),
		Source:        mexamplequery.QuerySourceOrigin, // Default to origin
	}, nil
}

func SerlializeQueryRPCtoModelNoIDForDelta(query *requestv1.Query, exID idwrap.IDWrap) (mexamplequery.Query, error) {
	var parentDeltaIDPtr *idwrap.IDWrap

	return mexamplequery.Query{
		QueryKey:      query.GetKey(),
		ExampleID:     exID,
		Enable:        query.GetEnabled(),
		Description:   query.GetDescription(),
		DeltaParentID: parentDeltaIDPtr,
		Value:         query.GetValue(),
		Source:        mexamplequery.QuerySourceDelta, // Set to delta for delta creation
	}, nil
}

func SerializeQueryModelToRPCDeltaItem(query mexamplequery.Query) *requestv1.QueryDeltaListItem {
	return &requestv1.QueryDeltaListItem{
		QueryId:     query.ID.Bytes(),
		Key:         query.QueryKey,
		Enabled:     query.Enable,
		Description: query.Description,
		Value:       query.Value,
	}
}
