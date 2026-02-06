//nolint:revive // exported
package mwauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/stoken"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/authutil"
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

// BetterAuthClaims represents the JWT claims from BetterAuth tokens.
// BetterAuth uses standard JWT claims with email and name in the payload.
type BetterAuthClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// betterAuthInterceptor validates BetterAuth JWT tokens.
type betterAuthInterceptor struct {
	jwtSecret []byte
}

// NewBetterAuthInterceptor creates a new interceptor that validates BetterAuth JWT tokens.
// It skips authentication for endpoints marked with @unauthenticated in TypeSpec.
func NewBetterAuthInterceptor(jwtSecret []byte) *betterAuthInterceptor {
	return &betterAuthInterceptor{jwtSecret: jwtSecret}
}

func (i *betterAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		proc := req.Spec().Procedure

		// Skip auth for unauthenticated endpoints
		if authutil.IsUnauthenticated(proc) {
			return next(ctx, req)
		}

		userID, err := i.extractUserID(req.Header().Get(stoken.TokenHeaderKey))
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}

		return next(CreateAuthedContext(ctx, userID), req)
	})
}

func (i *betterAuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return connect.StreamingClientFunc(func(
		ctx context.Context,
		spec connect.Spec,
	) connect.StreamingClientConn {
		// Streaming client doesn't need auth interception
		return next(ctx, spec)
	})
}

func (i *betterAuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(
		ctx context.Context,
		conn connect.StreamingHandlerConn,
	) error {
		proc := conn.Spec().Procedure

		// Skip auth for unauthenticated endpoints
		if authutil.IsUnauthenticated(proc) {
			return next(ctx, conn)
		}

		userID, err := i.extractUserID(conn.RequestHeader().Get(stoken.TokenHeaderKey))
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}

		return next(CreateAuthedContext(ctx, userID), conn)
	})
}

// extractUserID extracts and validates the JWT token from the Authorization header.
func (i *betterAuthInterceptor) extractUserID(headerValue string) (idwrap.IDWrap, error) {
	if headerValue == "" {
		return idwrap.IDWrap{}, errors.New("no authentication token")
	}

	parts := strings.SplitN(headerValue, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return idwrap.IDWrap{}, errors.New("invalid authorization header format")
	}

	tokenString := parts[1]
	claims, err := i.validateBetterAuthJWT(tokenString)
	if err != nil {
		slog.Error("JWT validation failed", "error", err)
		return idwrap.IDWrap{}, errors.New("invalid or expired token")
	}

	// BetterAuth uses the Subject claim for user ID
	userID, err := idwrap.NewText(claims.Subject)
	if err != nil {
		slog.Error("Invalid user ID in token", "subject", claims.Subject, "error", err)
		return idwrap.IDWrap{}, errors.New("invalid user ID in token")
	}

	return userID, nil
}

// validateBetterAuthJWT validates a BetterAuth JWT token.
func (i *betterAuthInterceptor) validateBetterAuthJWT(tokenString string) (*BetterAuthClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &BetterAuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return i.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*BetterAuthClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}
