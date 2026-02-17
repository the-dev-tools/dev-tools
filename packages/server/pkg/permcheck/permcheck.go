//nolint:revive // exported
package permcheck

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

func CheckPerm(ok bool, error error) *connect.Error {
	if error != nil {
		// If error is already a connect.Error, preserve it
		var connectErr *connect.Error
		if errors.As(error, &connectErr) {
			return connectErr
		}
		return connect.NewError(connect.CodeInternal, error)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, nil)
	}
	return nil
}

// CheckWorkspaceReadAccess verifies if user has read access to workspace (any role >= User)
func CheckWorkspaceReadAccess(ctx context.Context, userReader *sworkspace.UserReader, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	if wsUser.Role < mworkspace.RoleUser {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// CheckWorkspaceWriteAccess verifies if user has write access to workspace (Admin or Owner)
func CheckWorkspaceWriteAccess(ctx context.Context, userReader *sworkspace.UserReader, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	if wsUser.Role < mworkspace.RoleAdmin {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}

// CheckWorkspaceDeleteAccess verifies if user has delete access to workspace (Admin or Owner)
func CheckWorkspaceDeleteAccess(ctx context.Context, userReader *sworkspace.UserReader, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	if wsUser.Role < mworkspace.RoleAdmin {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}

	return nil
}
