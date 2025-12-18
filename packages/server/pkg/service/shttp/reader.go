package shttp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
)

type Reader struct {
	queries *gen.Queries
	logger  *slog.Logger
	wus     *sworkspacesusers.WorkspaceUserService
}

func NewReader(db *sql.DB, logger *slog.Logger, wus *sworkspacesusers.WorkspaceUserService) *Reader {
	return &Reader{
		queries: gen.New(db),
		logger:  logger,
		wus:     wus,
	}
}

func NewReaderFromQueries(queries *gen.Queries, logger *slog.Logger, wus *sworkspacesusers.WorkspaceUserService) *Reader {
	return &Reader{
		queries: queries,
		logger:  logger,
		wus:     wus,
	}
}

func (r *Reader) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTP, error) {
	http, err := r.queries.GetHTTP(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if r.logger != nil {
				r.logger.DebugContext(ctx, fmt.Sprintf("HTTP ID: %s not found", id.String()))
			}
			return nil, ErrNoHTTPFound
		}
		return nil, err
	}
	return ConvertToModelHTTP(http), nil
}

func (r *Reader) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	https, err := r.queries.GetHTTPsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if r.logger != nil {
				r.logger.InfoContext(ctx, fmt.Sprintf("workspaceID: %s has no HTTP entries", workspaceID.String()))
			}
			return []mhttp.HTTP{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTP, len(https))
	for i, http := range https {
		result[i] = *ConvertToModelHTTP(http)
	}
	return result, nil
}

func (r *Reader) GetDeltasByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	https, err := r.queries.GetHTTPDeltasByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTP{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTP, len(https))
	for i, http := range https {
		result[i] = *ConvertToModelHTTP(http)
	}
	return result, nil
}

func (r *Reader) GetDeltasByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	https, err := r.queries.GetHTTPDeltasByParentID(ctx, &parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTP{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTP, len(https))
	for i, h := range https {
		result[i] = mhttp.HTTP{
			ID:               h.ID,
			WorkspaceID:      h.WorkspaceID,
			FolderID:         h.FolderID,
			Name:             h.Name,
			Url:              h.Url,
			Method:           h.Method,
			BodyKind:         mhttp.HttpBodyKind(h.BodyKind),
			Description:      h.Description,
			ParentHttpID:     h.ParentHttpID,
			IsDelta:          h.IsDelta,
			DeltaName:        h.DeltaName,
			DeltaUrl:         h.DeltaUrl,
			DeltaMethod:      h.DeltaMethod,
			DeltaBodyKind:    interfaceToInt8Ptr(h.DeltaBodyKind),
			DeltaDescription: h.DeltaDescription,
			CreatedAt:        h.CreatedAt,
			UpdatedAt:        h.UpdatedAt,
		}
	}
	return result, nil
}

func (r *Reader) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	workspaceID, err := r.queries.GetHTTPWorkspaceID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrNoHTTPFound
		}
		return idwrap.IDWrap{}, err
	}
	return workspaceID, nil
}

func (r *Reader) FindByURLAndMethod(ctx context.Context, workspaceID idwrap.IDWrap, url, method string) (*mhttp.HTTP, error) {
	http, err := r.queries.FindHTTPByURLAndMethod(ctx, gen.FindHTTPByURLAndMethodParams{
		WorkspaceID: workspaceID,
		Url:         url,
		Method:      method,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHTTPFound
		}
		return nil, err
	}
	return ConvertToModelHTTP(http), nil
}

func (r *Reader) CheckUserBelongsToHttp(ctx context.Context, httpID, userID idwrap.IDWrap) (bool, error) {
	workspaceID, err := r.GetWorkspaceID(ctx, httpID)
	if err != nil {
		if errors.Is(err, ErrNoHTTPFound) {
			return false, nil
		}
		return false, err
	}

	if r.wus == nil {
		return false, errors.New("workspace user service not configured")
	}

	wsUser, err := r.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return false, nil
		}
		return false, err
	}

	return wsUser.Role >= mworkspace.RoleUser, nil
}

func (r *Reader) GetHttpVersionsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HttpVersion, error) {
	versions, err := r.queries.GetHttpVersionsByHttpID(ctx, httpID)
	if err != nil {
		return nil, fmt.Errorf("failed to get http versions: %w", err)
	}

	result := make([]mhttp.HttpVersion, len(versions))
	for i, version := range versions {
		result[i] = *ConvertToModelHttpVersion(version)
	}
	return result, nil
}
