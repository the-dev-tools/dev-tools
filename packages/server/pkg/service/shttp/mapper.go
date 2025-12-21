package shttp

import (
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mhttp"
)

func ConvertToDBHTTP(http mhttp.HTTP) gen.Http {
	var deltaBodyKind interface{}
	if http.DeltaBodyKind != nil {
		deltaBodyKind = int64(*http.DeltaBodyKind)
	}

	var lastRunAt interface{}
	if http.LastRunAt != nil {
		lastRunAt = *http.LastRunAt
	}

	var contentHash sql.NullString
	if http.ContentHash != nil {
		contentHash = sql.NullString{String: *http.ContentHash, Valid: true}
	}

	return gen.Http{
		ID:               http.ID,
		WorkspaceID:      http.WorkspaceID,
		FolderID:         http.FolderID,
		Name:             http.Name,
		Url:              http.Url,
		Method:           http.Method,
		BodyKind:         int8(http.BodyKind),
		Description:      http.Description,
		ContentHash:      contentHash,
		ParentHttpID:     http.ParentHttpID,
		IsDelta:          http.IsDelta,
		DeltaName:        http.DeltaName,
		DeltaUrl:         http.DeltaUrl,
		DeltaMethod:      http.DeltaMethod,
		DeltaBodyKind:    deltaBodyKind,
		DeltaDescription: http.DeltaDescription,
		LastRunAt:        lastRunAt,
		CreatedAt:        http.CreatedAt,
		UpdatedAt:        http.UpdatedAt,
	}
}

func interfaceToInt8Ptr(v interface{}) *mhttp.HttpBodyKind {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case int64:
		k := mhttp.HttpBodyKind(val) // nolint:gosec // G115
		return &k
	case int32:
		k := mhttp.HttpBodyKind(val) // nolint:gosec // G115
		return &k
	case int:
		k := mhttp.HttpBodyKind(val) // nolint:gosec // G115
		return &k
	case int8:
		k := mhttp.HttpBodyKind(val)
		return &k
	default:
		return nil
	}
}

func interfaceToInt64Ptr(v interface{}) *int64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case int64:
		return &val
	case int:
		i := int64(val)
		return &i
	default:
		return nil
	}
}

func ConvertToModelHTTP(http gen.Http) *mhttp.HTTP {
	var contentHash *string
	if http.ContentHash.Valid {
		contentHash = &http.ContentHash.String
	}

	return &mhttp.HTTP{
		ID:               http.ID,
		WorkspaceID:      http.WorkspaceID,
		FolderID:         http.FolderID,
		Name:             http.Name,
		Url:              http.Url,
		Method:           http.Method,
		BodyKind:         mhttp.HttpBodyKind(http.BodyKind),
		Description:      http.Description,
		ContentHash:      contentHash,
		ParentHttpID:     http.ParentHttpID,
		IsDelta:          http.IsDelta,
		DeltaName:        http.DeltaName,
		DeltaUrl:         http.DeltaUrl,
		DeltaMethod:      http.DeltaMethod,
		DeltaBodyKind:    interfaceToInt8Ptr(http.DeltaBodyKind),
		DeltaDescription: http.DeltaDescription,
		LastRunAt:        interfaceToInt64Ptr(http.LastRunAt),
		CreatedAt:        http.CreatedAt,
		UpdatedAt:        http.UpdatedAt,
	}
}

// ConvertToModelHttpVersion converts DB HttpVersion to model HttpVersion
func ConvertToModelHttpVersion(version gen.HttpVersion) *mhttp.HttpVersion {
	return &mhttp.HttpVersion{
		ID:                 version.ID,
		HttpID:             version.HttpID,
		VersionName:        version.VersionName,
		VersionDescription: version.VersionDescription,
		IsActive:           version.IsActive,
		CreatedAt:          version.CreatedAt,
		CreatedBy:          version.CreatedBy,
	}
}
