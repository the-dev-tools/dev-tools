package auth

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/pkg/model/muser"
	"dev-tools-backend/pkg/model/mworkspace"
	"dev-tools-backend/pkg/model/mworkspaceuser"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/sworkspace"
	"dev-tools-backend/pkg/service/sworkspacesusers"
	"dev-tools-backend/pkg/stoken"
	authv1 "dev-tools-services/gen/auth/v1"
	"dev-tools-services/gen/auth/v1/authv1connect"
	"errors"
	"fmt"
	"os"
	"time"

	"connectrpc.com/connect"

	"github.com/magiclabs/magic-admin-go"
	"github.com/magiclabs/magic-admin-go/client"
	"github.com/magiclabs/magic-admin-go/token"
	"github.com/oklog/ulid/v2"
)

var (
	AccessTokenTimeSpan  = time.Hour * 2
	RefreshTokenTimeSpan = time.Hour * 24 * 2
)

type AuthServer struct {
	ClientAPI   *client.API
	HmacSecret  []byte
	userService suser.UserService
	ws          sworkspace.WorkspaceService
	wus         sworkspacesusers.WorkspaceUserService
}

func (a *AuthServer) DID(ctx context.Context, req *connect.Request[authv1.AuthServiceDIDRequest]) (*connect.Response[authv1.AuthServiceDIDResponse], error) {
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
			fmt.Println("pk", publicAddress)
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

	respRaw := &authv1.AuthServiceDIDResponse{
		RefreshToken: jwtToken,
		AccessToken:  accessToken,
	}

	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func (a *AuthServer) RefreshToken(ctx context.Context, req *connect.Request[authv1.AuthServiceRefreshTokenRequest]) (*connect.Response[authv1.AuthServiceRefreshTokenResponse], error) {
	if req.Msg.GetRefreshToken() == "" {
		// connect invalid token error
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}

	jwtToken, err := stoken.ValidateJWT(req.Msg.GetRefreshToken(), stoken.RefreshToken, a.HmacSecret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	claims, err := stoken.GetClaims(jwtToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ulidID, err := ulid.Parse(claims.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// generate new refresh token
	newRefreshJWT, err := stoken.NewJWT(ulidID, claims.Email, stoken.RefreshToken, time.Hour*24*2, a.HmacSecret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	newAccessJWT, err := stoken.NewJWT(ulidID, claims.Email, stoken.RefreshToken, time.Hour*24*2, a.HmacSecret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.AuthServiceRefreshTokenResponse{RefreshToken: newRefreshJWT, AccessToken: newAccessJWT}), nil
}

func CreateService(db *sql.DB, secret []byte) (*api.Service, error) {
	magicLinkSecret := os.Getenv("MAGIC_LINK_SECRET")
	if magicLinkSecret == "" {
		return nil, errors.New("MAGIC_LINK_SECRET env var is required")
	}

	ctx := context.Background()
	userService, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	ws, err := sworkspace.New(ctx, db)
	if err != nil {
		return nil, err
	}

	wus, err := sworkspacesusers.New(ctx, db)
	if err != nil {
		return nil, err
	}

	cl := magic.NewClientWithRetry(5, time.Second, 10*time.Second)
	m, err := client.New(magicLinkSecret, cl)
	if err != nil {
		return nil, err
	}

	server := &AuthServer{
		ClientAPI:   m,
		HmacSecret:  secret,
		userService: *userService,
		ws:          *ws,
		wus:         *wus,
	}
	path, handler := authv1connect.NewAuthServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
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
		ID:   ulid.Make(),
		Name: fmt.Sprintf("%s's org", email),
	}
	err := a.ws.Create(ctx, org)
	if err != nil {
		return nil, err
	}

	user := &muser.User{
		ID:           ulid.Make(),
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
		ID:          ulid.Make(),
		WorkspaceID: org.ID,
		UserID:      user.ID,
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
