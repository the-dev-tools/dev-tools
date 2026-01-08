package stag

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mtag"
)

func ConvertDBToModel(item gen.Tag) mtag.Tag {
	return mtag.Tag{
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		Name:        item.Name,
		Color:       mtag.Color(item.Color), // nolint:gosec // G115
	}
}

func ConvertModelToDB(item mtag.Tag) gen.Tag {
	return gen.Tag{
		ID:          item.ID,
		WorkspaceID: item.WorkspaceID,
		Name:        item.Name,
		Color:       int8(item.Color), // nolint:gosec // G115
	}
}
