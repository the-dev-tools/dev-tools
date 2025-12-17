package sworkspace

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/model/mworkspace"
	"time"
)

func ConvertToDBWorkspace(workspace mworkspace.Workspace) gen.Workspace {
	return gen.Workspace{
		ID:              workspace.ID,
		Name:            workspace.Name,
		Updated:         workspace.Updated.Unix(),
		CollectionCount: workspace.CollectionCount,
		FlowCount:       workspace.FlowCount,
		ActiveEnv:       workspace.ActiveEnv,
		GlobalEnv:       workspace.GlobalEnv,
		DisplayOrder:    workspace.Order,
	}
}

func ConvertToModelWorkspace(workspace gen.Workspace) mworkspace.Workspace {
	return mworkspace.Workspace{
		ID:              workspace.ID,
		Name:            workspace.Name,
		Updated:         dbtime.DBTime(time.Unix(workspace.Updated, 0)),
		CollectionCount: workspace.CollectionCount,
		FlowCount:       workspace.FlowCount,
		ActiveEnv:       workspace.ActiveEnv,
		GlobalEnv:       workspace.GlobalEnv,
		Order:           workspace.DisplayOrder,
	}
}
