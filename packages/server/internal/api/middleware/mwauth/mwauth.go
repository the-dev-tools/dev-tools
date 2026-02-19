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
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/the-dev-tools/dev-tools/packages/auth-lib/jwks"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/stoken"
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


// betterAuthInterceptor validates BetterAuth JWT tokens using JWKS
// and auto-provisions users in the main database.
type betterAuthInterceptor struct {
	jwtKeyfunc  jwt.Keyfunc
	userService suser.UserService
}

// NewBetterAuthInterceptor creates a new interceptor that validates BetterAuth JWT tokens
// using the given keyfunc and auto-provisions users in the main database.
func NewBetterAuthInterceptor(kf jwt.Keyfunc, us suser.UserService) *betterAuthInterceptor {
	return &betterAuthInterceptor{
		jwtKeyfunc:  kf,
		userService: us,
	}
}

func (i *betterAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(
		ctx context.Context,
		req connect.AnyRequest,
	) (connect.AnyResponse, error) {
		userID, err := i.extractUserID(ctx, req.Header().Get(stoken.TokenHeaderKey))
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
		userID, err := i.extractUserID(ctx, conn.RequestHeader().Get(stoken.TokenHeaderKey))
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}

		return next(CreateAuthedContext(ctx, userID), conn)
	})
}

// extractUserID validates the JWT, looks up (or auto-creates) the user in the main DB,
// and returns the internal ULID.
func (i *betterAuthInterceptor) extractUserID(ctx context.Context, headerValue string) (idwrap.IDWrap, error) {
	if headerValue == "" {
		return idwrap.IDWrap{}, errors.New("no authentication token")
	}

	parts := strings.SplitN(headerValue, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return idwrap.IDWrap{}, errors.New("invalid authorization header format")
	}

	tokenString := parts[1]
	claims, err := jwks.ValidateJWT(tokenString, i.jwtKeyfunc)
	if err != nil {
		slog.Error("JWT validation failed", "error", err)
		return idwrap.IDWrap{}, errors.New("invalid or expired token")
	}

	// BetterAuth uses the Subject claim for user ID (external_id in our DB)
	externalID := claims.Subject
	if externalID == "" {
		return idwrap.IDWrap{}, errors.New("missing subject in token")
	}

	// Look up user by external_id → returns internal ULID
	user, err := i.userService.GetUserByExternalID(ctx, externalID)
	if err == nil {
		return user.ID, nil
	}

	if !errors.Is(err, suser.ErrUserNotFound) {
		slog.Error("Failed to look up user by external_id", "external_id", externalID, "error", err)
		return idwrap.IDWrap{}, errors.New("internal error looking up user")
	}

	// Auto-provision: create user with new ULID
	newID := idwrap.NewNow()
	newUser := &muser.User{
		ID:         newID,
		Email:      claims.Email,
		Name:       claims.Name,
		ExternalID: &externalID,
	}

	if err := i.userService.CreateUser(ctx, newUser); err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			// Race condition: concurrent provisioning created the same user
			user, retryErr := i.userService.GetUserByExternalID(ctx, externalID)
			if retryErr != nil {
				slog.Error("Failed to look up user after constraint violation", "external_id", externalID, "error", retryErr)
				return idwrap.IDWrap{}, errors.New("failed to provision user")
			}
			return user.ID, nil
		}
		slog.Error("Failed to create user", "external_id", externalID, "error", err)
		return idwrap.IDWrap{}, errors.New("failed to provision user")
	}

	slog.Info("Auto-provisioned user from BetterAuth", "internal_id", newID.String(), "external_id", externalID, "email", claims.Email)
	return newID, nil
}

