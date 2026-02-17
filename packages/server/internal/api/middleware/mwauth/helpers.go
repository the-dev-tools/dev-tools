package mwauth

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
)

// CheckOwnerWorkspace checks if the user in context belongs to the workspace
func CheckOwnerWorkspace(ctx context.Context, su suser.UserService, workspaceID idwrap.IDWrap) (bool, error) {
	userID, err := GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
}

