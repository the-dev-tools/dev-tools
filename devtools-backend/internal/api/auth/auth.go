package auth

import (
	"context"
	authv1 "devtools-services/gen/auth/v1"
	"devtools-services/gen/auth/v1/authv1connect"
	"errors"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

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

func ListenAndServe(port string) error {
	hmacSecret := os.Getenv("HMAC_SECRET")
	if hmacSecret == "" {
		return errors.New("HMAC_SECRET env var is required")
	}
	hmacSecretBytes := []byte(hmacSecret)

	magicLinkSecret := os.Getenv("MAGIC_LINK_SECRET")
	if magicLinkSecret == "" {
		return errors.New("MAGIC_LINK_SECRET env var is required")
	}

	cl := magic.NewClientWithRetry(5, time.Second, 10*time.Second)
	m, err := client.New(magicLinkSecret, cl)
	if err != nil {
		return err
	}

	server := &AuthServer{
		clientAPI:  m,
		hmacSecret: hmacSecretBytes,
	}
	mux := http.NewServeMux()
	path, handler := authv1connect.NewAuthServiceHandler(server)
	mux.Handle(path, handler)

	http.ListenAndServe(
		":"+port,
		// INFO: Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{
			IdleTimeout:          0,
			MaxConcurrentStreams: 100000,
			MaxHandlers:          0,
		}),
	)

	return nil
}
