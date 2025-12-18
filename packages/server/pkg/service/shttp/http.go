//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
)

var ErrNoHTTPFound = sql.ErrNoRows

type HTTPService struct {
	reader  *Reader
	queries *gen.Queries
	logger  *slog.Logger
	wus     *sworkspacesusers.WorkspaceUserService
}

func New(queries *gen.Queries, logger *slog.Logger) HTTPService {
	return HTTPService{
		reader:  NewReaderFromQueries(queries, logger, nil),
		queries: queries,
		logger:  logger,
		wus:     nil,
	}
}

func NewWithWorkspaceUserService(queries *gen.Queries, logger *slog.Logger, wus *sworkspacesusers.WorkspaceUserService) HTTPService {
	return HTTPService{
		reader:  NewReaderFromQueries(queries, logger, wus),
		queries: queries,
		logger:  logger,
		wus:     wus,
	}
}

func (hs HTTPService) TX(tx *sql.Tx) HTTPService {
	var wus *sworkspacesusers.WorkspaceUserService
	if hs.wus != nil {
		wusTx := hs.wus.TX(tx)
		wus = &wusTx
	}
	newQueries := hs.queries.WithTx(tx)
	return HTTPService{
		reader:  NewReaderFromQueries(newQueries, hs.logger, wus),
		queries: newQueries,
		logger:  hs.logger,
		wus:     wus,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HTTPService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}

	service := HTTPService{
		reader:  NewReaderFromQueries(queries, nil, nil),
		queries: queries,
		logger:  nil, // Logger should be provided by caller if needed
	}
	return &service, nil
}

func (hs HTTPService) Create(ctx context.Context, http *mhttp.HTTP) error {
	return NewWriterFromQueries(hs.queries).Create(ctx, http)
}

func (hs HTTPService) Upsert(ctx context.Context, http *mhttp.HTTP) error {
	return NewWriterFromQueries(hs.queries).Upsert(ctx, http)
}

func (hs HTTPService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	return hs.reader.GetByWorkspaceID(ctx, workspaceID)
}

func (hs HTTPService) GetDeltasByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	return hs.reader.GetDeltasByWorkspaceID(ctx, workspaceID)
}

func (hs HTTPService) GetDeltasByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	return hs.reader.GetDeltasByParentID(ctx, parentID)
}

func (hs HTTPService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTP, error) {
	return hs.reader.Get(ctx, id)
}

func (hs HTTPService) Update(ctx context.Context, http *mhttp.HTTP) error {
	return NewWriterFromQueries(hs.queries).Update(ctx, http)
}

func (hs HTTPService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(hs.queries).Delete(ctx, id)
}

func (hs HTTPService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	return hs.reader.GetWorkspaceID(ctx, id)
}

func (hs HTTPService) CheckUserBelongsToHttp(ctx context.Context, httpID, userID idwrap.IDWrap) (bool, error) {
	return hs.reader.CheckUserBelongsToHttp(ctx, httpID, userID)
}

func (hs HTTPService) FindByURLAndMethod(ctx context.Context, workspaceID idwrap.IDWrap, url, method string) (*mhttp.HTTP, error) {
	return hs.reader.FindByURLAndMethod(ctx, workspaceID, url, method)
}

// HttpVersion methods delegating to Reader/Writer

func (hs HTTPService) CreateHttpVersion(ctx context.Context, httpID, createdBy idwrap.IDWrap, versionName, versionDescription string) (*mhttp.HttpVersion, error) {
	return NewWriterFromQueries(hs.queries).CreateHttpVersion(ctx, httpID, createdBy, versionName, versionDescription)
}

func (hs HTTPService) GetHttpVersionsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HttpVersion, error) {
	return hs.reader.GetHttpVersionsByHttpID(ctx, httpID)
}

func (hs HTTPService) Reader() *Reader { return hs.reader }
