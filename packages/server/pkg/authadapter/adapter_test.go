package authadapter_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/authadapter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
)

func jsonStr(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func jsonInt(n int64) json.RawMessage {
	b, _ := json.Marshal(n)
	return b
}

func str(m map[string]any, key string) string {
	return m[key].(string)
}

func newAdapter(t *testing.T) (*authadapter.Adapter, func()) {
	t.Helper()
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	return authadapter.New(base.Queries, base.DB), base.Close
}

func TestAdapter_User(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()
	id := idwrap.NewNow()

	data := map[string]json.RawMessage{
		"id":            jsonStr(id.String()),
		"name":          jsonStr("Alice"),
		"email":         jsonStr("alice@example.com"),
		"emailVerified": jsonInt(0),
		"createdAt":     jsonInt(now),
		"updatedAt":     jsonInt(now),
	}

	// Create
	rec, err := a.Create(ctx, authadapter.ModelUser, data)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, id.String(), str(rec, "id"))
	testutil.Assert(t, "alice@example.com", str(rec, "email"))
	testutil.Assert(t, true, rec["image"] == nil)

	// FindOne by id
	found, err := a.FindOne(ctx, authadapter.ModelUser, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "alice@example.com", str(found, "email"))

	// FindOne by email
	found2, err := a.FindOne(ctx, authadapter.ModelUser, []authadapter.WhereClause{
		{Field: "email", Operator: "eq", Value: jsonStr("alice@example.com"), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, id.String(), str(found2, "id"))

	// FindOne missing → nil
	missing, err := a.FindOne(ctx, authadapter.ModelUser, []authadapter.WhereClause{
		{Field: "email", Operator: "eq", Value: jsonStr("nobody@example.com"), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, true, missing == nil)

	// Update
	updated, err := a.Update(ctx, authadapter.ModelUser,
		[]authadapter.WhereClause{{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"}},
		map[string]json.RawMessage{"name": jsonStr("Alice Updated"), "updatedAt": jsonInt(now + 1)},
	)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "Alice Updated", str(updated, "name"))

	// Count
	count, err := a.Count(ctx, authadapter.ModelUser)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, int64(1), count)

	// Delete
	err = a.Delete(ctx, authadapter.ModelUser, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	// Verify gone
	gone, err := a.FindOne(ctx, authadapter.ModelUser, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, true, gone == nil)
}

func TestAdapter_Session(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()

	// Create user first (FK constraint)
	userID := idwrap.NewNow()
	_, err := a.Create(ctx, authadapter.ModelUser, map[string]json.RawMessage{
		"id":            jsonStr(userID.String()),
		"name":          jsonStr("Bob"),
		"email":         jsonStr("bob@example.com"),
		"emailVerified": jsonInt(0),
		"createdAt":     jsonInt(now),
		"updatedAt":     jsonInt(now),
	})
	testutil.AssertFatal(t, nil, err)

	sessionID := idwrap.NewNow()
	data := map[string]json.RawMessage{
		"id":        jsonStr(sessionID.String()),
		"userId":    jsonStr(userID.String()),
		"token":     jsonStr("tok-abc123"),
		"expiresAt": jsonInt(now + 3600),
		"createdAt": jsonInt(now),
		"updatedAt": jsonInt(now),
	}

	// Create
	rec, err := a.Create(ctx, authadapter.ModelSession, data)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, sessionID.String(), str(rec, "id"))
	testutil.Assert(t, "tok-abc123", str(rec, "token"))

	// FindOne by id
	found, err := a.FindOne(ctx, authadapter.ModelSession, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(sessionID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "tok-abc123", str(found, "token"))

	// FindOne by token
	found2, err := a.FindOne(ctx, authadapter.ModelSession, []authadapter.WhereClause{
		{Field: "token", Operator: "eq", Value: jsonStr("tok-abc123"), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, sessionID.String(), str(found2, "id"))

	// FindMany by userId
	many, err := a.FindMany(ctx, authadapter.ModelSession, []authadapter.WhereClause{
		{Field: "userId", Operator: "eq", Value: jsonStr(userID.String()), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(many))

	// Delete by token
	err = a.Delete(ctx, authadapter.ModelSession, []authadapter.WhereClause{
		{Field: "token", Operator: "eq", Value: jsonStr("tok-abc123"), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	gone, err := a.FindOne(ctx, authadapter.ModelSession, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(sessionID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, true, gone == nil)
}

func TestAdapter_Account(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()

	// Create user first
	userID := idwrap.NewNow()
	_, err := a.Create(ctx, authadapter.ModelUser, map[string]json.RawMessage{
		"id":            jsonStr(userID.String()),
		"name":          jsonStr("Carol"),
		"email":         jsonStr("carol@example.com"),
		"emailVerified": jsonInt(1),
		"createdAt":     jsonInt(now),
		"updatedAt":     jsonInt(now),
	})
	testutil.AssertFatal(t, nil, err)

	accountID := idwrap.NewNow()
	data := map[string]json.RawMessage{
		"id":         jsonStr(accountID.String()),
		"userId":     jsonStr(userID.String()),
		"accountId":  jsonStr("google-sub-123"),
		"providerId": jsonStr("google"),
		"createdAt":  jsonInt(now),
		"updatedAt":  jsonInt(now),
	}

	// Create
	rec, err := a.Create(ctx, authadapter.ModelAccount, data)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, accountID.String(), str(rec, "id"))
	testutil.Assert(t, "google", str(rec, "providerId"))

	// FindOne by id
	found, err := a.FindOne(ctx, authadapter.ModelAccount, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(accountID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "google-sub-123", str(found, "accountId"))

	// FindOne by providerId + accountId
	found2, err := a.FindOne(ctx, authadapter.ModelAccount, []authadapter.WhereClause{
		{Field: "providerId", Operator: "eq", Value: jsonStr("google"), Connector: "AND"},
		{Field: "accountId", Operator: "eq", Value: jsonStr("google-sub-123"), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, accountID.String(), str(found2, "id"))

	// FindMany by userId
	many, err := a.FindMany(ctx, authadapter.ModelAccount, []authadapter.WhereClause{
		{Field: "userId", Operator: "eq", Value: jsonStr(userID.String()), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(many))

	// DeleteMany by userId
	err = a.DeleteMany(ctx, authadapter.ModelAccount, []authadapter.WhereClause{
		{Field: "userId", Operator: "eq", Value: jsonStr(userID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	empty, err := a.FindMany(ctx, authadapter.ModelAccount, []authadapter.WhereClause{
		{Field: "userId", Operator: "eq", Value: jsonStr(userID.String()), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(empty))
}

func TestAdapter_Verification(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()
	id := idwrap.NewNow()

	data := map[string]json.RawMessage{
		"id":         jsonStr(id.String()),
		"identifier": jsonStr("email:dave@example.com"),
		"value":      jsonStr("verify-token-xyz"),
		"expiresAt":  jsonInt(now + 3600),
		"createdAt":  jsonInt(now),
		"updatedAt":  jsonInt(now),
	}

	// Create
	rec, err := a.Create(ctx, authadapter.ModelVerification, data)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, id.String(), str(rec, "id"))

	// FindOne by identifier
	found, err := a.FindOne(ctx, authadapter.ModelVerification, []authadapter.WhereClause{
		{Field: "identifier", Operator: "eq", Value: jsonStr("email:dave@example.com"), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "verify-token-xyz", str(found, "value"))

	// DeleteMany expired (expiresAt lt now — nothing deleted since record expires in future)
	err = a.DeleteMany(ctx, authadapter.ModelVerification, []authadapter.WhereClause{
		{Field: "expiresAt", Operator: "lt", Value: jsonInt(now), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	// Still exists
	still, err := a.FindOne(ctx, authadapter.ModelVerification, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, true, still != nil)

	// Delete by id
	err = a.Delete(ctx, authadapter.ModelVerification, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
}

func TestAdapter_Organization(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()
	id := idwrap.NewNow()

	data := map[string]json.RawMessage{
		"id":        jsonStr(id.String()),
		"name":      jsonStr("Acme Corp"),
		"slug":      jsonStr("acme-corp"),
		"createdAt": jsonInt(now),
	}

	// Create
	rec, err := a.Create(ctx, authadapter.ModelOrganization, data)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, id.String(), str(rec, "id"))
	testutil.Assert(t, "Acme Corp", str(rec, "name"))
	testutil.Assert(t, "acme-corp", str(rec, "slug"))
	// ftDate regression: createdAt must be ISO 8601 string, not raw integer
	testutil.Assert(t, time.Unix(now, 0).UTC().Format(time.RFC3339), rec["createdAt"].(string))

	// FindOne by id
	found, err := a.FindOne(ctx, authadapter.ModelOrganization, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "Acme Corp", str(found, "name"))

	// FindOne by slug
	found2, err := a.FindOne(ctx, authadapter.ModelOrganization, []authadapter.WhereClause{
		{Field: "slug", Operator: "eq", Value: jsonStr("acme-corp"), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, id.String(), str(found2, "id"))

	// FindMany
	many, err := a.FindMany(ctx, authadapter.ModelOrganization, nil, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(many))

	// Update
	updated, err := a.Update(ctx, authadapter.ModelOrganization,
		[]authadapter.WhereClause{{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"}},
		map[string]json.RawMessage{"name": jsonStr("Acme Inc")},
	)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "Acme Inc", str(updated, "name"))

	// Count
	count, err := a.Count(ctx, authadapter.ModelOrganization)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, int64(1), count)

	// Delete
	err = a.Delete(ctx, authadapter.ModelOrganization, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	// Verify gone
	gone, err := a.FindOne(ctx, authadapter.ModelOrganization, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, true, gone == nil)
}

func TestAdapter_Member(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()

	// Create user first (FK)
	userID := idwrap.NewNow()
	_, err := a.Create(ctx, authadapter.ModelUser, map[string]json.RawMessage{
		"id":            jsonStr(userID.String()),
		"name":          jsonStr("Alice"),
		"email":         jsonStr("alice@example.com"),
		"emailVerified": jsonInt(0),
		"createdAt":     jsonInt(now),
		"updatedAt":     jsonInt(now),
	})
	testutil.AssertFatal(t, nil, err)

	// Create org (FK)
	orgID := idwrap.NewNow()
	_, err = a.Create(ctx, authadapter.ModelOrganization, map[string]json.RawMessage{
		"id":        jsonStr(orgID.String()),
		"name":      jsonStr("Test Org"),
		"slug":      jsonStr("test-org"),
		"createdAt": jsonInt(now),
	})
	testutil.AssertFatal(t, nil, err)

	memberID := idwrap.NewNow()
	data := map[string]json.RawMessage{
		"id":             jsonStr(memberID.String()),
		"userId":         jsonStr(userID.String()),
		"organizationId": jsonStr(orgID.String()),
		"role":           jsonStr("owner"),
		"createdAt":      jsonInt(now),
	}

	// Create
	rec, err := a.Create(ctx, authadapter.ModelMember, data)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, memberID.String(), str(rec, "id"))
	testutil.Assert(t, "owner", str(rec, "role"))
	// ftDate regression: createdAt must be ISO 8601 string, not raw integer
	testutil.Assert(t, time.Unix(now, 0).UTC().Format(time.RFC3339), rec["createdAt"].(string))

	// FindOne by id
	found, err := a.FindOne(ctx, authadapter.ModelMember, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(memberID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "owner", str(found, "role"))

	// FindOne by userId + organizationId
	found2, err := a.FindOne(ctx, authadapter.ModelMember, []authadapter.WhereClause{
		{Field: "userId", Operator: "eq", Value: jsonStr(userID.String()), Connector: "AND"},
		{Field: "organizationId", Operator: "eq", Value: jsonStr(orgID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, memberID.String(), str(found2, "id"))

	// FindMany by organizationId
	many, err := a.FindMany(ctx, authadapter.ModelMember, []authadapter.WhereClause{
		{Field: "organizationId", Operator: "eq", Value: jsonStr(orgID.String()), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(many))

	// FindMany by userId
	manyByUser, err := a.FindMany(ctx, authadapter.ModelMember, []authadapter.WhereClause{
		{Field: "userId", Operator: "eq", Value: jsonStr(userID.String()), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(manyByUser))

	// Update role
	updated, err := a.Update(ctx, authadapter.ModelMember,
		[]authadapter.WhereClause{{Field: "id", Operator: "eq", Value: jsonStr(memberID.String()), Connector: "AND"}},
		map[string]json.RawMessage{"role": jsonStr("admin")},
	)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "admin", str(updated, "role"))

	// DeleteMany by organizationId
	err = a.DeleteMany(ctx, authadapter.ModelMember, []authadapter.WhereClause{
		{Field: "organizationId", Operator: "eq", Value: jsonStr(orgID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	empty, err := a.FindMany(ctx, authadapter.ModelMember, []authadapter.WhereClause{
		{Field: "organizationId", Operator: "eq", Value: jsonStr(orgID.String()), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 0, len(empty))
}

func TestAdapter_Invitation(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()

	// Create user (FK for inviter)
	userID := idwrap.NewNow()
	_, err := a.Create(ctx, authadapter.ModelUser, map[string]json.RawMessage{
		"id":            jsonStr(userID.String()),
		"name":          jsonStr("Bob"),
		"email":         jsonStr("bob@example.com"),
		"emailVerified": jsonInt(1),
		"createdAt":     jsonInt(now),
		"updatedAt":     jsonInt(now),
	})
	testutil.AssertFatal(t, nil, err)

	// Create org (FK)
	orgID := idwrap.NewNow()
	_, err = a.Create(ctx, authadapter.ModelOrganization, map[string]json.RawMessage{
		"id":        jsonStr(orgID.String()),
		"name":      jsonStr("Invite Org"),
		"slug":      jsonStr("invite-org"),
		"createdAt": jsonInt(now),
	})
	testutil.AssertFatal(t, nil, err)

	invID := idwrap.NewNow()
	data := map[string]json.RawMessage{
		"id":             jsonStr(invID.String()),
		"email":          jsonStr("carol@example.com"),
		"inviterId":      jsonStr(userID.String()),
		"organizationId": jsonStr(orgID.String()),
		"role":           jsonStr("member"),
		"status":         jsonStr("pending"),
		"createdAt":      jsonInt(now),
		"expiresAt":      jsonInt(now + 86400),
	}

	// Create
	rec, err := a.Create(ctx, authadapter.ModelInvitation, data)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, invID.String(), str(rec, "id"))
	testutil.Assert(t, "pending", str(rec, "status"))
	testutil.Assert(t, "carol@example.com", str(rec, "email"))
	// ftDate regression: createdAt and expiresAt must be ISO 8601 strings, not raw integers
	testutil.Assert(t, time.Unix(now, 0).UTC().Format(time.RFC3339), rec["createdAt"].(string))
	testutil.Assert(t, time.Unix(now+86400, 0).UTC().Format(time.RFC3339), rec["expiresAt"].(string))

	// FindOne by id
	found, err := a.FindOne(ctx, authadapter.ModelInvitation, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(invID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "pending", str(found, "status"))

	// FindMany by organizationId
	many, err := a.FindMany(ctx, authadapter.ModelInvitation, []authadapter.WhereClause{
		{Field: "organizationId", Operator: "eq", Value: jsonStr(orgID.String()), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(many))

	// FindMany by email
	manyByEmail, err := a.FindMany(ctx, authadapter.ModelInvitation, []authadapter.WhereClause{
		{Field: "email", Operator: "eq", Value: jsonStr("carol@example.com"), Connector: "AND"},
	}, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(manyByEmail))

	// Update status
	updated, err := a.Update(ctx, authadapter.ModelInvitation,
		[]authadapter.WhereClause{{Field: "id", Operator: "eq", Value: jsonStr(invID.String()), Connector: "AND"}},
		map[string]json.RawMessage{"status": jsonStr("accepted")},
	)
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, "accepted", str(updated, "status"))

	// Delete
	err = a.Delete(ctx, authadapter.ModelInvitation, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(invID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	gone, err := a.FindOne(ctx, authadapter.ModelInvitation, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(invID.String()), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, true, gone == nil)
}

func TestAdapter_Jwks(t *testing.T) {
	a, cleanup := newAdapter(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Unix()

	data := map[string]json.RawMessage{
		"publicKey":  jsonStr(`{"kty":"RSA","n":"abc","e":"AQAB"}`),
		"privateKey": jsonStr(`{"kty":"RSA","n":"abc","d":"xyz"}`),
		"createdAt":  jsonInt(now),
	}

	// Create (auto-generated id)
	rec, err := a.Create(ctx, authadapter.ModelJwks, data)
	testutil.AssertFatal(t, nil, err)
	id := str(rec, "id")
	testutil.Assert(t, true, id != "")
	testutil.Assert(t, `{"kty":"RSA","n":"abc","e":"AQAB"}`, str(rec, "publicKey"))
	testutil.Assert(t, `{"kty":"RSA","n":"abc","d":"xyz"}`, str(rec, "privateKey"))
	testutil.Assert(t, now, rec["createdAt"].(int64))
	testutil.Assert(t, true, rec["expiresAt"] == nil)

	// Create second key with expiresAt
	data2 := map[string]json.RawMessage{
		"publicKey":  jsonStr(`{"kty":"RSA","n":"def","e":"AQAB"}`),
		"privateKey": jsonStr(`{"kty":"RSA","n":"def","d":"uvw"}`),
		"createdAt":  jsonInt(now + 1),
		"expiresAt":  jsonInt(now + 86400),
	}
	rec2, err := a.Create(ctx, authadapter.ModelJwks, data2)
	testutil.AssertFatal(t, nil, err)
	id2 := str(rec2, "id")
	testutil.Assert(t, now+86400, rec2["expiresAt"].(int64))

	// FindMany returns all keys (newest first)
	many, err := a.FindMany(ctx, authadapter.ModelJwks, nil, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 2, len(many))
	testutil.Assert(t, id2, str(many[0], "id")) // newest first

	// Delete first key
	err = a.Delete(ctx, authadapter.ModelJwks, []authadapter.WhereClause{
		{Field: "id", Operator: "eq", Value: jsonStr(id), Connector: "AND"},
	})
	testutil.AssertFatal(t, nil, err)

	// FindMany returns only second key
	remaining, err := a.FindMany(ctx, authadapter.ModelJwks, nil, authadapter.FindManyOpts{})
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, 1, len(remaining))
	testutil.Assert(t, id2, str(remaining[0], "id"))
}
