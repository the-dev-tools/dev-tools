package sflow

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func ConvertFlowToDB(item mflow.Flow) gen.Flow {
	return gen.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
		NodeIDMapping:   item.NodeIDMapping,
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
		NodeIDMapping:   item.NodeIDMapping,
	}
}
