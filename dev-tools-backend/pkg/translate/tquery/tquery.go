package tquery

import (
	"dev-tools-backend/pkg/model/mexamplequery"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"

	"github.com/oklog/ulid/v2"
)

func SerializeQueryModelToRPC(query mexamplequery.Query) *itemapiexamplev1.Query {
	return &itemapiexamplev1.Query{
		Id:          query.ID.String(),
		Key:         query.QueryKey,
		Enabled:     query.Enable,
		Description: query.Description,
		Value:       query.Value,
	}
}

func SerlializeQueryRPCtoModel(query *itemapiexamplev1.Query) (mexamplequery.Query, error) {
	queryId, err := ulid.Parse(query.GetId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	exampleUlid, err := ulid.Parse(query.GetExampleId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	return mexamplequery.Query{
		ID:          queryId,
		ExampleID:   exampleUlid,
		QueryKey:    query.GetKey(),
		Enable:      query.GetEnabled(),
		Description: query.GetDescription(),
		Value:       query.GetValue(),
	}, nil
}

func SerlializeQueryRPCtoModelNoID(query *itemapiexamplev1.Query) (mexamplequery.Query, error) {
	exampleUlid, err := ulid.Parse(query.GetExampleId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	return mexamplequery.Query{
		QueryKey:    query.GetKey(),
		ExampleID:   exampleUlid,
		Enable:      query.GetEnabled(),
		Description: query.GetDescription(),
		Value:       query.GetValue(),
	}, nil
}
