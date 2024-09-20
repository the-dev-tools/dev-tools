package texample

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mexampleheader"
	"dev-tools-backend/pkg/model/mexamplequery"
	"dev-tools-backend/pkg/model/mitemapiexample"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/theader"
	"dev-tools-backend/pkg/translate/tquery"
	bodyv1 "dev-tools-services/gen/body/v1"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func SerializeModelToRPC(ex mitemapiexample.ItemApiExample, q []mexamplequery.Query, h []mexampleheader.Header, b *bodyv1.Body) *itemapiexamplev1.ApiExample {
	return &itemapiexamplev1.ApiExample{
		Meta: &itemapiexamplev1.ApiExampleMeta{
			Id:   ex.ID.String(),
			Name: ex.Name,
		},
		Query:   tgeneric.MassConvert(q, tquery.SerializeQueryModelToRPC),
		Header:  tgeneric.MassConvert(h, theader.SerializeHeaderModelToRPC),
		Updated: timestamppb.New(ex.Updated),
		Body:    b,
	}
}

func DeserializeRPCToModel(ex *itemapiexamplev1.ApiExample) (mitemapiexample.ItemApiExample, error) {
	if ex == nil {
		return mitemapiexample.ItemApiExample{}, nil
	}
	if ex.Meta == nil {
		return mitemapiexample.ItemApiExample{}, nil
	}
	id, err := idwrap.NewWithParse(ex.Meta.Id)
	if err != nil {
		return mitemapiexample.ItemApiExample{}, err
	}

	return mitemapiexample.ItemApiExample{
		ID:      id,
		Name:    ex.Meta.Name,
		Updated: ex.Updated.AsTime(),
	}, nil
}
