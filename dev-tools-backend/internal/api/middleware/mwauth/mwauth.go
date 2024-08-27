package mwauth

import (
	"context"
	"dev-tools-backend/pkg/stoken"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type ContextKey string

const UserIDKeyCtx ContextKey = "UserID"

const (
	OrgHeaderKey            = "organization_id"
	OrgIDKeyCtx  ContextKey = "OrgID"
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

	token, err := stoken.ValidateJWT(tokenRaw[1], stoken.AccessToken, authData.secret)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	claims, err := stoken.GetClaims(token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	ulidID, err := ulid.Parse(claims.Subject)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	CtxWithValue := context.WithValue(ctx, UserIDKeyCtx, ulidID)

	return next(CtxWithValue, req)
}

func CrashInterceptor(ctx context.Context, req connect.AnyRequest, next connect.UnaryFunc) (resp connect.AnyResponse, err error) {
	if req.Spec().IsClient {
		return next(ctx, req)
	}

	defer func() {
		// recover from panic if one occurred and return an error
		if r := recover(); r != nil {
			err := errors.New("Internal Server Error")
			err = connect.NewError(connect.CodeInternal, err)
			resp = nil
		}
	}()
	return next(ctx, req)
}

func GetContextUserID(ctx context.Context) (ulid.ULID, error) {
	ulidID, ok := ctx.Value(UserIDKeyCtx).(ulid.ULID)
	if !ok {
		return ulidID, errors.New("user id not found in context")
	}
	return ulidID, nil
}
