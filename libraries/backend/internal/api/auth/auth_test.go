package auth_test

import (
	"context"
	"testing"
	"the-dev-tools/backend/internal/api/auth"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/stoken"
	authv1 "the-dev-tools/spec/dist/buf/go/auth/v1"
	"time"

	"connectrpc.com/connect"
	"github.com/magiclabs/magic-admin-go/client"
)

func TestRefreshToken(t *testing.T) {
	someSecret := []byte("secret")

	sv := auth.AuthServer{
		ClientAPI:  client.API{},
		HmacSecret: someSecret,
	}

	ctx := context.Background()

	ulidID := idwrap.NewNow()
	email := "some-email"

	tk, err := stoken.NewJWT(ulidID, email, stoken.RefreshToken, time.Minute, someSecret)
	if err != nil {
		t.Fatal(err)
	}

	req := &connect.Request[authv1.AuthRefreshRequest]{
		Msg: &authv1.AuthRefreshRequest{
			RefreshToken: tk,
		},
	}

	resp, err := sv.AuthRefresh(ctx, req)
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

	if resp.Msg.GetAccessToken() == "" {
		t.Fatal("access token is empty")
	}
}
