package rworkspace

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/muser"
	"the-dev-tools/server/pkg/model/mworkspaceuser"

	workspacev1 "the-dev-tools/spec/dist/buf/go/workspace/v1"
)

type stubWorkspaceUserService struct {
	members []mworkspaceuser.WorkspaceUser
}

func (s *stubWorkspaceUserService) CreateWorkspaceUser(ctx context.Context, user *mworkspaceuser.WorkspaceUser) error {
	panic("unexpected call to CreateWorkspaceUser")
}

func (s *stubWorkspaceUserService) DeleteWorkspaceUser(ctx context.Context, id idwrap.IDWrap) error {
	panic("unexpected call to DeleteWorkspaceUser")
}

func (s *stubWorkspaceUserService) GetWorkspaceUserByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mworkspaceuser.WorkspaceUser, error) {
	return s.members, nil
}

func (s *stubWorkspaceUserService) GetWorkspaceUsersByWorkspaceIDAndUserID(ctx context.Context, workspaceID, userID idwrap.IDWrap) (*mworkspaceuser.WorkspaceUser, error) {
	panic("unexpected call to GetWorkspaceUsersByWorkspaceIDAndUserID")
}

func (s *stubWorkspaceUserService) UpdateWorkspaceUser(ctx context.Context, user *mworkspaceuser.WorkspaceUser) error {
	panic("unexpected call to UpdateWorkspaceUser")
}

type stubUserService struct {
	belongs bool
	users   map[string]*muser.User
}

func (s *stubUserService) CheckUserBelongsToWorkspace(ctx context.Context, userID, workspaceID idwrap.IDWrap) (bool, error) {
	return s.belongs, nil
}

func (s *stubUserService) GetUser(ctx context.Context, id idwrap.IDWrap) (*muser.User, error) {
	if user, ok := s.users[id.String()]; ok {
		return user, nil
	}
	return nil, errors.New("user not found")
}

func TestModelRoleToProto(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		role    mworkspaceuser.Role
		want    workspacev1.MemberRole
		wantErr bool
	}{
		{
			name:    "user maps to basic",
			role:    mworkspaceuser.RoleUser,
			want:    workspacev1.MemberRole_MEMBER_ROLE_BASIC,
			wantErr: false,
		},
		{
			name:    "admin maps to admin",
			role:    mworkspaceuser.RoleAdmin,
			want:    workspacev1.MemberRole_MEMBER_ROLE_ADMIN,
			wantErr: false,
		},
		{
			name:    "owner maps to owner",
			role:    mworkspaceuser.RoleOwner,
			want:    workspacev1.MemberRole_MEMBER_ROLE_OWNER,
			wantErr: false,
		},
		{
			name:    "unknown role errors and falls back",
			role:    mworkspaceuser.RoleUnknown,
			want:    workspaceMemberRoleFallback,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := modelRoleToProto(tc.role)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if got != tc.want {
					t.Fatalf("expected fallback role %v, got %v", tc.want, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected role: want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestProtoRoleToModel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		role    workspacev1.MemberRole
		want    mworkspaceuser.Role
		wantErr bool
	}{
		{
			name: "basic maps to user",
			role: workspacev1.MemberRole_MEMBER_ROLE_BASIC,
			want: mworkspaceuser.RoleUser,
		},
		{
			name: "admin maps to admin",
			role: workspacev1.MemberRole_MEMBER_ROLE_ADMIN,
			want: mworkspaceuser.RoleAdmin,
		},
		{
			name: "owner maps to owner",
			role: workspacev1.MemberRole_MEMBER_ROLE_OWNER,
			want: mworkspaceuser.RoleOwner,
		},
		{
			name:    "unspecified errors",
			role:    workspacev1.MemberRole_MEMBER_ROLE_UNSPECIFIED,
			want:    mworkspaceuser.RoleUnknown,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := protoRoleToModel(tc.role)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if got != tc.want {
					t.Fatalf("expected role %v, got %v", tc.want, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("unexpected role: want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestWorkspaceMemberUpdateRejectsInvalidRole(t *testing.T) {
	t.Parallel()

	svc := WorkspaceServiceRPC{}

	workspaceID := idwrap.NewNow()
	targetUserID := idwrap.NewNow()
	ctx := mwauth.CreateAuthedContext(context.Background(), idwrap.NewNow())

	role := workspaceMemberRoleFallback
	req := connect.NewRequest(&workspacev1.WorkspaceMemberUpdateRequest{
		WorkspaceId: workspaceID.Bytes(),
		UserID:      targetUserID.Bytes(),
		Role:        &role,
	})

	_, err := svc.WorkspaceMemberUpdate(ctx, req)
	if err == nil {
		t.Fatalf("expected error for invalid role, got nil")
	}

	if code := connect.CodeOf(err); code != connect.CodeInvalidArgument {
		t.Fatalf("expected invalid argument, got %v", code)
	}
}

func TestWorkspaceMemberListFallsBackOnInvalidRole(t *testing.T) {
	t.Parallel()

	workspaceID := idwrap.NewNow()
	memberRecordID := idwrap.NewNow()
	targetUserID := idwrap.NewNow()
	actorID := idwrap.NewNow()

	svc := WorkspaceServiceRPC{
		wus: &stubWorkspaceUserService{
			members: []mworkspaceuser.WorkspaceUser{
				{
					ID:          memberRecordID,
					WorkspaceID: workspaceID,
					UserID:      targetUserID,
					Role:        mworkspaceuser.Role(99),
				},
			},
		},
		us: &stubUserService{
			belongs: true,
			users: map[string]*muser.User{
				memberRecordID.String(): {
					Email: "user@example.com",
				},
			},
		},
	}

	ctx := mwauth.CreateAuthedContext(context.Background(), actorID)
	req := connect.NewRequest(&workspacev1.WorkspaceMemberListRequest{
		WorkspaceId: workspaceID.Bytes(),
	})

	resp, err := svc.WorkspaceMemberList(ctx, req)
	if err != nil {
		t.Fatalf("WorkspaceMemberList() error = %v", err)
	}

	if len(resp.Msg.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Msg.Items))
	}

	if resp.Msg.Items[0].Role != workspaceMemberRoleFallback {
		t.Fatalf("expected fallback role %v, got %v", workspaceMemberRoleFallback, resp.Msg.Items[0].Role)
	}
}
