package sflow

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mflow"
)

func ConvertFlowToDB(item mflow.Flow) gen.Flow {
	return gen.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
	}
}

func ConvertDBToFlow(item gen.Flow) mflow.Flow {
	return mflow.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
	}
}
