package sflow

import (
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func nullStringFromPtr(s *string) sql.NullString {
	if s != nil {
		return sql.NullString{String: *s, Valid: true}
	}
	return sql.NullString{}
}

func ConvertFlowToDB(item mflow.Flow) gen.Flow {
	errField := nullStringFromPtr(item.Error)
	return gen.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
		Error:           errField,
		NodeIDMapping:   item.NodeIDMapping,
	}
}

func ConvertDBToFlow(item gen.Flow) mflow.Flow {
	var errField *string
	if item.Error.Valid {
		errField = &item.Error.String
	}
	return mflow.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
		Error:           errField,
		NodeIDMapping:   item.NodeIDMapping,
	}
}
