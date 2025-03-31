package mwauth_test

import (
	"context"
	"testing"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/stoken"
	"time"

	"connectrpc.com/connect"
)

type TestRequest struct{}

type testResponse struct {
	Msg int
}

func TestNewAuthInterceptor(t *testing.T) {
	secret := []byte("secret")
	middlewareFunc := mwauth.NewAuthInterceptorOne(secret)
	if middlewareFunc == nil {
		t.Error("Expected middleware function, got nil")
	}

	testResp := testResponse{
		Msg: 10,
	}

	nextFunc := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp := &connect.Response[testResponse]{
			Msg: &testResp,
		}
		return resp, nil
	}

	nestedAuthFunc := middlewareFunc(nextFunc)
	if nestedAuthFunc == nil {
		t.Error("Expected nested middleware function, got nil")
	}

	t.Run("incorrect jwt Header", func(t *testing.T) {
		testReq := &connect.Request[testResponse]{}
		testReq.Header().Set(stoken.TokenHeaderKey, "token")

		resp, err := nestedAuthFunc(context.Background(), testReq)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if resp != nil {
			t.Error("Expected nil response, got", resp)
		}
	})

	t.Run("none jwt Header", func(t *testing.T) {
		testReq := &connect.Request[testResponse]{}
		testReq.Header().Set(stoken.TokenHeaderKey, "")

		resp, err := nestedAuthFunc(context.Background(), testReq)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if resp != nil {
			t.Error("Expected nil response, got", resp)
		}
	})

	id := idwrap.NewNow()
	email := "ege@dev.tools"

	token, err := stoken.NewJWT(id, email, stoken.AccessToken, time.Minute, secret)
	if err != nil {
		t.Fatal(err)
	}

	bareToken := "Bearer " + token

	t.Run("correct jwt Header", func(t *testing.T) {
		testReq := &connect.Request[testResponse]{}
		testReq.Header().Set(stoken.TokenHeaderKey, bareToken)

		resp, err := nestedAuthFunc(context.Background(), testReq)
		if err != nil {
			t.Error("Expected nil, got", err)
		}

		if resp == nil {
			t.Error("Expected something, got", resp)
		}

		respCasted := resp.(*connect.Response[testResponse]).Msg
		if respCasted == nil {
			t.Error("Expected something, got", respCasted)
		}

		if respCasted.Msg != testResp.Msg {
			t.Error("Expecte same response, got", respCasted)
		}
	})
}
