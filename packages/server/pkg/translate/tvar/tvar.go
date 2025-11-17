package tvar

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mvar"
)

// TODO: variable/v1 protobuf package doesn't exist. Stub types provided.

type Variable struct {
	VariableId  []byte
	Name        string
	Value       string
	Enabled     bool
	Description string
}

type VariableListItem struct {
	VariableId []byte
	Name       string
	Enabled    bool
}

func SerializeModelToRPC(v mvar.Var) *Variable {
	return &Variable{
		VariableId:  v.ID.Bytes(),
		Name:        v.VarKey,
		Value:       v.Value,
		Enabled:     v.Enabled,
		Description: v.Description,
	}
}

func SerializeModelToRPCItem(v mvar.Var) *VariableListItem {
	return &VariableListItem{
		VariableId: v.ID.Bytes(),
		Name:       v.VarKey,
		Enabled:    v.Enabled,
	}
}

func DeserializeRPCToModel(v *Variable) (mvar.Var, error) {
	if v == nil {
		return mvar.Var{}, nil
	}

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

func DeserializeRPCToModelList(items []*VariableListItem) ([]mvar.Var, error) {
	if len(items) == 0 {
		return []mvar.Var{}, nil
	}

	result := make([]mvar.Var, 0, len(items))
	for _, item := range items {
		id, err := idwrap.NewFromBytes(item.VariableId)
		if err != nil {
			return nil, err
		}

		result = append(result, mvar.Var{
			ID:       id,
			VarKey:   item.Name,
			Enabled:  item.Enabled,
		})
	}
	return result, nil
}