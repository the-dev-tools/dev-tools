package rauthadapter

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/authadapter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	auth_adapterv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/private/auth_adapter/v1"
)

// newHandler returns a ready-to-use AuthAdapterRPC backed by an in-memory SQLite DB.
func newHandler(t *testing.T) (AuthAdapterRPC, func()) {
	t.Helper()
	base := testutil.CreateBaseDB(context.Background(), t)
	adapter := authadapter.New(base.Queries, base.DB)
	h := New(AuthAdapterRPCDeps{Adapter: adapter})
	return h, base.Close
}

// newHandlerWithFK is like newHandler but enables SQLite FK enforcement so that
// ON DELETE CASCADE constraints fire. Tests that rely on DB-level cascade must
// use this variant. PRAGMA foreign_keys is per-connection; SetMaxOpenConns(1)
// ensures it applies to every query on the returned DB.
func newHandlerWithFK(t *testing.T) (AuthAdapterRPC, func()) {
	t.Helper()
	base := testutil.CreateBaseDB(context.Background(), t)
	base.DB.SetMaxOpenConns(1)
	_, err := base.DB.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatal(err)
	}
	adapter := authadapter.New(base.Queries, base.DB)
	h := New(AuthAdapterRPCDeps{Adapter: adapter})
	return h, base.Close
}

// jsonValue builds a *structpb.Value from a plain Go map (panics on error — test helper only).
func jsonValue(m map[string]any) *structpb.Value {
	v, err := structpb.NewValue(m)
	if err != nil {
		panic(err)
	}
	return v
}

// jsonMap converts a map[string]any to map[string]*structpb.Value for use in
// CreateRequest.Data (which expects a proto map, not a single Value).
func jsonMap(m map[string]any) map[string]*structpb.Value {
	out := make(map[string]*structpb.Value, len(m))
	for k, v := range m {
		pv, err := structpb.NewValue(v)
		if err != nil {
			panic(err)
		}
		out[k] = pv
	}
	return out
}

// protoMapToAny converts map[string]*structpb.Value to map[string]any for
// test assertions on CreateResponse.Data.
func protoMapToAny(m map[string]*structpb.Value) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v.AsInterface()
	}
	return out
}

// eqWhere builds a single OPERATOR_EQUAL Where clause.
func eqWhere(field string, val *structpb.Value) *auth_adapterv1.Where {
	return &auth_adapterv1.Where{
		Field:     field,
		Operator:  auth_adapterv1.Operator_OPERATOR_EQUAL,
		Value:     val,
		Connector: auth_adapterv1.Connector_CONNECTOR_AND,
	}
}

// --- Create ---

func TestCreate_user(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name":          "Alice",
			"email":         "alice@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)

	fields := resp.Msg.Data
	require.Equal(t, "Alice", fields["name"].GetStringValue())
	require.Equal(t, "alice@example.com", fields["email"].GetStringValue())
	require.NotEmpty(t, fields["id"].GetStringValue())
}

func TestCreate_session(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	// Create a user first (session FK)
	userResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name":          "Bob",
			"email":         "bob@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	userID := userResp.Msg.Data["id"].GetStringValue()

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "tok-abc",
			"expiresAt": now + 3600,
			"createdAt": now,
			"updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data
	require.Equal(t, "tok-abc", fields["token"].GetStringValue())
}

func TestCreate_unsupportedModel(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "unknown",
		Data:  jsonMap(map[string]any{}),
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// --- Find ---

func TestFind_byID(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name":          "Carol",
			"email":         "carol@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{
			eqWhere("id", structpb.NewStringValue(id)),
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "Carol", resp.Msg.Data.GetStructValue().GetFields()["name"].GetStringValue())
}

func TestFind_byEmail(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name":          "Dave",
			"email":         "dave@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{
			eqWhere("email", structpb.NewStringValue("dave@example.com")),
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "Dave", resp.Msg.Data.GetStructValue().GetFields()["name"].GetStringValue())
}

func TestFind_notFound(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{
			eqWhere("email", structpb.NewStringValue("nobody@example.com")),
		},
	}))
	require.NoError(t, err)
	require.Nil(t, resp.Msg.Data)
}

// --- FindMany ---

func TestFindMany_sessions(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name": "Eve", "email": "eve@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	userID := userResp.Msg.Data["id"].GetStringValue()

	for _, tok := range []string{"tok-1", "tok-2"} {
		_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
			Model: "session",
			Data: jsonMap(map[string]any{
				"userId":    userID,
				"token":     tok,
				"expiresAt": now + 3600,
				"createdAt": now,
				"updatedAt": now,
			}),
		}))
		require.NoError(t, err)
	}

	resp, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{
			eqWhere("userId", structpb.NewStringValue(userID)),
		},
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 2)
}

// --- Update ---

func TestUpdate_user(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name": "Frank", "email": "frank@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	resp, err := h.Update(context.Background(), connect.NewRequest(&auth_adapterv1.UpdateRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{
			eqWhere("id", structpb.NewStringValue(id)),
		},
		Update: jsonValue(map[string]any{"name": "Frank Updated"}),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "Frank Updated", resp.Msg.Data.GetStructValue().GetFields()["name"].GetStringValue())
}

// --- Delete ---

func TestDelete_user(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name": "Grace", "email": "grace@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	_, err = h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{
			eqWhere("id", structpb.NewStringValue(id)),
		},
	}))
	require.NoError(t, err)

	// Verify deleted
	findResp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{
			eqWhere("id", structpb.NewStringValue(id)),
		},
	}))
	require.NoError(t, err)
	require.Nil(t, findResp.Msg.Data)
}

// --- DeleteMany ---

func TestDeleteMany_expiredSessions(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name": "Hank", "email": "hank@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	userID := userResp.Msg.Data["id"].GetStringValue()

	// Create 2 sessions: one expired, one not
	_, err = h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "expired-tok",
			"expiresAt": now - 1000, // expired
			"createdAt": now,
			"updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	_, err = h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "valid-tok",
			"expiresAt": now + 3600,
			"createdAt": now,
			"updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	// DeleteMany by userId
	_, err = h.DeleteMany(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteManyRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{
			eqWhere("userId", structpb.NewStringValue(userID)),
		},
	}))
	require.NoError(t, err)

	// Both sessions gone
	manyResp, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{
			eqWhere("userId", structpb.NewStringValue(userID)),
		},
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Empty(t, manyResp.Msg.Items)
}

// --- Count ---

func TestCount_users(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	for _, email := range []string{"u1@x.com", "u2@x.com", "u3@x.com"} {
		_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
			Model: "user",
			Data: jsonMap(map[string]any{
				"name": email, "email": email,
				"emailVerified": false, "createdAt": now, "updatedAt": now,
			}),
		}))
		require.NoError(t, err)
	}

	resp, err := h.Count(context.Background(), connect.NewRequest(&auth_adapterv1.CountRequest{
		Model: "user",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(3), resp.Msg.Count)
}

// --- UpdateMany ---

func TestUpdateMany_user(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	// Create two users
	for _, email := range []string{"um1@x.com", "um2@x.com"} {
		_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
			Model: "user",
			Data: jsonMap(map[string]any{
				"name": email, "email": email,
				"emailVerified": false, "createdAt": now, "updatedAt": now,
			}),
		}))
		require.NoError(t, err)
	}

	// UpdateMany: set name to "Updated" for all users with email containing "um"
	resp, err := h.UpdateMany(context.Background(), connect.NewRequest(&auth_adapterv1.UpdateManyRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{{
			Field:     "email",
			Operator:  auth_adapterv1.Operator_OPERATOR_CONTAINS,
			Value:     structpb.NewStringValue("um"),
			Connector: auth_adapterv1.Connector_CONNECTOR_AND,
		}},
		Update: jsonMap(map[string]any{"name": "Updated"}),
	}))
	require.NoError(t, err)
	require.Equal(t, int32(2), resp.Msg.Count)
}

func TestUpdateMany_unsupportedModel(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.UpdateMany(context.Background(), connect.NewRequest(&auth_adapterv1.UpdateManyRequest{
		Model: "unknown",
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// --- account ---

// createUserFixture is a test helper that creates a user and returns its ID.
func createUserFixture(t *testing.T, h AuthAdapterRPC, name, email string) string {
	t.Helper()
	now := float64(time.Now().Unix())
	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name":          name,
			"email":         email,
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	return resp.Msg.Data["id"].GetStringValue()
}

func TestCreate_account(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Ivan", "ivan@example.com")

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId":     userID,
			"accountId":  "gh-123",
			"providerId": "github",
			"createdAt":  now,
			"updatedAt":  now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data
	require.Equal(t, "gh-123", fields["accountId"].GetStringValue())
	require.Equal(t, "github", fields["providerId"].GetStringValue())
	require.NotEmpty(t, fields["id"].GetStringValue())
}

func TestCreate_accountWithTokens(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Julia", "julia@example.com")

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId":               userID,
			"accountId":            "google-456",
			"providerId":           "google",
			"accessToken":          "at-xyz",
			"refreshToken":         "rt-xyz",
			"accessTokenExpiresAt": now + 3600,
			"scope":                "openid email",
			"createdAt":            now,
			"updatedAt":            now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data
	require.Equal(t, "at-xyz", fields["accessToken"].GetStringValue())
	require.Equal(t, "rt-xyz", fields["refreshToken"].GetStringValue())
	require.Equal(t, "openid email", fields["scope"].GetStringValue())
}

func TestFind_accountByID(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Karl", "karl@example.com")

	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId": userID, "accountId": "gh-k", "providerId": "github",
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "gh-k", resp.Msg.Data.GetStructValue().GetFields()["accountId"].GetStringValue())
}

// TestFind_accountByProviderAndAccountId covers the two-field eq where path —
// BetterAuth's primary lookup during OAuth sign-in.
func TestFind_accountByProviderAndAccountId(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Lena", "lena@example.com")

	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId": userID, "accountId": "lena-gh", "providerId": "github",
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{
			eqWhere("providerId", structpb.NewStringValue("github")),
			eqWhere("accountId", structpb.NewStringValue("lena-gh")),
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, userID, resp.Msg.Data.GetStructValue().GetFields()["userId"].GetStringValue())
}

func TestFind_accountNotFound(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{
			eqWhere("providerId", structpb.NewStringValue("github")),
			eqWhere("accountId", structpb.NewStringValue("nonexistent")),
		},
	}))
	require.NoError(t, err)
	require.Nil(t, resp.Msg.Data)
}

func TestFindMany_accountsByUserId(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Mia", "mia@example.com")

	for _, p := range []string{"github", "google"} {
		_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
			Model: "account",
			Data: jsonMap(map[string]any{
				"userId": userID, "accountId": "mia-" + p, "providerId": p,
				"createdAt": now, "updatedAt": now,
			}),
		}))
		require.NoError(t, err)
	}

	resp, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{eqWhere("userId", structpb.NewStringValue(userID))},
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 2)
}

func TestUpdate_account(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Ned", "ned@example.com")

	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId": userID, "accountId": "ned-gh", "providerId": "github",
			"accessToken": "old-token",
			"createdAt":   now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	resp, err := h.Update(context.Background(), connect.NewRequest(&auth_adapterv1.UpdateRequest{
		Model:  "account",
		Where:  []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
		Update: jsonValue(map[string]any{"accessToken": "new-token", "updatedAt": now + 1}),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "new-token", resp.Msg.Data.GetStructValue().GetFields()["accessToken"].GetStringValue())
}

func TestDelete_account(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Olivia", "olivia@example.com")

	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId": userID, "accountId": "ol-gh", "providerId": "github",
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	_, err = h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
	}))
	require.NoError(t, err)

	findResp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
	}))
	require.NoError(t, err)
	require.Nil(t, findResp.Msg.Data)
}

func TestDeleteMany_accountsByUserId(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Pete", "pete@example.com")

	for _, p := range []string{"github", "google"} {
		_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
			Model: "account",
			Data: jsonMap(map[string]any{
				"userId": userID, "accountId": "pete-" + p, "providerId": p,
				"createdAt": now, "updatedAt": now,
			}),
		}))
		require.NoError(t, err)
	}

	_, err := h.DeleteMany(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteManyRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{eqWhere("userId", structpb.NewStringValue(userID))},
	}))
	require.NoError(t, err)

	listResp, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{eqWhere("userId", structpb.NewStringValue(userID))},
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Empty(t, listResp.Msg.Items)
}

// --- verification ---

func TestCreate_verification(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "verification",
		Data: jsonMap(map[string]any{
			"identifier": "quinn@example.com",
			"value":      "token-abc123",
			"expiresAt":  now + 86400,
			"createdAt":  now,
			"updatedAt":  now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data
	require.Equal(t, "quinn@example.com", fields["identifier"].GetStringValue())
	require.Equal(t, "token-abc123", fields["value"].GetStringValue())
	require.NotEmpty(t, fields["id"].GetStringValue())
}

func TestFind_verificationByID(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "verification",
		Data: jsonMap(map[string]any{
			"identifier": "rosa@example.com",
			"value":      "vtoken-rosa",
			"expiresAt":  now + 3600,
			"createdAt":  now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "verification",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "vtoken-rosa", resp.Msg.Data.GetStructValue().GetFields()["value"].GetStringValue())
}

// TestFind_verificationByIdentifier covers BetterAuth's primary verification lookup.
func TestFind_verificationByIdentifier(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "verification",
		Data: jsonMap(map[string]any{
			"identifier": "sam@example.com",
			"value":      "vtoken-sam",
			"expiresAt":  now + 3600,
			"createdAt":  now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "verification",
		Where: []*auth_adapterv1.Where{eqWhere("identifier", structpb.NewStringValue("sam@example.com"))},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "vtoken-sam", resp.Msg.Data.GetStructValue().GetFields()["value"].GetStringValue())
}

func TestDelete_verification(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "verification",
		Data: jsonMap(map[string]any{
			"identifier": "tara@example.com",
			"value":      "vtoken-tara",
			"expiresAt":  now + 3600,
			"createdAt":  now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	_, err = h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "verification",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
	}))
	require.NoError(t, err)

	findResp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "verification",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
	}))
	require.NoError(t, err)
	require.Nil(t, findResp.Msg.Data)
}

// TestDeleteMany_expiredVerifications uses the OPERATOR_LESS_THAN where path —
// BetterAuth's background cleanup of stale verification tokens.
func TestDeleteMany_expiredVerifications(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())

	// expired
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "verification",
		Data: jsonMap(map[string]any{
			"identifier": "expired@example.com",
			"value":      "old-token",
			"expiresAt":  now - 1000,
			"createdAt":  now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	// still valid
	_, err = h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "verification",
		Data: jsonMap(map[string]any{
			"identifier": "valid@example.com",
			"value":      "live-token",
			"expiresAt":  now + 3600,
			"createdAt":  now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	_, err = h.DeleteMany(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteManyRequest{
		Model: "verification",
		Where: []*auth_adapterv1.Where{{
			Field:     "expiresAt",
			Operator:  auth_adapterv1.Operator_OPERATOR_LESS_THAN,
			Value:     structpb.NewNumberValue(now),
			Connector: auth_adapterv1.Connector_CONNECTOR_AND,
		}},
	}))
	require.NoError(t, err)

	// expired one gone, live one intact
	findExpired, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "verification",
		Where: []*auth_adapterv1.Where{eqWhere("identifier", structpb.NewStringValue("expired@example.com"))},
	}))
	require.NoError(t, err)
	require.Nil(t, findExpired.Msg.Data)

	findValid, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "verification",
		Where: []*auth_adapterv1.Where{eqWhere("identifier", structpb.NewStringValue("valid@example.com"))},
	}))
	require.NoError(t, err)
	require.NotNil(t, findValid.Msg.Data)
}

// --- session additional paths ---

// TestFind_sessionByToken covers BetterAuth's primary session validation path.
func TestFind_sessionByToken(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Uma", "uma@example.com")

	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "session-tok-uma",
			"expiresAt": now + 3600,
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{eqWhere("token", structpb.NewStringValue("session-tok-uma"))},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, userID, resp.Msg.Data.GetStructValue().GetFields()["userId"].GetStringValue())
}

func TestCreate_sessionWithOptionalFields(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Victor", "victor@example.com")

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "tok-victor",
			"expiresAt": now + 3600,
			"ipAddress": "192.168.1.1",
			"userAgent": "Mozilla/5.0",
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data
	require.Equal(t, "192.168.1.1", fields["ipAddress"].GetStringValue())
	require.Equal(t, "Mozilla/5.0", fields["userAgent"].GetStringValue())
}

func TestUpdate_session(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Wendy", "wendy@example.com")

	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "tok-wendy",
			"expiresAt": now + 3600,
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()
	newExpiry := now + 7200

	resp, err := h.Update(context.Background(), connect.NewRequest(&auth_adapterv1.UpdateRequest{
		Model:  "session",
		Where:  []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
		Update: jsonValue(map[string]any{"expiresAt": newExpiry, "updatedAt": now + 1}),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, newExpiry, resp.Msg.Data.GetStructValue().GetFields()["expiresAt"].GetNumberValue())
}

// TestDelete_sessionByToken covers the logout path — BetterAuth deletes by token.
func TestDelete_sessionByToken(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Xena", "xena@example.com")

	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "tok-xena",
			"expiresAt": now + 3600,
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	_, err = h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{eqWhere("token", structpb.NewStringValue("tok-xena"))},
	}))
	require.NoError(t, err)

	findResp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{eqWhere("token", structpb.NewStringValue("tok-xena"))},
	}))
	require.NoError(t, err)
	require.Nil(t, findResp.Msg.Data)
}

// TestDeleteMany_sessionsByExpiresAtLt uses OPERATOR_LESS_THAN — BetterAuth's
// expired session GC path.
func TestDeleteMany_sessionsByExpiresAtLt(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Yara", "yara@example.com")

	for _, tok := range []string{"expired-1", "expired-2"} {
		_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
			Model: "session",
			Data: jsonMap(map[string]any{
				"userId":    userID,
				"token":     tok,
				"expiresAt": now - 1000,
				"createdAt": now, "updatedAt": now,
			}),
		}))
		require.NoError(t, err)
	}
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId":    userID,
			"token":     "live-tok",
			"expiresAt": now + 3600,
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	_, err = h.DeleteMany(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteManyRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{{
			Field:     "expiresAt",
			Operator:  auth_adapterv1.Operator_OPERATOR_LESS_THAN,
			Value:     structpb.NewNumberValue(now),
			Connector: auth_adapterv1.Connector_CONNECTOR_AND,
		}},
	}))
	require.NoError(t, err)

	listResp, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{eqWhere("userId", structpb.NewStringValue(userID))},
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Items, 1)
	require.Equal(t, "live-tok", listResp.Msg.Items[0].GetStructValue().GetFields()["token"].GetStringValue())
}

// --- user additional paths ---

func TestCreate_userWithImage(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name":          "Zara",
			"email":         "zara@example.com",
			"emailVerified": false,
			"image":         "https://example.com/avatar.png",
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	require.Equal(t, "https://example.com/avatar.png", resp.Msg.Data["image"].GetStringValue())
}

func TestUpdate_userEmailVerified(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	createResp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name":          "Alex",
			"email":         "alex@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data["id"].GetStringValue()

	resp, err := h.Update(context.Background(), connect.NewRequest(&auth_adapterv1.UpdateRequest{
		Model:  "user",
		Where:  []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(id))},
		Update: jsonValue(map[string]any{"emailVerified": true, "updatedAt": now + 1}),
	}))
	require.NoError(t, err)
	// emailVerified=true is stored as int64(1)
	require.Equal(t, float64(1), resp.Msg.Data.GetStructValue().GetFields()["emailVerified"].GetNumberValue())
}

// --- error paths ---

func TestFind_unsupportedModel(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "unknown",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue("x"))},
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestFindMany_unsupportedModel(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "unknown",
		Limit: 10,
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestCount_unsupportedModel(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.Count(context.Background(), connect.NewRequest(&auth_adapterv1.CountRequest{
		Model: "session",
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestDelete_unsupportedModel(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "unknown",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue("x"))},
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestFind_unsupportedWhereField(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	// Truly unknown field name that does not match any auth_user column
	_, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{eqWhere("nonExistentField", structpb.NewStringValue("Alice"))},
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestFind_userByName(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name": "NameLookup", "email": "namelookup@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	// Find by name — now supported via dynamic SQL fallback
	resp, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{eqWhere("name", structpb.NewStringValue("NameLookup"))},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.Equal(t, "namelookup@example.com", resp.Msg.Data.GetStructValue().GetFields()["email"].GetStringValue())
}

// --- connector conversion ---

func TestConnectorToString(t *testing.T) {
	require.Equal(t, "OR", connectorToString(auth_adapterv1.Connector_CONNECTOR_OR))
	require.Equal(t, "AND", connectorToString(auth_adapterv1.Connector_CONNECTOR_AND))
	require.Equal(t, "AND", connectorToString(auth_adapterv1.Connector_CONNECTOR_UNSPECIFIED))
}

// --- operator conversion ---

func TestOperatorToString(t *testing.T) {
	cases := []struct {
		op   auth_adapterv1.Operator
		want string
	}{
		{auth_adapterv1.Operator_OPERATOR_EQUAL, "eq"},
		{auth_adapterv1.Operator_OPERATOR_NOT_EQUAL, "ne"},
		{auth_adapterv1.Operator_OPERATOR_LESS_THAN, "lt"},
		{auth_adapterv1.Operator_OPERATOR_LESS_OR_EQUAL, "lte"},
		{auth_adapterv1.Operator_OPERATOR_GREATER_THAN, "gt"},
		{auth_adapterv1.Operator_OPERATOR_GREATER_OR_EQUAL, "gte"},
		{auth_adapterv1.Operator_OPERATOR_IN, "in"},
		{auth_adapterv1.Operator_OPERATOR_NOT_IN, "not_in"},
		{auth_adapterv1.Operator_OPERATOR_CONTAINS, "contains"},
		{auth_adapterv1.Operator_OPERATOR_STARTS_WITH, "starts_with"},
		{auth_adapterv1.Operator_OPERATOR_ENDS_WITH, "ends_with"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, operatorToString(c.op))
	}
}

// --- authadapter sentinel errors map to CodeInvalidArgument ---

func TestAdapterErr_unsupportedModel(t *testing.T) {
	err := adapterErr(authadapter.ErrUnsupportedModel)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestAdapterErr_unsupportedWhere(t *testing.T) {
	err := adapterErr(authadapter.ErrUnsupportedWhere)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// =============================================================================
// Schema-driven tests
//
// The tests below are derived from the BetterAuth schema (see package doc) and
// verify that uniqueness constraints, FK cascade behaviour, optional/required
// fields, and the unimplemented jwks model all behave as specified.
// =============================================================================

// --- user: schema constraints ---

// TestCreate_userDuplicateEmail verifies the email UNIQUE constraint.
func TestCreate_userDuplicateEmail(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	data := map[string]any{
		"name": "Dup", "email": "dup@example.com",
		"emailVerified": false, "createdAt": now, "updatedAt": now,
	}
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user", Data: jsonMap(data),
	}))
	require.NoError(t, err)

	_, err = h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user", Data: jsonMap(data),
	}))
	require.Error(t, err, "second create with same email must fail")
}

// TestCreate_userEmailVerifiedDefaultFalse verifies that emailVerified comes
// back as 0 (false) when explicitly sent as false — matching schema default.
func TestCreate_userEmailVerifiedDefaultFalse(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name": "DefaultVerify", "email": "dv@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	require.Equal(t, float64(0), resp.Msg.Data["emailVerified"].GetNumberValue())
}

// TestCreate_userImageOptional verifies that omitting the optional image field
// does not cause an error and the response carries a null image.
func TestCreate_userImageOptional(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "user",
		Data: jsonMap(map[string]any{
			"name": "NoImage", "email": "noimg@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	// image key is present with a null value
	imageVal := resp.Msg.Data["image"]
	require.NotNil(t, imageVal)
	_, isNull := imageVal.Kind.(*structpb.Value_NullValue)
	require.True(t, isNull)
}

// --- session: schema constraints ---

// TestCreate_sessionDuplicateToken verifies the token UNIQUE constraint.
func TestCreate_sessionDuplicateToken(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "TokUser", "tokuser@example.com")

	data := map[string]any{
		"userId": userID, "token": "dup-token",
		"expiresAt": now + 3600, "createdAt": now, "updatedAt": now,
	}
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session", Data: jsonMap(data),
	}))
	require.NoError(t, err)

	_, err = h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session", Data: jsonMap(data),
	}))
	require.Error(t, err, "second create with same token must fail")
}

// TestDelete_userCascadesSessions verifies the session.userId onDelete cascade:
// deleting a user must also remove all their sessions.
func TestDelete_userCascadesSessions(t *testing.T) {
	h, cleanup := newHandlerWithFK(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "CascadeUser", "cascade@example.com")

	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId": userID, "token": "cascade-tok",
			"expiresAt": now + 3600, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	_, err = h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(userID))},
	}))
	require.NoError(t, err)

	listResp, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "session",
		Where: []*auth_adapterv1.Where{eqWhere("userId", structpb.NewStringValue(userID))},
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Empty(t, listResp.Msg.Items, "sessions must be cascade-deleted with the user")
}

// TestSession_optionalFieldsNullWhenAbsent verifies that ipAddress and
// userAgent are null in the response when not supplied.
func TestSession_optionalFieldsNullWhenAbsent(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "NullFields", "nullfields@example.com")

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonMap(map[string]any{
			"userId": userID, "token": "no-meta-tok",
			"expiresAt": now + 3600, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data

	for _, f := range []string{"ipAddress", "userAgent"} {
		v, ok := fields[f]
		require.True(t, ok, "%s key must be present", f)
		_, isNull := v.Kind.(*structpb.Value_NullValue)
		require.True(t, isNull, "%s must be null when not supplied", f)
	}
}

// --- account: schema constraints ---

// TestDelete_userCascadesAccounts verifies the account.userId onDelete cascade.
func TestDelete_userCascadesAccounts(t *testing.T) {
	h, cleanup := newHandlerWithFK(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "AccCascade", "acccascade@example.com")

	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId": userID, "accountId": "gh-cascade", "providerId": "github",
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)

	_, err = h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "user",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue(userID))},
	}))
	require.NoError(t, err)

	listResp, err := h.FindMany(context.Background(), connect.NewRequest(&auth_adapterv1.FindManyRequest{
		Model: "account",
		Where: []*auth_adapterv1.Where{eqWhere("userId", structpb.NewStringValue(userID))},
		Limit: 10,
	}))
	require.NoError(t, err)
	require.Empty(t, listResp.Msg.Items, "accounts must be cascade-deleted with the user")
}

// TestCreate_accountCredentialProvider verifies the password field (credential
// provider path — BetterAuth stores hashed passwords on the account row).
func TestCreate_accountCredentialProvider(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "Cred", "cred@example.com")

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId": userID, "accountId": "cred@example.com",
			"providerId": "credential",
			"password":   "hashed-pw",
			"createdAt":  now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	// password is present in the map (returned: false only means BetterAuth strips
	// it in the HTTP layer — the adapter itself still stores and returns it)
	require.Equal(t, "hashed-pw", resp.Msg.Data["password"].GetStringValue())
}

// TestAccount_sensitiveFieldsNullWhenAbsent verifies that all optional
// "returned: false" token fields are null in the response when not supplied.
func TestAccount_sensitiveFieldsNullWhenAbsent(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	userID := createUserFixture(t, h, "SensFields", "sens@example.com")

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "account",
		Data: jsonMap(map[string]any{
			"userId": userID, "accountId": "sens-gh", "providerId": "github",
			"createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data

	for _, f := range []string{"accessToken", "refreshToken", "idToken", "accessTokenExpiresAt", "refreshTokenExpiresAt", "scope", "password"} {
		v, ok := fields[f]
		require.True(t, ok, "%s key must be present", f)
		_, isNull := v.Kind.(*structpb.Value_NullValue)
		require.True(t, isNull, "%s must be null when not supplied", f)
	}
}

// --- jwks: not yet implemented ---

func TestCreate_jwks_unimplemented(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	now := float64(time.Now().Unix())
	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "jwks",
		Data: jsonMap(map[string]any{
			"publicKey": "pub", "privateKey": "priv", "createdAt": now,
		}),
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestFind_jwks_unimplemented(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.Find(context.Background(), connect.NewRequest(&auth_adapterv1.FindRequest{
		Model: "jwks",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue("x"))},
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestDelete_jwks_unimplemented(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.Delete(context.Background(), connect.NewRequest(&auth_adapterv1.DeleteRequest{
		Model: "jwks",
		Where: []*auth_adapterv1.Where{eqWhere("id", structpb.NewStringValue("x"))},
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}
