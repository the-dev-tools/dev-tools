package sorg_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/morg"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sorg"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
)

type testEnv struct {
	ctx     context.Context
	db      *sql.DB
	queries *gen.Queries
	reader  *sorg.Reader
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

	return &testEnv{
		ctx:     ctx,
		db:      db,
		queries: queries,
		reader:  sorg.NewReader(db),
	}
}

func (e *testEnv) createUser(t *testing.T) idwrap.IDWrap {
	t.Helper()
	userID := idwrap.NewNow()
	us := suser.New(e.queries)
	err := us.CreateUser(e.ctx, &muser.User{
		ID:    userID,
		Email: "test-" + userID.String() + "@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return userID
}

func (e *testEnv) createOrg(t *testing.T, name string) idwrap.IDWrap {
	t.Helper()
	orgID := idwrap.NewNow()
	slug := sql.NullString{String: name + "-slug", Valid: true}
	err := e.queries.AuthCreateOrganization(e.ctx, gen.AuthCreateOrganizationParams{
		ID:        orgID,
		Name:      name,
		Slug:      slug,
		CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return orgID
}

func (e *testEnv) createMember(t *testing.T, userID, orgID idwrap.IDWrap, role string) idwrap.IDWrap {
	t.Helper()
	memberID := idwrap.NewNow()
	err := e.queries.AuthCreateMember(e.ctx, gen.AuthCreateMemberParams{
		ID:             memberID,
		UserID:         userID,
		OrganizationID: orgID,
		Role:           role,
		CreatedAt:      time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return memberID
}

func (e *testEnv) createInvitation(t *testing.T, inviterID, orgID idwrap.IDWrap, email string) idwrap.IDWrap {
	t.Helper()
	invID := idwrap.NewNow()
	now := time.Now().Unix()
	err := e.queries.AuthCreateInvitation(e.ctx, gen.AuthCreateInvitationParams{
		ID:             invID,
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
	return invID
}

func TestGetOrganization_Found(t *testing.T) {
	e := setupTest(t)
	orgID := e.createOrg(t, "TestOrg")

	org, err := e.reader.GetOrganization(e.ctx, orgID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "TestOrg", org.Name)
	testutil.Assert(t, orgID, org.ID)
}

func TestGetOrganization_NotFound(t *testing.T) {
	e := setupTest(t)

	_, err := e.reader.GetOrganization(e.ctx, idwrap.NewNow())
	testutil.AssertNotFatal(t, nil, err)
}

func TestListOrganizationsForUser_NoOrgs(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)

	orgs, err := e.reader.ListOrganizationsForUser(e.ctx, userID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(orgs))
}

func TestListOrganizationsForUser_TwoOrgs(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)

	org1 := e.createOrg(t, "Org1")
	org2 := e.createOrg(t, "Org2")
	e.createOrg(t, "Org3") // user is NOT a member of this one

	e.createMember(t, userID, org1, "owner")
	e.createMember(t, userID, org2, "member")

	orgs, err := e.reader.ListOrganizationsForUser(e.ctx, userID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 2, len(orgs))

	// Check that we got the right orgs (order may vary)
	names := map[string]bool{}
	for _, o := range orgs {
		names[o.Name] = true
	}
	testutil.Assert(t, true, names["Org1"])
	testutil.Assert(t, true, names["Org2"])
	testutil.Assert(t, false, names["Org3"])
}

func TestListOrganizationsForUser_NonMember(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)
	otherUser := e.createUser(t)

	orgID := e.createOrg(t, "OtherOrg")
	e.createMember(t, otherUser, orgID, "owner")

	orgs, err := e.reader.ListOrganizationsForUser(e.ctx, userID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(orgs))
}

func TestListMembers_WithMembers(t *testing.T) {
	e := setupTest(t)
	user1 := e.createUser(t)
	user2 := e.createUser(t)
	orgID := e.createOrg(t, "MemberOrg")

	e.createMember(t, user1, orgID, "owner")
	e.createMember(t, user2, orgID, "member")

	members, err := e.reader.ListMembers(e.ctx, orgID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 2, len(members))
}

func TestListMembers_EmptyOrg(t *testing.T) {
	e := setupTest(t)
	orgID := e.createOrg(t, "EmptyOrg")

	members, err := e.reader.ListMembers(e.ctx, orgID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(members))
}

func TestGetMemberByUserAndOrg_Found(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")
	e.createMember(t, userID, orgID, "admin")

	member, err := e.reader.GetMemberByUserAndOrg(e.ctx, userID, orgID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, morg.RoleAdmin, member.Role)
	testutil.Assert(t, userID, member.UserID)
	testutil.Assert(t, orgID, member.OrganizationID)
}

func TestGetMemberByUserAndOrg_NotFound(t *testing.T) {
	e := setupTest(t)
	userID := e.createUser(t)
	orgID := e.createOrg(t, "TestOrg")

	_, err := e.reader.GetMemberByUserAndOrg(e.ctx, userID, orgID)
	testutil.AssertNotFatal(t, nil, err)
}

func TestListInvitations_WithInvitations(t *testing.T) {
	e := setupTest(t)
	inviterID := e.createUser(t)
	orgID := e.createOrg(t, "InvOrg")
	e.createMember(t, inviterID, orgID, "owner")

	e.createInvitation(t, inviterID, orgID, "a@example.com")
	e.createInvitation(t, inviterID, orgID, "b@example.com")

	invitations, err := e.reader.ListInvitations(e.ctx, orgID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 2, len(invitations))
}

func TestListInvitations_Empty(t *testing.T) {
	e := setupTest(t)
	orgID := e.createOrg(t, "NoInvOrg")

	invitations, err := e.reader.ListInvitations(e.ctx, orgID)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(invitations))
}
