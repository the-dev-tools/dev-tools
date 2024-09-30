package tenv

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	environmentv1 "dev-tools-services/gen/environment/v1"
)

func SeralizeModelToRPC(e menv.Env) *environmentv1.Environment {
	return &environmentv1.Environment{
		Id:   e.ID.String(),
		Name: e.Name,
	}
}

func DeserializeRPCToModel(e *environmentv1.Environment) (menv.Env, error) {
	id, err := idwrap.NewWithParse(e.Id)
	if err != nil {
		return menv.Env{}, err
	}
	return menv.Env{
		ID:   id,
		Name: e.Name,
	}, nil
}

func DeseralizeRPCToModelWithID(id idwrap.IDWrap, e *environmentv1.Environment) menv.Env {
	return menv.Env{
		ID:   id,
		Name: e.Name,
	}
}
