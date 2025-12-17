package sfile

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mfile"
	"time"
)

// ConvertToDBFile converts model to DB representation
func ConvertToDBFile(file mfile.File) gen.File {
	return gen.File{
		ID:           file.ID,
		WorkspaceID:  file.WorkspaceID,
		ParentID:     file.ParentID,
		ContentID:    file.ContentID,
		ContentKind:  int8(file.ContentType),
		Name:         file.Name,
		DisplayOrder: file.Order,
		UpdatedAt:    file.UpdatedAt.Unix(),
	}
}

// ConvertToModelFile converts DB to model representation
func ConvertToModelFile(file gen.File) *mfile.File {
	return &mfile.File{
		ID:          file.ID,
		WorkspaceID: file.WorkspaceID,
		ParentID:    file.ParentID,
		ContentID:   file.ContentID,
		ContentType: mfile.ContentType(file.ContentKind),
		Name:        file.Name,
		Order:       file.DisplayOrder,
		UpdatedAt:   time.Unix(file.UpdatedAt, 0),
	}
}
