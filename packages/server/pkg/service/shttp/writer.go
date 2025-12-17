package shttp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{
		queries: gen.New(tx),
	}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{
		queries: queries,
	}
}

func (w *Writer) Create(ctx context.Context, http *mhttp.HTTP) error {
	now := dbtime.DBNow()
	http.CreatedAt = now.Unix()
	http.UpdatedAt = now.Unix()

	dbHttp := ConvertToDBHTTP(*http)
	return w.queries.CreateHTTP(ctx, gen.CreateHTTPParams(dbHttp))
}

func (w *Writer) Update(ctx context.Context, http *mhttp.HTTP) error {
	http.UpdatedAt = dbtime.DBNow().Unix()

	dbHttp := ConvertToDBHTTP(*http)
	return w.queries.UpdateHTTP(ctx, gen.UpdateHTTPParams{
		ID:          dbHttp.ID,
		FolderID:    dbHttp.FolderID,
		Name:        dbHttp.Name,
		Url:         dbHttp.Url,
		Method:      dbHttp.Method,
		BodyKind:    dbHttp.BodyKind,
		Description: dbHttp.Description,
		LastRunAt:   dbHttp.LastRunAt,
	})
}

func (w *Writer) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteHTTP(ctx, id)
}

func (w *Writer) Upsert(ctx context.Context, http *mhttp.HTTP) error {
	existing, err := w.queries.GetHTTP(ctx, http.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return w.Create(ctx, http)
		}
		return err
	}

	// Preserve creation time from existing record if not set in new record
	if http.CreatedAt == 0 {
		http.CreatedAt = existing.CreatedAt
	}

	// Update fields
	return w.Update(ctx, http)
}

func (w *Writer) CreateHttpVersion(ctx context.Context, httpID, createdBy idwrap.IDWrap, versionName, versionDescription string) (*mhttp.HttpVersion, error) {
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

	err := w.queries.CreateHttpVersion(ctx, gen.CreateHttpVersionParams(version))
	if err != nil {
		return nil, fmt.Errorf("failed to create http version: %w", err)
	}

	return ConvertToModelHttpVersion(version), nil
}
