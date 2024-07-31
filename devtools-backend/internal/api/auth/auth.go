package auth

import (
	"context"
	"devtools-backend/internal/api"
	"devtools-backend/pkg/stoken"
	authv1 "devtools-services/gen/auth/v1"
	"devtools-services/gen/auth/v1/authv1connect"
	"errors"
	"os"
	"time"

	"connectrpc.com/connect"

	"github.com/magiclabs/magic-admin-go"
	"github.com/magiclabs/magic-admin-go/client"
	"github.com/magiclabs/magic-admin-go/token"
)

type AuthServer struct {
	clientAPI  *client.API
	hmacSecret []byte
}

func (a *AuthServer) DID(ctx context.Context, req *connect.Request[authv1.AuthServiceDIDRequest]) (*connect.Response[authv1.AuthServiceDIDResponse], error) {
	if req.Msg.DidToken == "" {
		return nil, errors.New("did token is required")
	}
	tk, err := token.NewToken(req.Msg.DidToken)
	if err != nil {
		return nil, err
	}
	err = tk.Validate(a.clientAPI.ClientInfo.ClientId)
	if err != nil {
		return nil, err
	}

	publicAddress, err := tk.GetPublicAddress()
	if err != nil {
		return nil, err
	}

	jwtToken, err := stoken.NewJWT(publicAddress, stoken.RefreshToken, time.Hour*24*2, a.hmacSecret)
	if err != nil {
		return nil, err
	}

	respRaw := &authv1.AuthServiceDIDResponse{
		RefreshToken: jwtToken,
	}

	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func (a *AuthServer) RefreshToken(ctx context.Context, req *connect.Request[authv1.AuthServiceRefreshTokenRequest]) (*connect.Response[authv1.AuthServiceRefreshTokenResponse], error) {
	if req.Msg.RefreshToken == "" {
		// connect invalid token error
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}

	jwtToken, err := stoken.ValidateJWT(req.Msg.RefreshToken, stoken.RefreshToken, a.hmacSecret)

	subject, err := jwtToken.Claims.GetSubject()
	if err != nil {
		return nil, err
	}

	// generate new refresh token
	newJwtToken, err := stoken.NewJWT(subject, stoken.RefreshToken, time.Hour*24*2, a.hmacSecret)
	return connect.NewResponse(&authv1.AuthServiceRefreshTokenResponse{RefreshToken: newJwtToken}), nil
}

// AccessToken calls auth.v1.AuthService.AccessToken.
func (a *AuthServer) AccessToken(ctx context.Context, req *connect.Request[authv1.AuthServiceAccessTokenRequest]) (*connect.Response[authv1.AuthServiceAccessTokenResponse], error) {
	if req.Msg.RefreshToken == "" {
		// connect invalid token error
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}

	jwtToken, err := stoken.ValidateJWT(req.Msg.RefreshToken, stoken.AccessToken, a.hmacSecret)
	subject, err := jwtToken.Claims.GetSubject()
	if err != nil {
		return nil, err
	}

	// generate new refresh token
	newJwtToken, err := stoken.NewJWT(subject, stoken.RefreshToken, time.Hour*24*2, a.hmacSecret)
	return connect.NewResponse(&authv1.AuthServiceAccessTokenResponse{AccessToken: newJwtToken}), nil
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
		clientAPI:  m,
		hmacSecret: secret,
	}
	path, handler := authv1connect.NewAuthServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
