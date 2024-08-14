package auth_test

import (
	"context"
	"dev-tools-backend/internal/api/auth"
	"dev-tools-backend/pkg/stoken"
	authv1 "devtools-services/gen/auth/v1"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

func TestRefreshToken(t *testing.T) {
	someSecret := []byte("secret")

	sv := auth.AuthServer{
		ClientAPI:  nil,
		HmacSecret: someSecret,
	}

	ctx := context.Background()

	ulidID := ulid.Make()
	email := "some-email"

	tk, err := stoken.NewJWT(ulidID, email, stoken.RefreshToken, time.Minute, someSecret)
	if err != nil {
		t.Fatal(err)
	}

	req := &connect.Request[authv1.AuthServiceRefreshTokenRequest]{
		Msg: &authv1.AuthServiceRefreshTokenRequest{
			RefreshToken: tk,
		},
	}

	resp, err := sv.RefreshToken(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	if resp == nil {
		t.Fatal("response is nil")
		return // need this cuz golint cannot understand that t.Fatal will stop the function
	}

	if resp.Msg == nil {
		t.Fatal("response message is nil")
		return // need this cuz golint cannot understand that t.Fatal will stop the function
	}

	if resp.Msg.AccessToken == "" {
		t.Fatal("access token is empty")
	}
}
