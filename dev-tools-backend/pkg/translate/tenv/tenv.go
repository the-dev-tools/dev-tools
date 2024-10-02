package tenv

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	environmentv1 "dev-tools-services/gen/environment/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func SeralizeModelToRPC(e menv.Env) *environmentv1.Environment {
	rpcEnvType := environmentv1.EnvironmentType(e.Type)

	return &environmentv1.Environment{
		Id:          e.ID.String(),
		Name:        e.Name,
		Type:        rpcEnvType,
		Description: e.Description,
		UpdatedAt:   timestamppb.New(e.Updated),
	}
}

func DeserializeRPCToModel(e *environmentv1.Environment) (menv.Env, error) {
	id, err := idwrap.NewWithParse(e.Id)
	if err != nil {
		return menv.Env{}, err
	}
	return menv.Env{
		ID:          id,
		Name:        e.Name,
		Type:        menv.EnvType(e.Type),
		Description: e.Description,
		Updated:     e.UpdatedAt.AsTime(),
	}, nil
}

func DeseralizeRPCToModelWithID(id idwrap.IDWrap, e *environmentv1.Environment) menv.Env {
	return menv.Env{
		ID:          id,
		Name:        e.Name,
		Type:        menv.EnvType(e.Type),
		Description: e.Description,
		Updated:     e.UpdatedAt.AsTime(),
	}
}
