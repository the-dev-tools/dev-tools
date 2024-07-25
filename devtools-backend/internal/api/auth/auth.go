package auth

import (
	"context"
	"devtools-backend/internal/api"
	authv1 "devtools-services/gen/auth/v1"
	"devtools-services/gen/auth/v1/authv1connect"
	"errors"
	"os"
	"time"

	"connectrpc.com/connect"

	"github.com/golang-jwt/jwt/v5"
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

	now := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"publicAddress": publicAddress,
		"iat":           now.Unix(),
		"nbf":           now.Add(24 * time.Hour).Unix(),
		"exp":           now.Add(48 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(a.hmacSecret)
	if err != nil {
		panic(err)
	}

	respRaw := &authv1.AuthServiceDIDResponse{
		Token: tokenString,
	}

	resp := connect.NewResponse(respRaw)
	return resp, nil
}

func CreateService() (*api.Service, error) {
	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		return nil, errors.New("HMAC_SECRET env var is required")
	}
	hmacSecretBytes := []byte(hmacSecret)

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
		hmacSecret: hmacSecretBytes,
	}
	path, handler := authv1connect.NewAuthServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
