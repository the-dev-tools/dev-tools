//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
	"the-dev-tools/server/pkg/service/sworkspacesusers"
)

var ErrNoHTTPFound = sql.ErrNoRows

type HTTPService struct {
	queries *gen.Queries
	logger  *slog.Logger
	wus     *sworkspacesusers.WorkspaceUserService
}

func ConvertToDBHTTP(http mhttp.HTTP) gen.Http {
	var deltaBodyKind interface{}
	if http.DeltaBodyKind != nil {
		deltaBodyKind = int64(*http.DeltaBodyKind)
	}

	var lastRunAt interface{}
	if http.LastRunAt != nil {
		lastRunAt = *http.LastRunAt
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
	return &mhttp.HTTP{
		ID:               http.ID,
		WorkspaceID:      http.WorkspaceID,
		FolderID:         http.FolderID,
		Name:             http.Name,
		Url:              http.Url,
		Method:           http.Method,
		BodyKind:         mhttp.HttpBodyKind(http.BodyKind),
		Description:      http.Description,
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

func New(queries *gen.Queries, logger *slog.Logger) HTTPService {
	return HTTPService{
		queries: queries,
		logger:  logger,
		wus:     nil,
	}
}

func NewWithWorkspaceUserService(queries *gen.Queries, logger *slog.Logger, wus *sworkspacesusers.WorkspaceUserService) HTTPService {
	return HTTPService{
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
	return HTTPService{
		queries: hs.queries.WithTx(tx),
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
		queries: queries,
		logger:  nil, // Logger should be provided by caller if needed
	}
	return &service, nil
}

func (hs HTTPService) Create(ctx context.Context, http *mhttp.HTTP) error {
	now := dbtime.DBNow()
	http.CreatedAt = now.Unix()
	http.UpdatedAt = now.Unix()

	dbHttp := ConvertToDBHTTP(*http)
	return hs.queries.CreateHTTP(ctx, gen.CreateHTTPParams(dbHttp))
}

func (hs HTTPService) Upsert(ctx context.Context, http *mhttp.HTTP) error {
	existing, err := hs.Get(ctx, http.ID)
	if err != nil {
		if errors.Is(err, ErrNoHTTPFound) {
			return hs.Create(ctx, http)
		}
		return err
	}

	// Preserve creation time from existing record if not set in new record (though usually import sets it)
	if http.CreatedAt == 0 {
		http.CreatedAt = existing.CreatedAt
	}

	// Update fields
	return hs.Update(ctx, http)
}

func (hs HTTPService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	https, err := hs.queries.GetHTTPsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			hs.logger.InfoContext(ctx, fmt.Sprintf("workspaceID: %s has no HTTP entries", workspaceID.String()))
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

func (hs HTTPService) GetDeltasByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	https, err := hs.queries.GetHTTPDeltasByWorkspaceID(ctx, workspaceID)
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

func (hs HTTPService) GetDeltasByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	https, err := hs.queries.GetHTTPDeltasByParentID(ctx, &parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTP{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTP, len(https))
	for i, h := range https {
		// Manual mapping from GetHTTPDeltasByParentIDRow to mhttp.HTTP
		// Assuming the query returns all columns
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
			// LastRunAt not available in this query view?
			CreatedAt: h.CreatedAt,
			UpdatedAt: h.UpdatedAt,
		}
	}
	return result, nil
}

func (hs HTTPService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTP, error) {
	http, err := hs.queries.GetHTTP(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			hs.logger.DebugContext(ctx, fmt.Sprintf("HTTP ID: %s not found", id.String()))
			return nil, ErrNoHTTPFound
		}
		return nil, err
	}
	return ConvertToModelHTTP(http), nil
}

func (hs HTTPService) Update(ctx context.Context, http *mhttp.HTTP) error {
	http.UpdatedAt = dbtime.DBNow().Unix()

	dbHttp := ConvertToDBHTTP(*http)
	return hs.queries.UpdateHTTP(ctx, gen.UpdateHTTPParams{
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

func (hs HTTPService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return hs.queries.DeleteHTTP(ctx, id)
}

func (hs HTTPService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	workspaceID, err := hs.queries.GetHTTPWorkspaceID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrNoHTTPFound
		}
		return idwrap.IDWrap{}, err
	}
	return workspaceID, nil
}

func (hs HTTPService) CheckUserBelongsToHttp(ctx context.Context, httpID, userID idwrap.IDWrap) (bool, error) {
	workspaceID, err := hs.GetWorkspaceID(ctx, httpID)
	if err != nil {
		if errors.Is(err, ErrNoHTTPFound) {
			return false, nil
		}
		return false, err
	}

	// If workspace user service is not available, cannot verify membership
	if hs.wus == nil {
		return false, errors.New("workspace user service not configured")
	}

	// Check if user belongs to workspace
	wsUser, err := hs.wus.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspacesusers.ErrWorkspaceUserNotFound) {
			return false, nil
		}
		return false, err
	}

	// User must have at least RoleUser to access the HTTP resource
	return wsUser.Role >= mworkspaceuser.RoleUser, nil
}

func (hs HTTPService) FindByURLAndMethod(ctx context.Context, workspaceID idwrap.IDWrap, url, method string) (*mhttp.HTTP, error) {
	http, err := hs.queries.FindHTTPByURLAndMethod(ctx, gen.FindHTTPByURLAndMethodParams{
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
