//nolint:revive // exported
package shttp

import (
	"context"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

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

// CreateHttpVersion creates a new HTTP version
func (hs HTTPService) CreateHttpVersion(ctx context.Context, httpID, createdBy idwrap.IDWrap, versionName, versionDescription string) (*mhttp.HttpVersion, error) {
	id := idwrap.NewNow()
	now := dbtime.DBNow().Unix()

	version := gen.HttpVersion{
		ID:                 id,
		HttpID:             httpID,
		VersionName:        versionName,
		VersionDescription: versionDescription,
		IsActive:           true,
		CreatedAt:          now,
		CreatedBy:          &createdBy,
	}

	err := hs.queries.CreateHttpVersion(ctx, gen.CreateHttpVersionParams{
		ID:                 version.ID,
		HttpID:             version.HttpID,
		VersionName:        version.VersionName,
		VersionDescription: version.VersionDescription,
		IsActive:           version.IsActive,
		CreatedAt:          version.CreatedAt,
		CreatedBy:          version.CreatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create http version: %w", err)
	}

	return ConvertToModelHttpVersion(version), nil
}

// GetHttpVersionsByHttpID retrieves all versions for a specific HTTP entry
func (hs HTTPService) GetHttpVersionsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HttpVersion, error) {
	versions, err := hs.queries.GetHttpVersionsByHttpID(ctx, httpID)
	if err != nil {
		return nil, fmt.Errorf("failed to get http versions: %w", err)
	}

	result := make([]mhttp.HttpVersion, len(versions))
	for i, version := range versions {
		result[i] = *ConvertToModelHttpVersion(version)
	}
	return result, nil
}