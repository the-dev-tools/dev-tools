package tvar

import (
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mvar"
	variablev1 "dev-tools-services/gen/variable/v1"
)

func SerializeModelToRPC(v mvar.Var) *variablev1.Variable {
	return &variablev1.Variable{
		Id:          v.ID.String(),
		Name:        v.VarKey,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}

func DeserializeRPCToModel(v *variablev1.Variable) (mvar.Var, error) {
	id, err := idwrap.NewWithParse(v.Id)
	if err != nil {
		return mvar.Var{}, err
	}

	return mvar.Var{
		ID:          id,
		VarKey:      v.Name,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}, nil
}

func DeserializeRPCToModelWithID(id, envID idwrap.IDWrap, v *variablev1.Variable) mvar.Var {
	return mvar.Var{
		ID:          id,
		VarKey:      v.Name,
		EnvID:       envID,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}
