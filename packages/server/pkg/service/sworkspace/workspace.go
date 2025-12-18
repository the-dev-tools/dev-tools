//nolint:revive // exported
package sworkspace

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mworkspace"
)

var ErrNoWorkspaceFound = sql.ErrNoRows

type WorkspaceService struct {
	reader  *WorkspaceReader
	queries *gen.Queries
}

func NewWorkspaceService(queries *gen.Queries) WorkspaceService {
	return WorkspaceService{
		reader:  NewWorkspaceReaderFromQueries(queries),
		queries: queries,
	}
}

func (ws WorkspaceService) TX(tx *sql.Tx) WorkspaceService {
	// Create new instances with transaction support
	txQueries := ws.queries.WithTx(tx)

	return WorkspaceService{
		reader:  NewWorkspaceReaderFromQueries(txQueries),
		queries: txQueries,
	}
}

func NewWorkspaceServiceTX(ctx context.Context, tx *sql.Tx) (*WorkspaceService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoWorkspaceFound
		}
		return nil, err
	}

	return &WorkspaceService{
		reader:  NewWorkspaceReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (ws WorkspaceService) Create(ctx context.Context, w *mworkspace.Workspace) error {
	return NewWorkspaceWriterFromQueries(ws.queries).Create(ctx, w)
}

func (ws WorkspaceService) Get(ctx context.Context, id idwrap.IDWrap) (*mworkspace.Workspace, error) {
	return ws.reader.Get(ctx, id)
}

func (ws WorkspaceService) Update(ctx context.Context, org *mworkspace.Workspace) error {
	return NewWorkspaceWriterFromQueries(ws.queries).Update(ctx, org)
}

func (ws WorkspaceService) UpdateUpdatedTime(ctx context.Context, org *mworkspace.Workspace) error {
	return NewWorkspaceWriterFromQueries(ws.queries).UpdateUpdatedTime(ctx, org)
}

func (ws WorkspaceService) Delete(ctx context.Context, userID, id idwrap.IDWrap) error {
	return NewWorkspaceWriterFromQueries(ws.queries).Delete(ctx, id)
}

func (ws WorkspaceService) GetMultiByUserID(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	return ws.reader.GetMultiByUserID(ctx, userID)
}

func (ws WorkspaceService) GetByIDandUserID(ctx context.Context, orgID, userID idwrap.IDWrap) (*mworkspace.Workspace, error) {
	return ws.reader.GetByIDandUserID(ctx, orgID, userID)
}

// GetWorkspacesByUserIDOrdered returns workspaces for a user in their proper order
func (ws WorkspaceService) GetWorkspacesByUserIDOrdered(ctx context.Context, userID idwrap.IDWrap) ([]mworkspace.Workspace, error) {
	return ws.reader.GetWorkspacesByUserIDOrdered(ctx, userID)
}

func (ws WorkspaceService) Reader() *WorkspaceReader { return ws.reader }
