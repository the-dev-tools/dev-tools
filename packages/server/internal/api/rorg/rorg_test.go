package rorg_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rorg"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sorg"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	v1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/organization/v1"
)

type testEnv struct {
	ctx     context.Context
	db      *sql.DB
	queries *gen.Queries
	handler rorg.OrgServiceRPC
}

func setupTest(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	queries := gen.New(db)
	orgService := sorg.New(db)
	handler := rorg.New(rorg.OrgServiceRPCDeps{
		Reader: orgService.Reader(),
	})

	return &testEnv{
		ctx:     ctx,
		db:      db,
		queries: queries,
		handler: handler,
	}
}

func (e *testEnv) createUser(t *testing.T) idwrap.IDWrap {
	t.Helper()
	userID := idwrap.NewNow()
	us := suser.New(e.queries)
	err := us.CreateUser(e.ctx, &muser.User{
		ID:    userID,
		Email: "user-" + userID.String() + "@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return userID
}

func (e *testEnv) createOrg(t *testing.T, name string) idwrap.IDWrap {
	t.Helper()
	orgID := idwrap.NewNow()
	err := e.queries.AuthCreateOrganization(e.ctx, gen.AuthCreateOrganizationParams{
		ID:        orgID,
		Name:      name,
		Slug:      sql.NullString{String: name + "-slug", Valid: true},
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return orgID
}

func (e *testEnv) createMember(t *testing.T, userID, orgID idwrap.IDWrap, role string) {
	t.Helper()
	err := e.queries.AuthCreateMember(e.ctx, gen.AuthCreateMemberParams{
		ID:             idwrap.NewNow(),
		UserID:         userID,
		OrganizationID: orgID,
		Role:           role,
		CreatedAt:      time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *testEnv) createInvitation(t *testing.T, inviterID, orgID idwrap.IDWrap, email string) {
	t.Helper()
	now := time.Now().Unix()
	err := e.queries.AuthCreateInvitation(e.ctx, gen.AuthCreateInvitationParams{
		ID:             idwrap.NewNow(),
		Email:          email,
		InviterID:      inviterID,
		OrganizationID: orgID,
		Role:           "member",
		Status:         "pending",
		CreatedAt:      now,
		ExpiresAt:      now + 86400,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *testEnv) authedCtx(userID idwrap.IDWrap) context.Context {
	return mwauth.CreateAuthedContext(e.ctx, userID)
}

func TestOrganizationCollection_TwoOrgs(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)

	org1 := e.createOrg(t, "OrgA")
	org2 := e.createOrg(t, "OrgB")
	e.createMember(t, userID, org1, "owner")
	e.createMember(t, userID, org2, "member")

	resp, err := e.handler.OrganizationCollection(e.authedCtx(userID), connect.NewRequest(&emptypb.Empty{}))
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 2, len(resp.Msg.Items))
}

func TestOrganizationCollection_NoOrgs(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)

	resp, err := e.handler.OrganizationCollection(e.authedCtx(userID), connect.NewRequest(&emptypb.Empty{}))
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(resp.Msg.Items))
}

func TestOrganizationMemberCollection(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)
	otherUser := e.createUser(t)

	org1 := e.createOrg(t, "OrgA")
	org2 := e.createOrg(t, "OrgB")

	e.createMember(t, userID, org1, "owner")
	e.createMember(t, otherUser, org1, "member")
	e.createMember(t, userID, org2, "admin")

	resp, err := e.handler.OrganizationMemberCollection(e.authedCtx(userID), connect.NewRequest(&emptypb.Empty{}))
	testutil.AssertFatal(t, nil, err)
	// org1 has 2 members, org2 has 1 member → 3 total
	testutil.Assert(t, 3, len(resp.Msg.Items))
}

func TestOrganizationInvitationCollection_AdminSeesInvitations(t *testing.T) {
	e := setupTest(t)
	adminUser := e.createUser(t)

	orgID := e.createOrg(t, "InvOrg")
	e.createMember(t, adminUser, orgID, "admin")
	e.createInvitation(t, adminUser, orgID, "invite1@example.com")
	e.createInvitation(t, adminUser, orgID, "invite2@example.com")

	resp, err := e.handler.OrganizationInvitationCollection(e.authedCtx(adminUser), connect.NewRequest(&emptypb.Empty{}))
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 2, len(resp.Msg.Items))
}

func TestOrganizationInvitationCollection_MemberSeesNoInvitations(t *testing.T) {
	e := setupTest(t)
	adminUser := e.createUser(t)
	memberUser := e.createUser(t)

	orgID := e.createOrg(t, "InvOrg")
	e.createMember(t, adminUser, orgID, "owner")
	e.createMember(t, memberUser, orgID, "member")
	e.createInvitation(t, adminUser, orgID, "someone@example.com")

	resp, err := e.handler.OrganizationInvitationCollection(e.authedCtx(memberUser), connect.NewRequest(&emptypb.Empty{}))
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(resp.Msg.Items))
}

func TestOrganizationInsert_Unimplemented(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)

	_, err := e.handler.OrganizationInsert(e.authedCtx(userID), connect.NewRequest(&v1.OrganizationInsertRequest{}))
	testutil.AssertNotFatal(t, nil, err)

	var connectErr *connect.Error
	testutil.Assert(t, true, errors.As(err, &connectErr))
	testutil.Assert(t, connect.CodeUnimplemented, connectErr.Code())
}

func TestOrganizationUpdate_Unimplemented(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)

	_, err := e.handler.OrganizationUpdate(e.authedCtx(userID), connect.NewRequest(&v1.OrganizationUpdateRequest{}))
	testutil.AssertNotFatal(t, nil, err)

	var connectErr *connect.Error
	testutil.Assert(t, true, errors.As(err, &connectErr))
	testutil.Assert(t, connect.CodeUnimplemented, connectErr.Code())
}
