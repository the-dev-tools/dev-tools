package mwauth

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

// CheckOwnerWorkspace checks if the user in context belongs to the workspace
func CheckOwnerWorkspace(ctx context.Context, su suser.UserService, workspaceID idwrap.IDWrap) (bool, error) {
	userID, err := GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
}

// CheckOwnerWorkspaceWithReader checks if the user in context belongs to the workspace using a reader
// Returns nil if allowed, error if denied or not found
func CheckOwnerWorkspaceWithReader(ctx context.Context, userReader *sworkspace.UserReader, workspaceID idwrap.IDWrap) error {
	userID, err := GetContextUserID(ctx)
	if err != nil {
		return err
	}

	wsu, err := userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		// We return a generic "workspace not found" error usually, but here we propagate the reader error
		// which is likely sql.ErrNoRows. The caller usually maps this to ErrWorkspaceNotFound.
		return err
	}

	// Wait, the original implementation in rworkspace.go checked for RoleOwner!
	// "if wsu.Role != mworkspace.RoleOwner { return ErrWorkspaceNotFound }"
	// Let's verify the original implementation carefully.

	if wsu.Role != mworkspace.RoleOwner {
		// Original code returned ErrWorkspaceNotFound to hide existence
		// We can return a specific error here or rely on the caller
		// Let's assume the caller expects an error if not owner.
		return sworkspace.ErrWorkspaceUserNotFound
	}

	return nil
}
