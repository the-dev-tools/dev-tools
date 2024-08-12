package mwauth

import (
	"context"
	"devtools-backend/pkg/stoken"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
)

type ContextKey string

const UserIDKeyCtx ContextKey = "UserID"

func NewAuthInterceptor(secret []byte) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
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

			token, err := stoken.ValidateJWT(tokenRaw[1], stoken.AccessToken, secret)
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
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func GetContextUserID(ctx context.Context) (*ulid.ULID, error) {
	ulidID, ok := ctx.Value(UserIDKeyCtx).(ulid.ULID)
	if !ok {
		return nil, errors.New("user id not found in context")
	}
	return &ulidID, nil
}
