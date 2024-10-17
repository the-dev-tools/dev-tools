package tassert

/*

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/massert"
	itemapiexamplev1 "dev-tools-services/gen/itemapiexample/v1"
)

func SerializeAssertModelToRPC(a massert.Assert) *itemapiexamplev1.Asssert {
	return &itemapiexamplev1.Asssert{
		Id:        a.ID.String(),
		ExampleId: a.ExampleID.String(),
		Name:      a.Name,
		Value:     a.Value,
		Type:      itemapiexamplev1.AssertType(a.Type),
		Target:    itemapiexamplev1.AssertTarget(a.Target),
	}
}

func SerializeAssertRPCToModel(a *itemapiexamplev1.Asssert) (massert.Assert, error) {
	id, err := idwrap.NewWithParse(a.GetId())
	if err != nil {
		return massert.Assert{}, err
	}

	exampleID, err := idwrap.NewWithParse(a.GetExampleId())
	if err != nil {
		return massert.Assert{}, err
	}

	return massert.Assert{
		ID:        id,
		ExampleID: exampleID,
		Name:      a.Name,
		Value:     a.Value,
		Type:      massert.AssertType(a.Type),
		Target:    massert.AssertTarget(a.Target),
	}, nil
}

func SerializeAssertRPCToModelWithoutID(a *itemapiexamplev1.Asssert) (massert.Assert, error) {
	exampleID, err := idwrap.NewWithParse(a.GetExampleId())
	if err != nil {
		return massert.Assert{}, err
	}

	return massert.Assert{
		ExampleID: exampleID,
		Name:      a.Name,
		Value:     a.Value,
		Type:      massert.AssertType(a.Type),
		Target:    massert.AssertTarget(a.Target),
	}, nil
}
*/
