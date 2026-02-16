package ruser

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	authcommonv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_common/v1"
	authinternalv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/user/v1"
)

// --- mock auth client ---

type mockAuthInternalClient struct {
	auth_internalv1connect.UnimplementedAuthInternalServiceHandler

	accountsByUserIdResp *authinternalv1.AccountsByUserIdResponse
	accountsByUserIdErr  error
}

func (m *mockAuthInternalClient) AccountsByUserId(_ context.Context, req *connect.Request[authinternalv1.AccountsByUserIdRequest]) (*connect.Response[authinternalv1.AccountsByUserIdResponse], error) {
	if m.accountsByUserIdErr != nil {
		return nil, m.accountsByUserIdErr
	}
	return connect.NewResponse(m.accountsByUserIdResp), nil
}

// --- test fixture ---

type userFixture struct {
	ctx                 context.Context
	base                *testutil.BaseDBQueries
	handler             UserServiceRPC
	userStream          eventstream.SyncStreamer[UserTopic, UserEvent]
	linkedAccountStream eventstream.SyncStreamer[LinkedAccountTopic, LinkedAccountEvent]
	userID              idwrap.IDWrap
}

func newUserFixture(t *testing.T) *userFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()

	userStream := memory.NewInMemorySyncStreamer[UserTopic, UserEvent]()
	linkedAccountStream := memory.NewInMemorySyncStreamer[LinkedAccountTopic, LinkedAccountEvent]()
	t.Cleanup(userStream.Shutdown)
	t.Cleanup(linkedAccountStream.Shutdown)

	userID := idwrap.NewNow()
	externalID := idwrap.NewNow().String()
	err := services.UserService.CreateUser(context.Background(), &muser.User{
		ID:         userID,
		Email:      "test@example.com",
		Name:       "Test User",
		ExternalID: &externalID,
	})
	require.NoError(t, err, "create user")

	authCtx := mwauth.CreateAuthedContext(context.Background(), userID)

	handler := New(UserServiceRPCDeps{
		DB:                    base.DB,
		User:                  services.UserService,
		Streamer:              userStream,
		LinkedAccountStreamer: linkedAccountStream,
		AuthClient:            &mockAuthInternalClient{},
	})

	t.Cleanup(base.Close)

	return &userFixture{
		ctx:                 authCtx,
		base:                base,
		handler:             handler,
		userStream:          userStream,
		linkedAccountStream: linkedAccountStream,
		userID:              userID,
	}
}

// --- helpers ---

func collectUserSyncItems(t *testing.T, ch <-chan *apiv1.UserSyncResponse, count int) []*apiv1.UserSync {
	t.Helper()
	var items []*apiv1.UserSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed before collecting %d items", count)
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
				}
				if len(items) == count {
					break
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items", "timeout waiting for %d user sync items, collected %d", count, len(items))
		}
	}
	return items
}

func collectLinkedAccountSyncItems(t *testing.T, ch <-chan *apiv1.LinkedAccountSyncResponse, count int) []*apiv1.LinkedAccountSync {
	t.Helper()
	var items []*apiv1.LinkedAccountSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			require.True(t, ok, "channel closed before collecting %d items", count)
			for _, item := range resp.GetItems() {
				if item != nil {
					items = append(items, item)
				}
				if len(items) == count {
					break
				}
			}
		case <-timeout:
			require.FailNow(t, "timeout waiting for items", "timeout waiting for %d linked account sync items, collected %d", count, len(items))
		}
	}
	return items
}

// --- UserCollection tests ---

func TestUserCollection(t *testing.T) {
	f := newUserFixture(t)

	resp, err := f.handler.UserCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, f.userID.Bytes(), resp.Msg.Items[0].UserId)
	require.Equal(t, "Test User", resp.Msg.Items[0].Name)
}

func TestUserCollection_unauthenticated(t *testing.T) {
	f := newUserFixture(t)

	_, err := f.handler.UserCollection(context.Background(), connect.NewRequest(&emptypb.Empty{}))
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.True(t, errors.As(err, &connectErr))
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestUserCollection_notFound(t *testing.T) {
	f := newUserFixture(t)

	otherID := idwrap.NewNow()
	ctx := mwauth.CreateAuthedContext(context.Background(), otherID)

	_, err := f.handler.UserCollection(ctx, connect.NewRequest(&emptypb.Empty{}))
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.True(t, errors.As(err, &connectErr))
	require.Equal(t, connect.CodeNotFound, connectErr.Code())
}

// --- UserUpdate tests ---

func TestUserUpdate_name(t *testing.T) {
	f := newUserFixture(t)

	newName := "Updated Name"
	req := connect.NewRequest(&apiv1.UserUpdateRequest{
		Items: []*apiv1.UserUpdate{
			{
				UserId: f.userID.Bytes(),
				Name:   &newName,
			},
		},
	})

	_, err := f.handler.UserUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify via collection
	resp, err := f.handler.UserCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Equal(t, "Updated Name", resp.Msg.Items[0].Name)
}

func TestUserUpdate_image(t *testing.T) {
	f := newUserFixture(t)

	imageURL := "https://example.com/avatar.png"
	req := connect.NewRequest(&apiv1.UserUpdateRequest{
		Items: []*apiv1.UserUpdate{
			{
				UserId: f.userID.Bytes(),
				Image: &apiv1.UserUpdate_ImageUnion{
					Kind:  apiv1.UserUpdate_ImageUnion_KIND_VALUE,
					Value: &imageURL,
				},
			},
		},
	})

	_, err := f.handler.UserUpdate(f.ctx, req)
	require.NoError(t, err)

	// Verify via collection
	resp, err := f.handler.UserCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Items[0].Image)
	require.Equal(t, imageURL, *resp.Msg.Items[0].Image)
}

func TestUserUpdate_wrongUser(t *testing.T) {
	f := newUserFixture(t)

	otherID := idwrap.NewNow()
	newName := "Hacked"
	req := connect.NewRequest(&apiv1.UserUpdateRequest{
		Items: []*apiv1.UserUpdate{
			{
				UserId: otherID.Bytes(),
				Name:   &newName,
			},
		},
	})

	_, err := f.handler.UserUpdate(f.ctx, req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.True(t, errors.As(err, &connectErr))
	require.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

// --- UserSync tests ---

func TestUserSync_streamsUpdates(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.UserSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamUserSync(ctx, f.userID, func(resp *apiv1.UserSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for subscription to be active
	time.Sleep(50 * time.Millisecond)

	// Trigger an update
	newName := "Streamed Name"
	req := connect.NewRequest(&apiv1.UserUpdateRequest{
		Items: []*apiv1.UserUpdate{
			{
				UserId: f.userID.Bytes(),
				Name:   &newName,
			},
		},
	})
	_, err := f.handler.UserUpdate(f.ctx, req)
	require.NoError(t, err)

	items := collectUserSyncItems(t, msgCh, 1)
	val := items[0].GetValue()
	require.NotNil(t, val)
	require.Equal(t, apiv1.UserSync_ValueUnion_KIND_UPDATE, val.GetKind())
	require.Equal(t, "Streamed Name", val.GetUpdate().GetName())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestUserSync_noSnapshotOnConnect(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.UserSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamUserSync(ctx, f.userID, func(resp *apiv1.UserSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good — stream active, no snapshot sent
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestUserSync_filtersOtherUsers(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.UserSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamUserSync(ctx, f.userID, func(resp *apiv1.UserSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for subscription to be active
	time.Sleep(50 * time.Millisecond)

	// Publish event for a different user
	otherID := idwrap.NewNow()
	f.userStream.Publish(UserTopic{UserID: otherID}, UserEvent{
		Type: eventTypeUpdate,
		User: &apiv1.User{UserId: otherID.Bytes(), Name: "Other"},
	})

	select {
	case <-msgCh:
		require.FailNow(t, "received event for another user")
	case <-time.After(100 * time.Millisecond):
		// Good — filtered
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestUserSync_unauthenticated(t *testing.T) {
	f := newUserFixture(t)

	err := f.handler.UserSync(context.Background(), connect.NewRequest(&emptypb.Empty{}), nil)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.True(t, errors.As(err, &connectErr))
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

// --- LinkedAccountCollection tests ---

func TestLinkedAccountCollection(t *testing.T) {
	t.Run("returns only current user accounts", func(t *testing.T) {
		testAccID1 := idwrap.NewNow()
		testAccID2 := idwrap.NewNow()

		f := newUserFixture(t)
		f.handler.authClient = &mockAuthInternalClient{
			accountsByUserIdResp: &authinternalv1.AccountsByUserIdResponse{
				Accounts: []*authinternalv1.Account{
					{
						Id:                testAccID1.Bytes(),
						UserId:            f.userID.Bytes(),
						Provider:          authcommonv1.AuthProvider_AUTH_PROVIDER_EMAIL,
						ProviderAccountId: "email-id",
					},
					{
						Id:                testAccID2.Bytes(),
						UserId:            f.userID.Bytes(),
						Provider:          authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
						ProviderAccountId: "google-id",
					},
				},
			},
		}

		resp, err := f.handler.LinkedAccountCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Len(t, resp.Msg.Items, 2)

		require.True(t, bytes.Equal(resp.Msg.Items[0].AccountId, testAccID1.Bytes()))
		require.True(t, bytes.Equal(resp.Msg.Items[0].UserId, f.userID.Bytes()))
		require.Equal(t, authcommonv1.AuthProvider_AUTH_PROVIDER_EMAIL, resp.Msg.Items[0].Provider)

		require.True(t, bytes.Equal(resp.Msg.Items[1].AccountId, testAccID2.Bytes()))
		require.Equal(t, authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE, resp.Msg.Items[1].Provider)
	})

	t.Run("user with no external ID returns empty", func(t *testing.T) {
		base := testutil.CreateBaseDB(context.Background(), t)
		services := base.GetBaseServices()

		userStream := memory.NewInMemorySyncStreamer[UserTopic, UserEvent]()
		linkedAccountStream := memory.NewInMemorySyncStreamer[LinkedAccountTopic, LinkedAccountEvent]()
		t.Cleanup(userStream.Shutdown)
		t.Cleanup(linkedAccountStream.Shutdown)

		// Create user WITHOUT ExternalID (local-mode user)
		localUserID := idwrap.NewNow()
		err := services.UserService.CreateUser(context.Background(), &muser.User{
			ID:    localUserID,
			Email: "local@example.com",
			Name:  "Local User",
		})
		require.NoError(t, err)

		authCtx := mwauth.CreateAuthedContext(context.Background(), localUserID)
		handler := New(UserServiceRPCDeps{
			DB:                    base.DB,
			User:                  services.UserService,
			Streamer:              userStream,
			LinkedAccountStreamer: linkedAccountStream,
			AuthClient:            &mockAuthInternalClient{},
		})
		t.Cleanup(base.Close)

		resp, err := handler.LinkedAccountCollection(authCtx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Empty(t, resp.Msg.Items)
	})

	t.Run("local mode returns empty items", func(t *testing.T) {
		f := newUserFixture(t)
		f.handler.authClient = nil

		resp, err := f.handler.LinkedAccountCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Empty(t, resp.Msg.Items)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		f := newUserFixture(t)

		_, err := f.handler.LinkedAccountCollection(context.Background(), connect.NewRequest(&emptypb.Empty{}))
		require.Error(t, err)

		connectErr := new(connect.Error)
		require.True(t, errors.As(err, &connectErr))
		require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("backend error", func(t *testing.T) {
		f := newUserFixture(t)
		f.handler.authClient = &mockAuthInternalClient{
			accountsByUserIdErr: connect.NewError(connect.CodeUnavailable, errors.New("auth service down")),
		}

		_, err := f.handler.LinkedAccountCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.Error(t, err)

		connectErr := new(connect.Error)
		require.True(t, errors.As(err, &connectErr))
		require.Equal(t, connect.CodeUnavailable, connectErr.Code())
	})

	t.Run("empty accounts", func(t *testing.T) {
		f := newUserFixture(t)
		f.handler.authClient = &mockAuthInternalClient{
			accountsByUserIdResp: &authinternalv1.AccountsByUserIdResponse{
				Accounts: []*authinternalv1.Account{},
			},
		}

		resp, err := f.handler.LinkedAccountCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
		require.NoError(t, err)
		require.Empty(t, resp.Msg.Items)
	})
}

// --- LinkedAccountSync tests ---

func TestLinkedAccountSync_streamsInsert(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.LinkedAccountSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamLinkedAccountSync(ctx, f.userID, func(resp *apiv1.LinkedAccountSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for subscription to be active
	time.Sleep(50 * time.Millisecond)

	accID := idwrap.NewNow()
	f.linkedAccountStream.Publish(LinkedAccountTopic{UserID: f.userID}, LinkedAccountEvent{
		Type: eventTypeInsert,
		LinkedAccount: &apiv1.LinkedAccount{
			AccountId: accID.Bytes(),
			UserId:    f.userID.Bytes(),
			Provider:  authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
		},
	})

	items := collectLinkedAccountSyncItems(t, msgCh, 1)
	val := items[0].GetValue()
	require.NotNil(t, val)
	require.Equal(t, apiv1.LinkedAccountSync_ValueUnion_KIND_INSERT, val.GetKind())
	require.True(t, bytes.Equal(accID.Bytes(), val.GetInsert().GetAccountId()))
	require.True(t, bytes.Equal(f.userID.Bytes(), val.GetInsert().GetUserId()))
	require.Equal(t, authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE, val.GetInsert().GetProvider())

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestLinkedAccountSync_streamsDelete(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.LinkedAccountSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamLinkedAccountSync(ctx, f.userID, func(resp *apiv1.LinkedAccountSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for subscription to be active
	time.Sleep(50 * time.Millisecond)

	accID := idwrap.NewNow()
	f.linkedAccountStream.Publish(LinkedAccountTopic{UserID: f.userID}, LinkedAccountEvent{
		Type: eventTypeDelete,
		LinkedAccount: &apiv1.LinkedAccount{
			AccountId: accID.Bytes(),
		},
	})

	items := collectLinkedAccountSyncItems(t, msgCh, 1)
	val := items[0].GetValue()
	require.NotNil(t, val)
	require.Equal(t, apiv1.LinkedAccountSync_ValueUnion_KIND_DELETE, val.GetKind())
	require.True(t, bytes.Equal(accID.Bytes(), val.GetDelete().GetAccountId()))

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestLinkedAccountSync_noSnapshotOnConnect(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.LinkedAccountSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamLinkedAccountSync(ctx, f.userID, func(resp *apiv1.LinkedAccountSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good — stream active, no snapshot sent
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestLinkedAccountSync_filtersOtherUsers(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *apiv1.LinkedAccountSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamLinkedAccountSync(ctx, f.userID, func(resp *apiv1.LinkedAccountSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for subscription to be active
	time.Sleep(50 * time.Millisecond)

	// Publish event for a different user
	otherID := idwrap.NewNow()
	f.linkedAccountStream.Publish(LinkedAccountTopic{UserID: otherID}, LinkedAccountEvent{
		Type: eventTypeInsert,
		LinkedAccount: &apiv1.LinkedAccount{
			AccountId: idwrap.NewNow().Bytes(),
			UserId:    otherID.Bytes(),
			Provider:  authcommonv1.AuthProvider_AUTH_PROVIDER_EMAIL,
		},
	})

	select {
	case <-msgCh:
		require.FailNow(t, "received event for another user")
	case <-time.After(100 * time.Millisecond):
		// Good — filtered
	}

	cancel()
	err := <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestLinkedAccountSync_unauthenticated(t *testing.T) {
	f := newUserFixture(t)

	err := f.handler.LinkedAccountSync(context.Background(), connect.NewRequest(&emptypb.Empty{}), nil)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.True(t, errors.As(err, &connectErr))
	require.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
}

func TestLinkedAccountSync_blocksUntilEvent(t *testing.T) {
	f := newUserFixture(t)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- f.handler.streamLinkedAccountSync(ctx, f.userID, func(_ *apiv1.LinkedAccountSyncResponse) error {
			return nil
		})
	}()

	// Stream should stay open with no events
	select {
	case <-errCh:
		require.FailNow(t, "stream returned before context cancellation")
	case <-time.After(100 * time.Millisecond):
		// Good — still blocking
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			require.ErrorIs(t, err, context.Canceled)
		}
	case <-time.After(time.Second):
		require.FailNow(t, "stream did not return after context cancellation")
	}
}
