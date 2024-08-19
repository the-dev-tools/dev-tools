package mwauth

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/service/sworkspace"
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

// this need NewAuthInterceptor before execute
func NewOrgInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			orgID := req.Header().Get(OrgHeaderKey)
			if orgID == "" {
				// Check token in handlers.
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					errors.New("no organization_id provided"),
				)
			}

			userID, err := GetContextUserID(ctx)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("user id not found"))
			}

			OrgUlid, err := ulid.Parse(orgID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}

			org, err := sworkspace.GetByIDandUserID(OrgUlid, userID)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, connect.NewError(connect.CodeNotFound, errors.New("org not found"))
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if org == nil {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("org not found"))
			}

			CtxWithValue := context.WithValue(ctx, OrgIDKeyCtx, OrgUlid)

			return next(CtxWithValue, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

func GetContextUserID(ctx context.Context) (ulid.ULID, error) {
	ulidID, ok := ctx.Value(UserIDKeyCtx).(ulid.ULID)
	if !ok {
		return ulidID, errors.New("user id not found in context")
	}
	return ulidID, nil
}

func GetContextUserOrgID(ctx context.Context) (ulid.ULID, error) {
	ulidID, ok := ctx.Value(OrgIDKeyCtx).(ulid.ULID)
	if !ok {
		return ulidID, errors.New("org id not found in context")
	}
	return ulidID, nil
}
