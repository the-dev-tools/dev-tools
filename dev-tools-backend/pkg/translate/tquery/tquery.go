package tquery

/*

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexamplequery"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
)

func SerializeQueryModelToRPC(query mexamplequery.Query) *itemapiexamplev1.Query {
	return &itemapiexamplev1.Query{
		Id:          query.ID.String(),
		ExampleId:   query.ExampleID.String(),
		Key:         query.QueryKey,
		Enabled:     query.Enable,
		Description: query.Description,
		Value:       query.Value,
	}
}

func SerlializeQueryRPCtoModel(query *itemapiexamplev1.Query) (mexamplequery.Query, error) {
	queryId, err := idwrap.NewWithParse(query.GetId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	exampleID, err := idwrap.NewWithParse(query.GetExampleId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	return mexamplequery.Query{
		ID:          queryId,
		ExampleID:   exampleID,
		QueryKey:    query.GetKey(),
		Enable:      query.GetEnabled(),
		Description: query.GetDescription(),
		Value:       query.GetValue(),
	}, nil
}

func SerlializeQueryRPCtoModelNoID(query *itemapiexamplev1.Query) (mexamplequery.Query, error) {
	exampleID, err := idwrap.NewWithParse(query.GetExampleId())
	if err != nil {
		return mexamplequery.Query{}, err
	}
	return mexamplequery.Query{
		QueryKey:    query.GetKey(),
		ExampleID:   exampleID,
		Enable:      query.GetEnabled(),
		Description: query.GetDescription(),
		Value:       query.GetValue(),
	}, nil
}
*/
