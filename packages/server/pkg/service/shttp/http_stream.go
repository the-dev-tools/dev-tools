package shttp

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// HTTPStreamService keeps HTTP requests' stream: settings.
type HTTPStreamService struct {
	queries *gen.Queries
}

// NewHTTPStreamService returns the service over queries.
func NewHTTPStreamService(queries *gen.Queries) HTTPStreamService {
	return HTTPStreamService{queries: queries}
}

// TX binds the service to a transaction.
func (s HTTPStreamService) TX(tx *sql.Tx) HTTPStreamService {
	return HTTPStreamService{queries: s.queries.WithTx(tx)}
}

func streamFromDB(r gen.HttpStream) mhttp.HTTPStream {
	return mhttp.HTTPStream{HttpID: r.HttpID, Preset: r.Preset, TimeoutMs: r.TimeoutMs}
}

// Get returns an HTTP request's stream settings, nil when it has none.
func (s HTTPStreamService) Get(ctx context.Context, httpID idwrap.IDWrap) (*mhttp.HTTPStream, error) {
	r, err := s.queries.GetHTTPStream(ctx, httpID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := streamFromDB(r)
	return &out, nil
}

// GetByWorkspaceID returns the stream settings of every HTTP request in a workspace.
func (s HTTPStreamService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTPStream, error) {
	rows, err := s.queries.GetHTTPStreamsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]mhttp.HTTPStream, 0, len(rows))
	for _, r := range rows {
		out = append(out, streamFromDB(r))
	}
	return out, nil
}

// Upsert stores an HTTP request's stream settings.
func (s HTTPStreamService) Upsert(ctx context.Context, st mhttp.HTTPStream) error {
	return s.queries.UpsertHTTPStream(ctx, gen.UpsertHTTPStreamParams{HttpID: st.HttpID, Preset: st.Preset, TimeoutMs: st.TimeoutMs})
}

// Delete removes an HTTP request's stream settings.
func (s HTTPStreamService) Delete(ctx context.Context, httpID idwrap.IDWrap) error {
	return s.queries.DeleteHTTPStream(ctx, httpID)
}
