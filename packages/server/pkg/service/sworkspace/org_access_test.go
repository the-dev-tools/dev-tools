package sworkspace_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

type orgTestEnv struct {
	ctx        context.Context
	db         *sql.DB
	queries    *gen.Queries
	wsService  sworkspace.WorkspaceService
	wusService sworkspace.UserService
	usService  suser.UserService
}

func setupOrgTest(t *testing.T) *orgTestEnv {
	t.Helper()
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	queries := gen.New(db)
	return &orgTestEnv{
		ctx:        ctx,
		db:         db,
		queries:    queries,
		wsService:  sworkspace.NewWorkspaceService(queries),
		wusService: sworkspace.NewUserService(queries),
		usService:  suser.New(queries),
	}
}

func (e *orgTestEnv) createUser(t *testing.T) idwrap.IDWrap {
	t.Helper()
	userID := idwrap.NewNow()
	err := e.usService.CreateUser(e.ctx, &muser.User{
		ID:    userID,
		Email: "test-" + userID.String() + "@example.com",
	})
	require.NoError(t, err)
	return userID
}

func (e *orgTestEnv) createOrg(t *testing.T, name string) idwrap.IDWrap {
	t.Helper()
	orgID := idwrap.NewNow()
	err := e.queries.AuthCreateOrganization(e.ctx, gen.AuthCreateOrganizationParams{
		ID:        orgID,
		Name:      name,
		Slug:      sql.NullString{String: name + "-slug", Valid: true},
		CreatedAt: time.Now().Unix(),
	})
	require.NoError(t, err)
	return orgID
}

func (e *orgTestEnv) createMember(t *testing.T, userID, orgID idwrap.IDWrap, role string) {
	t.Helper()
	err := e.queries.AuthCreateMember(e.ctx, gen.AuthCreateMemberParams{
		ID:             idwrap.NewNow(),
		UserID:         userID,
		OrganizationID: orgID,
		Role:           role,
		CreatedAt:      time.Now().Unix(),
	})
	require.NoError(t, err)
}

func (e *orgTestEnv) createWorkspace(t *testing.T, name string, orgID *idwrap.IDWrap) idwrap.IDWrap {
	t.Helper()
	wsID := idwrap.NewNow()
	err := e.wsService.Create(e.ctx, &mworkspace.Workspace{
		ID:             wsID,
		Name:           name,
		Updated:        dbtime.DBNow(),
		OrganizationID: orgID,
	})
	require.NoError(t, err)
	return wsID
}

func (e *orgTestEnv) createWorkspaceWithUser(t *testing.T, name string, userID idwrap.IDWrap, orgID *idwrap.IDWrap) idwrap.IDWrap {
	t.Helper()
	wsID := e.createWorkspace(t, name, orgID)
	err := e.wusService.CreateWorkspaceUser(e.ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        mworkspace.RoleOwner,
	})
	require.NoError(t, err)
	return wsID
}

// --- CheckUserBelongsToWorkspace tests ---

func TestCheckUserBelongsToWorkspace_DirectMembership(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	wsID := e.createWorkspaceWithUser(t, "DirectWS", userID, nil)

	belongs, err := e.usService.CheckUserBelongsToWorkspace(e.ctx, userID, wsID)
	require.NoError(t, err)
	require.True(t, belongs)
}

func TestCheckUserBelongsToWorkspace_OrgMembership(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "member")

	// Workspace belongs to org, user has NO workspaces_users row
	wsID := e.createWorkspace(t, "OrgWS", &orgID)

	belongs, err := e.usService.CheckUserBelongsToWorkspace(e.ctx, userID, wsID)
	require.NoError(t, err)
	require.True(t, belongs)
}

func TestCheckUserBelongsToWorkspace_NoAccess(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	otherUser := e.createUser(t)

	// Workspace with direct access for otherUser only
	wsID := e.createWorkspaceWithUser(t, "OtherWS", otherUser, nil)

	belongs, err := e.usService.CheckUserBelongsToWorkspace(e.ctx, userID, wsID)
	require.NoError(t, err)
	require.False(t, belongs)
}

func TestCheckUserBelongsToWorkspace_OrgWorkspaceNonMember(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	// User is NOT a member of the org

	wsID := e.createWorkspace(t, "OrgWS", &orgID)

	belongs, err := e.usService.CheckUserBelongsToWorkspace(e.ctx, userID, wsID)
	require.NoError(t, err)
	require.False(t, belongs)
}

func TestCheckUserBelongsToWorkspace_NullOrgIDLocalMode(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)

	// Workspace with no org (local mode), no workspaces_users row
	wsID := e.createWorkspace(t, "LocalWS", nil)

	belongs, err := e.usService.CheckUserBelongsToWorkspace(e.ctx, userID, wsID)
	require.NoError(t, err)
	require.False(t, belongs, "no access without direct membership or org membership")
}

// --- GetWorkspaceUsersByWorkspaceIDAndUserID role mapping tests ---

func TestGetWorkspaceUser_OrgOwnerRole(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "owner")
	wsID := e.createWorkspace(t, "OrgWS", &orgID)

	wsUser, err := e.wusService.GetWorkspaceUsersByWorkspaceIDAndUserID(e.ctx, wsID, userID)
	require.NoError(t, err)
	require.Equal(t, mworkspace.RoleOwner, wsUser.Role)
	require.Equal(t, wsID, wsUser.WorkspaceID)
	require.Equal(t, userID, wsUser.UserID)
}

func TestGetWorkspaceUser_OrgAdminRole(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "admin")
	wsID := e.createWorkspace(t, "OrgWS", &orgID)

	wsUser, err := e.wusService.GetWorkspaceUsersByWorkspaceIDAndUserID(e.ctx, wsID, userID)
	require.NoError(t, err)
	require.Equal(t, mworkspace.RoleAdmin, wsUser.Role)
}

func TestGetWorkspaceUser_OrgMemberRole(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "member")
	wsID := e.createWorkspace(t, "OrgWS", &orgID)

	wsUser, err := e.wusService.GetWorkspaceUsersByWorkspaceIDAndUserID(e.ctx, wsID, userID)
	require.NoError(t, err)
	require.Equal(t, mworkspace.RoleUser, wsUser.Role)
}

func TestGetWorkspaceUser_DirectTakesPrecedence(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "member") // org role: member → RoleUser(1)

	// Create workspace with org AND direct membership (RoleAdmin)
	wsID := e.createWorkspace(t, "OrgWS", &orgID)
	err := e.wusService.CreateWorkspaceUser(e.ctx, &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        mworkspace.RoleAdmin,
	})
	require.NoError(t, err)

	wsUser, err := e.wusService.GetWorkspaceUsersByWorkspaceIDAndUserID(e.ctx, wsID, userID)
	require.NoError(t, err)
	require.Equal(t, mworkspace.RoleAdmin, wsUser.Role, "direct membership should take precedence over org fallback")
}

func TestGetWorkspaceUser_NotFound(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	wsID := e.createWorkspace(t, "NoAccessWS", nil)

	_, err := e.wusService.GetWorkspaceUsersByWorkspaceIDAndUserID(e.ctx, wsID, userID)
	require.ErrorIs(t, err, sworkspace.ErrWorkspaceUserNotFound)
}

// --- Workspace listing tests ---

func TestGetWorkspacesByUserIDOrdered_IncludesOrgWorkspaces(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "member")

	// Direct workspace
	directWS := e.createWorkspaceWithUser(t, "DirectWS", userID, nil)
	// Org workspace (no workspaces_users row)
	orgWS := e.createWorkspace(t, "OrgWS", &orgID)

	workspaces, err := e.wsService.GetWorkspacesByUserIDOrdered(e.ctx, userID)
	require.NoError(t, err)
	require.Len(t, workspaces, 2)

	ids := map[idwrap.IDWrap]bool{}
	for _, ws := range workspaces {
		ids[ws.ID] = true
	}
	require.True(t, ids[directWS], "direct workspace should be in list")
	require.True(t, ids[orgWS], "org workspace should be in list")
}

func TestGetWorkspacesByUserIDOrdered_NoDuplicates(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "owner")

	// Workspace with BOTH direct and org access
	wsID := e.createWorkspaceWithUser(t, "DualAccessWS", userID, &orgID)

	workspaces, err := e.wsService.GetWorkspacesByUserIDOrdered(e.ctx, userID)
	require.NoError(t, err)
	require.Len(t, workspaces, 1, "UNION should deduplicate")
	require.Equal(t, wsID, workspaces[0].ID)
}

func TestGetWorkspacesByUserIDOrdered_ExcludesOtherOrgWorkspaces(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	otherUser := e.createUser(t)

	orgID := e.createOrg(t, "OtherOrg")
	e.createMember(t, otherUser, orgID, "owner")
	// User is NOT in this org

	e.createWorkspace(t, "OtherOrgWS", &orgID)

	workspaces, err := e.wsService.GetWorkspacesByUserIDOrdered(e.ctx, userID)
	require.NoError(t, err)
	require.Len(t, workspaces, 0)
}

func TestGetAllWorkspacesByUserID_IncludesOrgWorkspaces(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "admin")

	e.createWorkspaceWithUser(t, "DirectWS", userID, nil)
	e.createWorkspace(t, "OrgWS", &orgID)

	workspaces, err := e.wsService.GetMultiByUserID(e.ctx, userID)
	require.NoError(t, err)
	require.Len(t, workspaces, 2)
}

func TestGetWorkspacesByUserIDOrdered_LocalModeNoOrgDependency(t *testing.T) {
	e := setupOrgTest(t)
	userID := e.createUser(t)

	// Local mode: no orgs, no auth_member rows, workspaces have nil org_id
	ws1 := e.createWorkspaceWithUser(t, "Local1", userID, nil)
	ws2 := e.createWorkspaceWithUser(t, "Local2", userID, nil)

	workspaces, err := e.wsService.GetWorkspacesByUserIDOrdered(e.ctx, userID)
	require.NoError(t, err)
	require.Len(t, workspaces, 2)

	ids := map[idwrap.IDWrap]bool{}
	for _, ws := range workspaces {
		ids[ws.ID] = true
	}
	require.True(t, ids[ws1])
	require.True(t, ids[ws2])
}

// --- OrganizationID field persistence test ---

func TestWorkspaceOrganizationID_Persisted(t *testing.T) {
	e := setupOrgTest(t)
	orgID := e.createOrg(t, "TestOrg")
	wsID := e.createWorkspace(t, "OrgWS", &orgID)

	ws, err := e.wsService.Get(e.ctx, wsID)
	require.NoError(t, err)
	require.NotNil(t, ws.OrganizationID)
	require.Equal(t, orgID, *ws.OrganizationID)
}

func TestWorkspaceOrganizationID_NilForLocalMode(t *testing.T) {
	e := setupOrgTest(t)
	wsID := e.createWorkspace(t, "LocalWS", nil)

	ws, err := e.wsService.Get(e.ctx, wsID)
	require.NoError(t, err)
	require.Nil(t, ws.OrganizationID)
}
