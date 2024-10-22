package mwauth

import (
	"context"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/stoken"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

type ContextKey int

const (
	UserIDKeyCtx ContextKey = iota
	WorkspaceIDKeyCtx
)

func NewAuthInterceptor(secret []byte) connect.UnaryInterceptorFunc {
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

func NewCrashInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			return CrashInterceptor(ctx, req, next)
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
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ID, err := idwrap.NewWithParse(claims.Subject)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return next(CreateAuthedContext(ctx, ID), req)
}

func CrashInterceptor(ctx context.Context, req connect.AnyRequest, next connect.UnaryFunc) (resp connect.AnyResponse, err error) {
	if req.Spec().IsClient {
		return next(ctx, req)
	}

	defer func() {
		// recover from panic if one occurred and return an error
		if r := recover(); r != nil {
			err = connect.NewError(connect.CodeInternal, err)
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
