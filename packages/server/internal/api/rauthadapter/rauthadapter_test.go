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
	auth_adapterv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_adapter/v1"
)

// newHandler returns a ready-to-use AuthAdapterRPC backed by an in-memory SQLite DB.
func newHandler(t *testing.T) (AuthAdapterRPC, func()) {
	t.Helper()
	base := testutil.CreateBaseDB(context.Background(), t)
	adapter := authadapter.New(base.Queries)
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
		Data: jsonValue(map[string]any{
			"name":          "Alice",
			"email":         "alice@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)

	fields := resp.Msg.Data.GetStructValue().GetFields()
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
		Data: jsonValue(map[string]any{
			"name":          "Bob",
			"email":         "bob@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	userID := userResp.Msg.Data.GetStructValue().GetFields()["id"].GetStringValue()

	resp, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonValue(map[string]any{
			"userId":    userID,
			"token":     "tok-abc",
			"expiresAt": now + 3600,
			"createdAt": now,
			"updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	fields := resp.Msg.Data.GetStructValue().GetFields()
	require.Equal(t, "tok-abc", fields["token"].GetStringValue())
}

func TestCreate_unsupportedModel(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "unknown",
		Data:  jsonValue(map[string]any{}),
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
		Data: jsonValue(map[string]any{
			"name":          "Carol",
			"email":         "carol@example.com",
			"emailVerified": false,
			"createdAt":     now,
			"updatedAt":     now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data.GetStructValue().GetFields()["id"].GetStringValue()

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
		Data: jsonValue(map[string]any{
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
		Data: jsonValue(map[string]any{
			"name": "Eve", "email": "eve@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	userID := userResp.Msg.Data.GetStructValue().GetFields()["id"].GetStringValue()

	for _, tok := range []string{"tok-1", "tok-2"} {
		_, err := h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
			Model: "session",
			Data: jsonValue(map[string]any{
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
		Data: jsonValue(map[string]any{
			"name": "Frank", "email": "frank@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data.GetStructValue().GetFields()["id"].GetStringValue()

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
		Data: jsonValue(map[string]any{
			"name": "Grace", "email": "grace@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	id := createResp.Msg.Data.GetStructValue().GetFields()["id"].GetStringValue()

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
		Data: jsonValue(map[string]any{
			"name": "Hank", "email": "hank@example.com",
			"emailVerified": false, "createdAt": now, "updatedAt": now,
		}),
	}))
	require.NoError(t, err)
	userID := userResp.Msg.Data.GetStructValue().GetFields()["id"].GetStringValue()

	// Create 2 sessions: one expired, one not
	_, err = h.Create(context.Background(), connect.NewRequest(&auth_adapterv1.CreateRequest{
		Model: "session",
		Data: jsonValue(map[string]any{
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
		Data: jsonValue(map[string]any{
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
			Data: jsonValue(map[string]any{
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

func TestUpdateMany_returnsUnimplemented(t *testing.T) {
	h, cleanup := newHandler(t)
	defer cleanup()

	_, err := h.UpdateMany(context.Background(), connect.NewRequest(&auth_adapterv1.UpdateManyRequest{
		Model: "user",
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeUnimplemented, connectErr.Code())
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
