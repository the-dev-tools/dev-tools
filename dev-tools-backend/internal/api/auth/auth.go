package auth

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	"dev-tools-backend/pkg/stoken"
	authv1 "dev-tools-spec/dist/buf/go/auth/v1"
	"dev-tools-spec/dist/buf/go/auth/v1/authv1connect"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/magiclabs/magic-admin-go/client"
	"github.com/magiclabs/magic-admin-go/token"
)

var (
	AccessTokenTimeSpan  = time.Hour * 2
	RefreshTokenTimeSpan = time.Hour * 24 * 2
)

type AuthServer struct {
	ClientAPI   client.API
	userService suser.UserService
	ws          sworkspace.WorkspaceService
	wus         sworkspacesusers.WorkspaceUserService
	HmacSecret  []byte
}

func New(client client.API, us suser.UserService, ws sworkspace.WorkspaceService, wus sworkspacesusers.WorkspaceUserService, secret []byte) AuthServer {
	return AuthServer{
		ClientAPI:   client,
		userService: us,
		ws:          ws,
		wus:         wus,
		HmacSecret:  secret,
	}
}

func CreateService(srv AuthServer, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := authv1connect.NewAuthServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (a *AuthServer) AuthMagicLink(ctx context.Context, req *connect.Request[authv1.AuthMagicLinkRequest]) (*connect.Response[authv1.AuthMagicLinkResponse], error) {
	if req.Msg.GetDidToken() == "" {
		return nil, errors.New("did token is required")
	}
	tk, err := token.NewToken(req.Msg.GetDidToken())
	if err != nil {
		return nil, err
	}
	err = tk.Validate(a.ClientAPI.ClientInfo.ClientId)
	if err != nil {
		return nil, err
	}

	publicAddress, err := tk.GetPublicAddress()
	if err != nil {
		return nil, err
	}

	userInfo, err := a.ClientAPI.User.GetMetadataByPublicAddress(publicAddress)
	if err != nil {
		return nil, err
	}

	email := userInfo.Email

	user, err := a.userService.GetUserWithOAuthIDAndType(ctx, publicAddress, muser.MagicLink)
	// TODO: make it simpler
	if err != nil {
		if err == sql.ErrNoRows {
			tempUser, err := a.GetPendingUserByEmail(ctx, email)
			if err != nil {
				if err == sql.ErrNoRows {
					tempUser, err = a.handleUserNotFound(ctx, email, publicAddress, muser.MagicLink)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, err
				}
			}
			tempUser.ProviderID = &publicAddress
			tempUser.ProviderType = muser.MagicLink
			err = a.userService.UpdateUser(ctx, tempUser)
			if err != nil {
				return nil, err
			}
			user = tempUser
		} else {
			return nil, err
		}
	}

	jwtToken, err := stoken.NewJWT(user.ID, email, stoken.RefreshToken, RefreshTokenTimeSpan, a.HmacSecret)
	if err != nil {
		return nil, err
	}

	accessToken, err := stoken.NewJWT(user.ID, email, stoken.AccessToken, AccessTokenTimeSpan, a.HmacSecret)
	if err != nil {
		return nil, err
	}

	respRaw := &authv1.AuthMagicLinkResponse{
		RefreshToken: jwtToken,
		AccessToken:  accessToken,
	}

	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func (a *AuthServer) AuthRefresh(ctx context.Context, req *connect.Request[authv1.AuthRefreshRequest]) (*connect.Response[authv1.AuthRefreshResponse], error) {
	if req.Msg.GetRefreshToken() == "" {
		// connect invalid token error
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}

	claims, err := stoken.ValidateJWT(req.Msg.GetRefreshToken(), stoken.RefreshToken, a.HmacSecret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	userUlid, err := idwrap.NewWithParse(claims.Subject)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// generate new refresh token
	newRefreshJWT, err := stoken.NewJWT(userUlid, claims.Email, stoken.RefreshToken, time.Hour*24*2, a.HmacSecret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	newAccessJWT, err := stoken.NewJWT(userUlid, claims.Email, stoken.AccessToken, time.Hour*24*2, a.HmacSecret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.AuthRefreshResponse{RefreshToken: newRefreshJWT, AccessToken: newAccessJWT}), nil
}

func (a *AuthServer) GetPendingUserByEmail(ctx context.Context, email string) (*muser.User, error) {
	user, err := a.userService.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if user.Status == muser.Pending {
		return nil, errors.New("user is pending")
	}
	return user, nil
}

func (a AuthServer) handleUserNotFound(ctx context.Context, email, ProviderID string, ProviderType muser.ProviderType) (*muser.User, error) {
	org := &mworkspace.Workspace{
		ID:   idwrap.NewNow(),
		Name: fmt.Sprintf("%s's org", email),
	}
	err := a.ws.Create(ctx, org)
	if err != nil {
		return nil, err
	}

	user := &muser.User{
		ID:           idwrap.NewNow(),
		Email:        email,
		Password:     nil,
		ProviderType: ProviderType,
		ProviderID:   &ProviderID,
	}

	_, err = a.userService.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	orgUser := &mworkspaceuser.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: org.ID,
		UserID:      user.ID,
		Role:        mworkspaceuser.RoleOwner,
	}

	err = a.wus.CreateWorkspaceUser(ctx, orgUser)
	if err != nil {
		return nil, err
	}

	_, err = a.userService.GetUserWithOAuthIDAndType(ctx, ProviderID, muser.MagicLink)
	if err != nil {
		return nil, err
	}

	return user, nil
}
