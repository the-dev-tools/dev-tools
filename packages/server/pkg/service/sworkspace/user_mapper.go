package sworkspace

import (
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
)

func ConvertToDBWorkspaceUser(wsuser mworkspace.WorkspaceUser) gen.WorkspacesUser {
	return gen.WorkspacesUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        int8(wsuser.Role), // nolint:gosec // G115
	}
}

func ConvertToModelWorkspaceUser(wsuser gen.WorkspacesUser) mworkspace.WorkspaceUser {
	return mworkspace.WorkspaceUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        mworkspace.Role(wsuser.Role), // nolint:gosec // G115
	}
}
