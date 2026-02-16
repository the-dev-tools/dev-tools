//nolint:revive // exported
package ruser

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	authinternalv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/user/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/user/v1/userv1connect"
)

const (
	eventTypeUpdate = "update"
	eventTypeInsert = "insert"
	eventTypeDelete = "delete"
)

type UserTopic struct {
	UserID idwrap.IDWrap
}

type UserEvent struct {
	Type string
	User *apiv1.User
}

type LinkedAccountTopic struct {
	UserID idwrap.IDWrap
}

type LinkedAccountEvent struct {
	Type          string
	LinkedAccount *apiv1.LinkedAccount
}

type UserServiceRPC struct {
	DB         *sql.DB
	us         suser.UserService
	stream     eventstream.SyncStreamer[UserTopic, UserEvent]
	authClient auth_internalv1connect.AuthInternalServiceClient // nil in local mode

	linkedAccountStream eventstream.SyncStreamer[LinkedAccountTopic, LinkedAccountEvent]
}

type UserServiceRPCDeps struct {
	DB         *sql.DB
	User       suser.UserService
	Streamer   eventstream.SyncStreamer[UserTopic, UserEvent]
	AuthClient auth_internalv1connect.AuthInternalServiceClient // nil in local mode

	LinkedAccountStreamer eventstream.SyncStreamer[LinkedAccountTopic, LinkedAccountEvent]
}

func (d *UserServiceRPCDeps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if d.Streamer == nil {
		return fmt.Errorf("streamer is required")
	}
	if d.LinkedAccountStreamer == nil {
		return fmt.Errorf("linked account streamer is required")
	}
	return nil
}

func New(deps UserServiceRPCDeps) UserServiceRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("UserServiceRPC Deps validation failed: %v", err))
	}

	return UserServiceRPC{
		DB:         deps.DB,
		us:         deps.User,
		stream:     deps.Streamer,
		authClient: deps.AuthClient,

		linkedAccountStream: deps.LinkedAccountStreamer,
	}
}

func CreateService(srv UserServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := userv1connect.NewUserServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func toAPIUser(u *muser.User) *apiv1.User {
	return &apiv1.User{
		UserId: u.ID.Bytes(),
		Email:  u.Email,
		Name:   u.Name,
		Image:  u.Image,
	}
}

func (c *UserServiceRPC) UserCollection(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.UserCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	user, err := c.us.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, suser.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&apiv1.UserCollectionResponse{
		Items: []*apiv1.User{toAPIUser(user)},
	}), nil
}

func (c *UserServiceRPC) UserInsert(_ context.Context, _ *connect.Request[apiv1.UserInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("user insert is not supported"))
}

func (c *UserServiceRPC) UserUpdate(ctx context.Context, req *connect.Request[apiv1.UserUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// FETCH: Read user outside transaction
	user, err := c.us.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, suser.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// CHECK: Only allow updating own user record
	item := req.Msg.Items[0]
	if len(item.UserId) > 0 {
		requestedID, err := idwrap.NewFromBytes(item.UserId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if requestedID.Compare(userID) != 0 {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only update own user record"))
		}
	}

	// Apply updates
	if item.Name != nil {
		user.Name = *item.Name
	}
	if item.Image != nil {
		switch item.Image.Kind {
		case apiv1.UserUpdate_ImageUnion_KIND_VALUE:
			user.Image = item.Image.Value
		case apiv1.UserUpdate_ImageUnion_KIND_UNSET:
			user.Image = nil
		}
	}

	// ACT: Write inside transaction
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	writer := suser.NewWriter(tx)
	if err := writer.UpdateUser(ctx, user); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// NOTIFY
	c.stream.Publish(UserTopic{UserID: userID}, UserEvent{
		Type: eventTypeUpdate,
		User: toAPIUser(user),
	})

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (c *UserServiceRPC) UserSync(ctx context.Context, _ *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.UserSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return c.streamUserSync(ctx, userID, stream.Send)
}

func (c *UserServiceRPC) streamUserSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.UserSyncResponse) error) error {
	filter := func(topic UserTopic) bool {
		return topic.UserID.Compare(userID) == 0
	}

	events, err := c.stream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := userSyncResponseFrom(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *UserServiceRPC) LinkedAccountCollection(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.LinkedAccountCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if c.authClient == nil {
		return connect.NewResponse(&apiv1.LinkedAccountCollectionResponse{}), nil
	}

	// Look up the user to get their ExternalID (BetterAuth user ID)
	user, err := c.us.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, suser.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Local-mode users have no external ID — return empty
	if user.ExternalID == nil {
		return connect.NewResponse(&apiv1.LinkedAccountCollectionResponse{}), nil
	}

	externalULID, err := idwrap.NewText(*user.ExternalID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid external ID: %w", err))
	}

	resp, err := c.authClient.AccountsByUserId(ctx, connect.NewRequest(&authinternalv1.AccountsByUserIdRequest{
		UserId: externalULID.Bytes(),
	}))
	if err != nil {
		if connectErr := new(connect.Error); errors.As(err, &connectErr) {
			return nil, connectErr
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.LinkedAccount, 0, len(resp.Msg.Accounts))
	for _, acc := range resp.Msg.Accounts {
		items = append(items, &apiv1.LinkedAccount{
			AccountId: acc.Id,
			UserId:    acc.UserId,
			Provider:  acc.Provider,
		})
	}

	return connect.NewResponse(&apiv1.LinkedAccountCollectionResponse{
		Items: items,
	}), nil
}

func (c *UserServiceRPC) LinkedAccountSync(ctx context.Context, _ *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.LinkedAccountSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return c.streamLinkedAccountSync(ctx, userID, stream.Send)
}

func (c *UserServiceRPC) streamLinkedAccountSync(ctx context.Context, userID idwrap.IDWrap, send func(*apiv1.LinkedAccountSyncResponse) error) error {
	filter := func(topic LinkedAccountTopic) bool {
		return topic.UserID.Compare(userID) == 0
	}

	events, err := c.linkedAccountStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := linkedAccountSyncResponseFrom(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func userSyncResponseFrom(evt UserEvent) *apiv1.UserSyncResponse {
	if evt.User == nil {
		return nil
	}

	switch evt.Type {
	case eventTypeUpdate:
		update := &apiv1.UserSyncUpdate{
			UserId: evt.User.UserId,
			Name:   &evt.User.Name,
		}
		if evt.User.Image != nil {
			update.Image = &apiv1.UserSyncUpdate_ImageUnion{
				Kind:  apiv1.UserSyncUpdate_ImageUnion_KIND_VALUE,
				Value: evt.User.Image,
			}
		} else {
			update.Image = &apiv1.UserSyncUpdate_ImageUnion{
				Kind: apiv1.UserSyncUpdate_ImageUnion_KIND_UNSET,
			}
		}
		msg := &apiv1.UserSync{
			Value: &apiv1.UserSync_ValueUnion{
				Kind:   apiv1.UserSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
		return &apiv1.UserSyncResponse{Items: []*apiv1.UserSync{msg}}
	default:
		return nil
	}
}

func linkedAccountSyncResponseFrom(evt LinkedAccountEvent) *apiv1.LinkedAccountSyncResponse {
	if evt.LinkedAccount == nil {
		return nil
	}

	var msg *apiv1.LinkedAccountSync

	switch evt.Type {
	case eventTypeInsert:
		msg = &apiv1.LinkedAccountSync{
			Value: &apiv1.LinkedAccountSync_ValueUnion{
				Kind: apiv1.LinkedAccountSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.LinkedAccountSyncInsert{
					AccountId: evt.LinkedAccount.AccountId,
					UserId:    evt.LinkedAccount.UserId,
					Provider:  evt.LinkedAccount.Provider,
				},
			},
		}
	case eventTypeDelete:
		msg = &apiv1.LinkedAccountSync{
			Value: &apiv1.LinkedAccountSync_ValueUnion{
				Kind: apiv1.LinkedAccountSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.LinkedAccountSyncDelete{
					AccountId: evt.LinkedAccount.AccountId,
				},
			},
		}
	default:
		return nil
	}

	return &apiv1.LinkedAccountSyncResponse{Items: []*apiv1.LinkedAccountSync{msg}}
}
