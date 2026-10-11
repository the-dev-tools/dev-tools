// Package scloudlink stores links between Files rows and Stresseur Cloud repos and flow files.
package scloudlink

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcloudlink"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

// ErrNotFound is returned when no link matches.
var ErrNotFound = errors.New("cloud link not found")

// Reader reads cloud links. It uses the connection pool and never a transaction.
type Reader struct {
	queries *gen.Queries
}

// NewReader creates a Reader on the connection pool.
func NewReader(db *sql.DB) *Reader {
	return &Reader{queries: gen.New(db)}
}

// NewReaderFromQueries creates a Reader from existing queries.
func NewReaderFromQueries(queries *gen.Queries) *Reader {
	return &Reader{queries: queries}
}

// Get returns the link of a Files row.
func (r *Reader) Get(ctx context.Context, fileID idwrap.IDWrap) (mcloudlink.CloudLink, error) {
	item, err := r.queries.GetCloudLink(ctx, fileID)
	if err != nil {
		return mcloudlink.CloudLink{}, notFound(err)
	}
	return ConvertDBToModel(item), nil
}

// GetByTarget returns the link of a repo (empty path) or flow file.
func (r *Reader) GetByTarget(ctx context.Context, t mcloudlink.Target) (mcloudlink.CloudLink, error) {
	item, err := r.queries.GetCloudLinkByTarget(ctx, gen.GetCloudLinkByTargetParams{
		WorkspaceID:        t.WorkspaceID,
		ApiUrl:             t.APIURL,
		CloudWorkspaceSlug: t.CloudWorkspaceSlug,
		RepoSlug:           t.RepoSlug,
		Path:               t.Path,
	})
	if err != nil {
		return mcloudlink.CloudLink{}, notFound(err)
	}
	return ConvertDBToModel(item), nil
}

// ListByWorkspace returns every link in a workspace.
func (r *Reader) ListByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mcloudlink.CloudLink, error) {
	items, err := r.queries.GetCloudLinksByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}

// ListByRepo returns a repo's link and the links of its flow files.
func (r *Reader) ListByRepo(ctx context.Context, t mcloudlink.Target) ([]mcloudlink.CloudLink, error) {
	items, err := r.queries.GetCloudLinksByRepo(ctx, gen.GetCloudLinksByRepoParams{
		WorkspaceID:        t.WorkspaceID,
		ApiUrl:             t.APIURL,
		CloudWorkspaceSlug: t.CloudWorkspaceSlug,
		RepoSlug:           t.RepoSlug,
	})
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
