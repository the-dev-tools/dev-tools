//nolint:revive // exported
package mwauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/stoken"

	"connectrpc.com/connect"
)

type ContextKey int

const (
	UserIDKeyCtx ContextKey = iota
	WorkspaceIDKeyCtx
)

const LocalDummyIDStr = "00000000000000000000000000"

var LocalDummyID = idwrap.NewTextMust(LocalDummyIDStr)

type authInterceptor struct{}

func NewAuthInterceptor() *authInterceptor {
	return &authInterceptor{}
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	// Same as previous UnaryInterceptorFunc.
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		return next(CreateAuthedContext(ctx, LocalDummyID), req)
	})
}

func (*authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return connect.StreamingClientFunc(func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		conn := next(CreateAuthedContext(ctx, LocalDummyID), spec)
		return conn
	})
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		return next(CreateAuthedContext(ctx, LocalDummyID), conn)
	})
}

func NewAuthInterceptorOne(secret []byte) connect.UnaryInterceptorFunc {
	data := AuthInterceptorData{secret: secret}
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			return data.AuthInterceptor(ctx, req, next)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func NewAuthInterceptorLocal() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			return AuthInterceptorLocal(ctx, req, next)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

type AuthInterceptorData struct {
	secret []byte
}

func CreateAuthedContext(ctx context.Context, userID idwrap.IDWrap) context.Context {
	return context.WithValue(ctx, UserIDKeyCtx, userID)
}

func (authData AuthInterceptorData) AuthInterceptor(ctx context.Context, req connect.AnyRequest, next connect.UnaryFunc) (connect.AnyResponse, error) {
	headerValue := req.Header().Get(stoken.TokenHeaderKey)
	if headerValue == "" {
		// Check token in handlers.
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("no token provided"),
		)
	}

	tokenRaw := strings.Split(headerValue, "Bearer ")
	if len(tokenRaw) != 2 {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, errors.New("invalid token"))
	}

	claims, err := stoken.ValidateJWT(tokenRaw[1], stoken.AccessToken, authData.secret)
	if err != nil {
		slog.ErrorContext(ctx, "Error validating JWT token", "error", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ID, err := idwrap.NewText(claims.Subject)
	if err != nil {
		slog.ErrorContext(ctx, "Error creating ID from claims.Subject", "error", err)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return next(CreateAuthedContext(ctx, ID), req)
}

func AuthInterceptorLocal(ctx context.Context, req connect.AnyRequest, next connect.UnaryFunc) (connect.AnyResponse, error) {
	return next(CreateAuthedContext(ctx, LocalDummyID), req)
}

func AuthInterceptorLocalStreamHandlerLocal(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		return next(CreateAuthedContext(ctx, LocalDummyID), conn)
	})
}

func CrashInterceptor(ctx context.Context, req connect.AnyRequest, next connect.UnaryFunc) (resp connect.AnyResponse, err error) {
	if req.Spec().IsClient {
		return next(ctx, req)
	}

	defer func() {
		// recover from panic if one occurred and return an error
		if r := recover(); r != nil {
			err = connect.NewError(connect.CodeInternal, fmt.Errorf("panic: %v", r))
			resp = nil
		}
	}()
	return next(ctx, req)
}

func GetContextUserID(ctx context.Context) (idwrap.IDWrap, error) {
	ulidID, ok := ctx.Value(UserIDKeyCtx).(idwrap.IDWrap)
	if !ok {
		return ulidID, errors.New("user id not found in context")
	}
	return ulidID, nil
}
