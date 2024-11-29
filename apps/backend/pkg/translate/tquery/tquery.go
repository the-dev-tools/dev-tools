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
	return &requestv1.QueryListItem{
		QueryId:     query.ID.Bytes(),
		Key:         query.QueryKey,
		Enabled:     query.Enable,
		Description: query.Description,
		Value:       query.Value,
	}
}

func SerlializeQueryRPCtoModel(query *requestv1.Query, exID idwrap.IDWrap) (mexamplequery.Query, error) {
	q := SerlializeQueryRPCtoModelNoID(query, exID)
	queryId, err := idwrap.NewFromBytes(query.GetQueryId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	q.ID = queryId
	return q, nil
}

func SerlializeQueryRPCtoModelNoID(query *requestv1.Query, exID idwrap.IDWrap) mexamplequery.Query {
	return mexamplequery.Query{
		QueryKey:    query.GetKey(),
		ExampleID:   exID,
		Enable:      query.GetEnabled(),
		Description: query.GetDescription(),
		Value:       query.GetValue(),
	}
}
