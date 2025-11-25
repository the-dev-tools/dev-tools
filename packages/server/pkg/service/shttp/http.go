package shttp

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

var ErrNoHTTPFound = sql.ErrNoRows

type HTTPService struct {
	queries *gen.Queries
	logger  *slog.Logger
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
	}
}

func (hs HTTPService) TX(tx *sql.Tx) HTTPService {
	return HTTPService{
		queries: hs.queries.WithTx(tx),
		logger:  hs.logger,
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
	return hs.queries.CreateHTTP(ctx, gen.CreateHTTPParams{
		ID:               dbHttp.ID,
		WorkspaceID:      dbHttp.WorkspaceID,
		FolderID:         dbHttp.FolderID,
		Name:             dbHttp.Name,
		Url:              dbHttp.Url,
		Method:           dbHttp.Method,
		BodyKind:         dbHttp.BodyKind,
		Description:      dbHttp.Description,
		ParentHttpID:     dbHttp.ParentHttpID,
		IsDelta:          dbHttp.IsDelta,
		DeltaName:        dbHttp.DeltaName,
		DeltaUrl:         dbHttp.DeltaUrl,
		DeltaMethod:      dbHttp.DeltaMethod,
		DeltaBodyKind:    dbHttp.DeltaBodyKind,
		DeltaDescription: dbHttp.DeltaDescription,
		LastRunAt:        dbHttp.LastRunAt,
		CreatedAt:        dbHttp.CreatedAt,
		UpdatedAt:        dbHttp.UpdatedAt,
	})
}

func (hs HTTPService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	https, err := hs.queries.GetHTTPsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
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

func (hs HTTPService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTP, error) {
	http, err := hs.queries.GetHTTP(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
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
		if err == sql.ErrNoRows {
			return idwrap.IDWrap{}, ErrNoHTTPFound
		}
		return idwrap.IDWrap{}, err
	}
	return workspaceID, nil
}

func (hs HTTPService) CheckUserBelongsToHttp(ctx context.Context, httpID, userID idwrap.IDWrap) (bool, error) {
	workspaceID, err := hs.GetWorkspaceID(ctx, httpID)
	if err != nil {
		if err == ErrNoHTTPFound {
			return false, nil
		}
		return false, err
	}

	// Check if user belongs to workspace
	// This would typically use a workspace user service
	// For now, we'll return a placeholder implementation
	// In a real implementation, you'd inject a workspace service or use a permission checker
	_ = workspaceID
	_ = userID

	// TODO: Implement proper workspace permission checking
	// This should check if the user has access to the workspace
	return true, nil
}
