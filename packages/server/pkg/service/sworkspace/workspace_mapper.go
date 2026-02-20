package sworkspace

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
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
		OrganizationID:  workspace.OrganizationID,
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
		OrganizationID:  workspace.OrganizationID,
	}
}
