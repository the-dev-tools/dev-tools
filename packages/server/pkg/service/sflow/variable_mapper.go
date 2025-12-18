package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertFlowVariableToDB(item mflow.FlowVariable) gen.FlowVariable {
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

func ConvertDBToFlowVariable(item gen.FlowVariable) mflow.FlowVariable {
	return mflow.FlowVariable{
		ID:          item.ID,
		FlowID:      item.FlowID,
		Name:        item.Key,
		Value:       item.Value,
		Enabled:     item.Enabled,
		Description: item.Description,
		Order:       item.DisplayOrder,
	}
}
