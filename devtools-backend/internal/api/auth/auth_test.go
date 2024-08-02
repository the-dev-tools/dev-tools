package auth_test

import (
	"context"
	"devtools-backend/internal/api/auth"
	"devtools-backend/pkg/stoken"
	authv1 "devtools-services/gen/auth/v1"
	"testing"
	"time"

	"connectrpc.com/connect"
)

func TestRefreshToken(t *testing.T) {
	someSecret := []byte("secret")

	sv := auth.AuthServer{
		ClientAPI:  nil,
		HmacSecret: someSecret,
	}

	ctx := context.Background()

	id := "some-id"
	email := "some-email"

	tk, err := stoken.NewJWT(id, email, stoken.RefreshToken, time.Minute, someSecret)
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
	}

	if resp.Msg == nil {
		t.Fatal("response message is nil")
	}

	if resp.Msg.AccessToken == "" {
		t.Fatal("access token is empty")
	}
}
