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

	jwtToken, err := stoken.NewJWT(publicAddress, a.hmacSecret)
	if err != nil {
		return nil, err
	}

	respRaw := &authv1.AuthServiceDIDResponse{
		Token: jwtToken,
	}

	resp := connect.NewResponse(respRaw)
	return resp, nil
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
