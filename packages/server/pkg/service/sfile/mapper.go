package sfile

import (
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"time"
)

// ConvertToDBFile converts model to DB representation
func ConvertToDBFile(file mfile.File) gen.File {
	var pathHash sql.NullString
	if file.PathHash != nil {
		pathHash = sql.NullString{String: *file.PathHash, Valid: true}
	}

	return gen.File{
		ID:           file.ID,
		WorkspaceID:  file.WorkspaceID,
		ParentID:     file.ParentID,
		ContentID:    file.ContentID,
		ContentKind:  int8(file.ContentType),
		Name:         file.Name,
		DisplayOrder: file.Order,
		PathHash:     pathHash,
		UpdatedAt:    file.UpdatedAt.Unix(),
	}
}

// ConvertToModelFile converts DB to model representation
func ConvertToModelFile(file gen.File) *mfile.File {
	var pathHash *string
	if file.PathHash.Valid {
		pathHash = &file.PathHash.String
	}

	return &mfile.File{
		ID:          file.ID,
		WorkspaceID: file.WorkspaceID,
		ParentID:    file.ParentID,
		ContentID:   file.ContentID,
		ContentType: mfile.ContentType(file.ContentKind),
		Name:        file.Name,
		Order:       file.DisplayOrder,
		PathHash:    pathHash,
		UpdatedAt:   time.Unix(file.UpdatedAt, 0),
	}
}
