package sworkspacesusers

import (
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mworkspaceuser"
)

func ConvertToDBWorkspaceUser(wsuser mworkspaceuser.WorkspaceUser) gen.WorkspacesUser {
	return gen.WorkspacesUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        int8(wsuser.Role), // nolint:gosec // G115
	}
}

func ConvertToModelWorkspaceUser(wsuser gen.WorkspacesUser) mworkspaceuser.WorkspaceUser {
	return mworkspaceuser.WorkspaceUser{
		ID:          wsuser.ID,
		WorkspaceID: wsuser.WorkspaceID,
		UserID:      wsuser.UserID,
		Role:        mworkspaceuser.Role(wsuser.Role), // nolint:gosec // G115
	}
}
