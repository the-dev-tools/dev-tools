package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertModelToDB(item mflow.Flow) gen.Flow {
	return gen.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
	}
}

func ConvertDBToModel(item gen.Flow) mflow.Flow {
	return mflow.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
	}
}
