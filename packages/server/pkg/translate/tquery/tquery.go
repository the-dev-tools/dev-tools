package tquery

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplequery"
)

// TODO: collection/item/request/v1 protobuf package doesn't exist. Stub types provided.

type Query struct {
	QueryId     []byte
	Key         string
	Enabled     bool
	Description string
	Value       string
}

type QueryListItem struct {
	QueryId []byte
	Key     string
	Enabled bool
}

func SerializeQueryModelToRPC(query mexamplequery.Query) *Query {
	return &Query{
		QueryId:     query.ID.Bytes(),
		Key:         query.QueryKey,
		Enabled:     query.Enable,
		Description: query.Description,
		Value:       query.Value,
	}
}

func SerializeQueryModelToRPCItem(query mexamplequery.Query) *QueryListItem {
	return &QueryListItem{
		QueryId: query.ID.Bytes(),
		Key:     query.QueryKey,
		Enabled: query.Enable,
	}
}

func DeserializeRPCToModel(query *Query) (mexamplequery.Query, error) {
	if query == nil {
		return mexamplequery.Query{}, nil
	}

	id, err := idwrap.NewFromBytes(query.QueryId)
	if err != nil {
		return mexamplequery.Query{}, err
	}

	return mexamplequery.Query{
		ID:          id,
		QueryKey:    query.Key,
		Enable:      query.Enabled,
		Description: query.Description,
		Value:       query.Value,
	}, nil
}

func DeserializeRPCToModelList(items []*QueryListItem) ([]mexamplequery.Query, error) {
	if len(items) == 0 {
		return []mexamplequery.Query{}, nil
	}

	result := make([]mexamplequery.Query, 0, len(items))
	for _, item := range items {
		id, err := idwrap.NewFromBytes(item.QueryId)
		if err != nil {
			return nil, err
		}

		result = append(result, mexamplequery.Query{
			ID:       id,
			QueryKey: item.Key,
			Enable:   item.Enabled,
		})
	}
	return result, nil
}