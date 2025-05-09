package tvar

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
	variablev1 "the-dev-tools/spec/dist/buf/go/variable/v1"
)

func SerializeModelToRPC(v mvar.Var) *variablev1.Variable {
	return &variablev1.Variable{
		VariableId:  v.ID.Bytes(),
		Name:        v.VarKey,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}

func SerializeModelToRPCItem(v mvar.Var) *variablev1.VariableListItem {
	return &variablev1.VariableListItem{
		VariableId:  v.ID.Bytes(),
		Name:        v.VarKey,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}

func DeserializeRPCToModel(v *variablev1.Variable) (mvar.Var, error) {
	id, err := idwrap.NewFromBytes(v.VariableId)
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
