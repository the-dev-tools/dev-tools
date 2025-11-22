package shttp

import (
	"context"
	"fmt"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
)

// CreateHttpVersion creates a new HTTP version
func (hs HTTPService) CreateHttpVersion(ctx context.Context, httpID, createdBy idwrap.IDWrap, versionName, versionDescription string) (*gen.HttpVersion, error) {
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

	return &version, nil
}
