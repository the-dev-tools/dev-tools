package tenv

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	environmentv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func SeralizeModelToRPC(e menv.Env) *environmentv1.Environment {
	return &environmentv1.Environment{
		EnvironmentId: e.ID.Bytes(),
		Name:          e.Name,
		IsGlobal:      e.Type == menv.EnvGlobal,
		Description:   e.Description,
		Updated:       timestamppb.New(e.Updated),
	}
}

func DeserializeRPCToModel(e *environmentv1.Environment) (menv.Env, error) {
	id, err := idwrap.NewFromBytes(e.EnvironmentId)
	if err != nil {
		return menv.Env{}, err
	}

	return DeseralizeRPCToModelWithID(id, e), nil
}

func SeralizeModelToRPCItem(e menv.Env) *environmentv1.EnvironmentListItem {
	return &environmentv1.EnvironmentListItem{
		EnvironmentId: e.ID.Bytes(),
		Name:          e.Name,
		IsGlobal:      e.Type == menv.EnvGlobal,
		Description:   e.Description,
		Updated:       timestamppb.New(e.Updated),
	}
}

func DeseralizeRPCToModelWithID(id idwrap.IDWrap, e *environmentv1.Environment) menv.Env {
	var typ menv.EnvType
	if e.IsGlobal {
		typ = menv.EnvGlobal
	} else {
		typ = menv.EnvNormal
	}

	return menv.Env{
		ID:          id,
		Name:        e.Name,
		Type:        typ,
		Description: e.Description,
		Updated:     e.Updated.AsTime(),
	}
}

/*
func SeralizeModelToGroupRPC(key string, envs []menv.Env) *variablev1.Variable {
	return &environmentv1.EnvironmentListItem{
		VariableKey: key,
		Environment: tgeneric.MassConvert(envs, SeralizeModelToRPC),
	}
}
*/
