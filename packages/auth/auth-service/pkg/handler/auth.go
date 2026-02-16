// Package handler implements the Connect RPC handlers for the auth service.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"

	"github.com/the-dev-tools/dev-tools/packages/auth/authlib/jwks"
	authv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth/v1"
	authbackend "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
)

// unauthenticatedEndpoints are procedures that don't require a JWT token.
// SignOut is unauthenticated so users with expired access tokens can still
// revoke their session using a refresh token.
var unauthenticatedEndpoints = map[string]struct{}{
	"/api.auth.v1.AuthService/SignUp":              {},
	"/api.auth.v1.AuthService/SignIn":              {},
	"/api.auth.v1.AuthService/SignOut":             {},
	"/api.auth.v1.AuthService/OAuthUrl":            {},
	"/api.auth.v1.AuthService/HandleOAuthCallback": {},
	"/api.auth.v1.AuthService/RefreshToken":        {},
}

// AuthHandler implements the AuthService Connect RPC handlers.
type AuthHandler struct {
	client     auth_internalv1connect.AuthInternalServiceClient
	jwtKeyfunc jwt.Keyfunc
}

// NewAuthHandler creates an auth handler with the given keyfunc for JWT validation.
func NewAuthHandler(client auth_internalv1connect.AuthInternalServiceClient, kf jwt.Keyfunc) *AuthHandler {
	return &AuthHandler{client: client, jwtKeyfunc: kf}
}

// Context keys for storing auth info.
type ctxKey string

const (
	userIDKey ctxKey = "auth_user_id"
	emailKey  ctxKey = "auth_email"
	nameKey   ctxKey = "auth_name"
)

// extractToken extracts the JWT from request headers.
func extractToken(headers http.Header) string {
	auth := headers.Get("Authorization")
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}
	return ""
}

// AuthInterceptor returns an interceptor that validates JWT and adds user to context.
func (h *AuthHandler) AuthInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			proc := req.Spec().Procedure

			if _, ok := unauthenticatedEndpoints[proc]; ok {
				return next(ctx, req)
			}

			token := extractToken(req.Header())
			if token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no authentication token"))
			}

			// Validate JWT via JWKS — no network call!
			claims, err := jwks.ValidateJWT(token, h.jwtKeyfunc)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
			}

			// Add claims to context
			ctx = context.WithValue(ctx, userIDKey, claims.Subject)
			ctx = context.WithValue(ctx, emailKey, claims.Email)
			ctx = context.WithValue(ctx, nameKey, claims.Name)

			return next(ctx, req)
		}
	}
}

// UserIDFromContext extracts the user ID from context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// EmailFromContext extracts the email from context.
func EmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(emailKey).(string)
	return email, ok
}

// NameFromContext extracts the name from context.
func NameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(nameKey).(string)
	return name, ok
}

// SignUp creates a new user account.
func (h *AuthHandler) SignUp(
	ctx context.Context,
	req *connect.Request[authv1.SignUpRequest],
) (*connect.Response[authv1.SignUpResponse], error) {
	// 1. Create user with password via BetterAuth
	createResp, err := h.client.CreateUserWithPassword(ctx, connect.NewRequest(&authbackend.CreateUserWithPasswordRequest{
		Email:    req.Msg.Email,
		Password: req.Msg.Password,
		Name:     req.Msg.Name,
	}))
	if err != nil {
		return nil, err
	}

	if createResp.Msg.User == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create user"))
	}

	// 2. Get JWT access token using the session token
	tokenResp, err := h.client.Token(ctx, connect.NewRequest(&authbackend.TokenRequest{
		SessionToken: createResp.Msg.SessionToken,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.SignUpResponse{
		UserId:       createResp.Msg.User.Id,
		AccessToken:  tokenResp.Msg.AccessToken,
		RefreshToken: createResp.Msg.SessionToken, // Session token as refresh token
	}), nil
}

// SignIn authenticates a user with email and password.
func (h *AuthHandler) SignIn(
	ctx context.Context,
	req *connect.Request[authv1.SignInRequest],
) (*connect.Response[authv1.SignInResponse], error) {
	// 1. Verify credentials via BetterAuth
	verifyResp, err := h.client.VerifyCredentials(ctx, connect.NewRequest(&authbackend.VerifyCredentialsRequest{
		Email:    req.Msg.Email,
		Password: req.Msg.Password,
	}))
	if err != nil {
		slog.Error("credential verification failed", "error", err)
		// Propagate Connect errors from the backend; don't mask service failures as auth errors
		if connectErr := new(connect.Error); errors.As(err, &connectErr) {
			return nil, connectErr
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("credential verification failed"))
	}

	if !verifyResp.Msg.Valid {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	if verifyResp.Msg.User == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}

	// 2. Get JWT access token using the session token
	sessionToken := ""
	if verifyResp.Msg.SessionToken != nil {
		sessionToken = *verifyResp.Msg.SessionToken
	}

	tokenResp, err := h.client.Token(ctx, connect.NewRequest(&authbackend.TokenRequest{
		SessionToken: sessionToken,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.SignInResponse{
		UserId:       verifyResp.Msg.User.Id,
		AccessToken:  tokenResp.Msg.AccessToken,
		RefreshToken: sessionToken,
	}), nil
}

// SignOut revokes the BetterAuth session if a refresh token is provided.
func (h *AuthHandler) SignOut(
	ctx context.Context,
	req *connect.Request[authv1.SignOutRequest],
) (*connect.Response[authv1.SignOutResponse], error) {
	if req.Msg.RefreshToken != nil && *req.Msg.RefreshToken != "" {
		_, err := h.client.RevokeSession(ctx, connect.NewRequest(&authbackend.RevokeSessionRequest{
			SessionToken: *req.Msg.RefreshToken,
		}))
		if err != nil {
			slog.Warn("failed to revoke session during sign-out", "error", err)
		}
	}

	return connect.NewResponse(&authv1.SignOutResponse{}), nil
}

// OAuthUrl returns the OAuth authorization URL for a provider.
func (h *AuthHandler) OAuthUrl(
	ctx context.Context,
	req *connect.Request[authv1.OAuthUrlRequest],
) (*connect.Response[authv1.OAuthUrlResponse], error) {
	resp, err := h.client.OAuthUrl(ctx, connect.NewRequest(&authbackend.OAuthUrlRequest{
		Provider:    req.Msg.Provider,
		CallbackUrl: req.Msg.CallbackUrl,
		State:       req.Msg.State,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.OAuthUrlResponse{
		Url:   resp.Msg.Url,
		State: resp.Msg.State,
	}), nil
}

// HandleOAuthCallback handles the OAuth callback and creates tokens.
func (h *AuthHandler) HandleOAuthCallback(
	ctx context.Context,
	req *connect.Request[authv1.HandleOAuthCallbackRequest],
) (*connect.Response[authv1.HandleOAuthCallbackResponse], error) {
	// 1. Exchange OAuth code via BetterAuth
	resp, err := h.client.ExchangeOAuthCode(ctx, connect.NewRequest(&authbackend.ExchangeOAuthCodeRequest{
		Provider: req.Msg.Provider,
		Code:     req.Msg.Code,
		State:    req.Msg.State,
	}))
	if err != nil {
		return nil, err
	}

	if resp.Msg.User == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("OAuth callback returned no user"))
	}

	// 2. Get JWT access token using the session token
	tokenResp, err := h.client.Token(ctx, connect.NewRequest(&authbackend.TokenRequest{
		SessionToken: resp.Msg.SessionToken,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.HandleOAuthCallbackResponse{
		UserId:       resp.Msg.User.Id,
		AccessToken:  tokenResp.Msg.AccessToken,
		RefreshToken: resp.Msg.SessionToken, // Session token as refresh token
		IsNewUser:    resp.Msg.IsNewUser,
	}), nil
}

// RefreshToken refreshes the access token using a refresh token (BetterAuth session token).
func (h *AuthHandler) RefreshToken(
	ctx context.Context,
	req *connect.Request[authv1.RefreshTokenRequest],
) (*connect.Response[authv1.RefreshTokenResponse], error) {
	if req.Msg.RefreshToken == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("refresh_token is required"))
	}

	// The refresh token IS the BetterAuth session token.
	// Call Token to get a fresh JWT access token.
	tokenResp, err := h.client.Token(ctx, connect.NewRequest(&authbackend.TokenRequest{
		SessionToken: req.Msg.RefreshToken,
	}))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired refresh token"))
	}

	return connect.NewResponse(&authv1.RefreshTokenResponse{
		AccessToken:  tokenResp.Msg.AccessToken,
		RefreshToken: req.Msg.RefreshToken, // Session token stays the same
	}), nil
}

