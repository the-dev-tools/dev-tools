package auth

import (
	"context"
	"database/sql"
	"devtools-backend/internal/api"
	"devtools-backend/pkg/model/morg"
	"devtools-backend/pkg/model/morguser"
	"devtools-backend/pkg/model/muser"
	"devtools-backend/pkg/service/sorg"
	"devtools-backend/pkg/service/sorguser"
	"devtools-backend/pkg/service/suser"
	"devtools-backend/pkg/stoken"
	authv1 "devtools-services/gen/auth/v1"
	"devtools-services/gen/auth/v1/authv1connect"
	"encoding/hex"
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
	ClientAPI  *client.API
	HmacSecret []byte
}

func (a *AuthServer) DID(ctx context.Context, req *connect.Request[authv1.AuthServiceDIDRequest]) (*connect.Response[authv1.AuthServiceDIDResponse], error) {
	if req.Msg.DidToken == "" {
		return nil, errors.New("did token is required")
	}
	tk, err := token.NewToken(req.Msg.DidToken)
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

	user, err := suser.GetUserWithOAuthIDAndType(publicAddress, muser.MagicLink)
	if err != nil {
		if err == sql.ErrNoRows {
			org := &morg.Org{
				ID:   ulid.Make(),
				Name: fmt.Sprintf("%s's org", email),
			}
			err = sorg.CreateOrg(org)
			if err != nil {
				return nil, err
			}

			user := &muser.User{
				ID:        ulid.Make(),
				Email:     email,
				Password:  nil,
				OAuthType: muser.MagicLink,
				OAuthID:   publicAddress,
			}

			err = suser.CreateUser(user)
			if err != nil {
				return nil, err
			}

			orgUser := &morguser.OrgUser{
				ID:     ulid.Make(),
				OrgID:  org.ID,
				UserID: user.ID,
			}

			err = sorguser.CreateOrgUser(orgUser)
			if err != nil {
				return nil, err
			}

			_, err = suser.GetUserWithOAuthIDAndType(publicAddress, muser.MagicLink)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	str := hex.EncodeToString(user.ID.Bytes())
	fmt.Println("userID: ", str)

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
	if req.Msg.RefreshToken == "" {
		// connect invalid token error
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}

	jwtToken, err := stoken.ValidateJWT(req.Msg.RefreshToken, stoken.RefreshToken, a.HmacSecret)
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

	fmt.Println(ulidID, claims.Email)

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

func CreateService(secret []byte) (*api.Service, error) {
	magicLinkSecret := os.Getenv("MAGIC_LINK_SECRET")
	if magicLinkSecret == "" {
		return nil, errors.New("MAGIC_LINK_SECRET env var is required")
	}

	cl := magic.NewClientWithRetry(5, time.Second, 10*time.Second)
	m, err := client.New(magicLinkSecret, cl)
	if err != nil {
		return nil, err
	}

	server := &AuthServer{
		ClientAPI:  m,
		HmacSecret: secret,
	}
	path, handler := authv1connect.NewAuthServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
