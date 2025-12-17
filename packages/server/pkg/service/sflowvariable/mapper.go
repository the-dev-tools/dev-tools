package sflowvariable

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflowvariable"
)

func ConvertModelToDB(item mflowvariable.FlowVariable) gen.FlowVariable {
	return gen.FlowVariable{
		ID:           item.ID,
		FlowID:       item.FlowID,
		Key:          item.Name,
		Value:        item.Value,
		Enabled:      item.Enabled,
		Description:  item.Description,
		DisplayOrder: item.Order,
	}
}

func ConvertDBToModel(item gen.FlowVariable) mflowvariable.FlowVariable {
	return mflowvariable.FlowVariable{
		ID:          item.ID,
		FlowID:      item.FlowID,
		Name:        item.Key,
		Value:       item.Value,
		Enabled:     item.Enabled,
		Description: item.Description,
		Order:       item.DisplayOrder,
	}
}
