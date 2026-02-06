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
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth/v1"
	authbackend "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/authutil"
)

// JWTClaims represents the claims in our JWT tokens.
type JWTClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// AuthHandler implements the AuthService Connect RPC handlers.
type AuthHandler struct {
	client    auth_internalv1connect.AuthInternalServiceClient
	jwtSecret []byte
}

// NewAuthHandler creates a new auth handler.
// jwtSecret must not be empty.
func NewAuthHandler(client auth_internalv1connect.AuthInternalServiceClient, jwtSecret []byte) (*AuthHandler, error) {
	if len(jwtSecret) == 0 {
		return nil, errors.New("jwt secret is required")
	}
	return &AuthHandler{client: client, jwtSecret: jwtSecret}, nil
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

// validateJWT validates a JWT token and returns the claims.
func (h *AuthHandler) validateJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// getClientInfo extracts IP address and user agent from request headers.
func getClientInfo(headers http.Header) (ipAddress, userAgent string) {
	ipAddress = headers.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = headers.Get("X-Real-IP")
	}
	userAgent = headers.Get("User-Agent")
	return
}

// AuthInterceptor returns an interceptor that validates JWT and adds user to context.
func (h *AuthHandler) AuthInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			proc := req.Spec().Procedure

			// Skip auth for unauthenticated endpoints (defined in TypeSpec with @unauthenticated)
			if authutil.IsUnauthenticated(proc) {
				return next(ctx, req)
			}

			token := extractToken(req.Header())
			if token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no authentication token"))
			}

			// Validate JWT locally - no network call!
			claims, err := h.validateJWT(token)
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

// convertInternalUser converts internal user to public user.
func convertInternalUser(u *authbackend.User) *authv1.User {
	if u == nil {
		return nil
	}
	return &authv1.User{
		Id:            u.Id,
		Email:         u.Email,
		Name:          u.Name,
		Image:         u.Image,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

// convertInternalProvider converts public provider enum to internal provider enum.
func convertInternalProvider(p authv1.AuthProvider) authbackend.AuthProvider {
	return authbackend.AuthProvider(p)
}

// SignUp creates a new user account.
func (h *AuthHandler) SignUp(
	ctx context.Context,
	req *connect.Request[authv1.SignUpRequest],
) (*connect.Response[authv1.SignUpResponse], error) {
	ipAddress, userAgent := getClientInfo(req.Header())

	// Create user with password using the dedicated endpoint
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

	// Create tokens (JWT + refresh)
	tokensResp, err := h.client.CreateTokens(ctx, connect.NewRequest(&authbackend.CreateTokensRequest{
		UserId:    createResp.Msg.User.Id,
		Email:     createResp.Msg.User.Email,
		Name:      createResp.Msg.User.Name,
		IpAddress: &ipAddress,
		UserAgent: &userAgent,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.SignUpResponse{
		User:         convertInternalUser(createResp.Msg.User),
		AccessToken:  tokensResp.Msg.AccessToken,
		RefreshToken: tokensResp.Msg.RefreshToken,
	}), nil
}

// SignIn authenticates a user with email and password.
func (h *AuthHandler) SignIn(
	ctx context.Context,
	req *connect.Request[authv1.SignInRequest],
) (*connect.Response[authv1.SignInResponse], error) {
	ipAddress, userAgent := getClientInfo(req.Header())

	// Verify credentials
	verifyResp, err := h.client.VerifyCredentials(ctx, connect.NewRequest(&authbackend.VerifyCredentialsRequest{
		Email:    req.Msg.Email,
		Password: req.Msg.Password,
	}))
	if err != nil {
		// Log actual error for debugging, return generic message to client
		slog.Error("credential verification failed", "error", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	if !verifyResp.Msg.Valid {
		// Always return generic message to client
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	if verifyResp.Msg.User == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("user not found"))
	}

	// Create tokens (JWT + refresh)
	tokensResp, err := h.client.CreateTokens(ctx, connect.NewRequest(&authbackend.CreateTokensRequest{
		UserId:    verifyResp.Msg.User.Id,
		Email:     verifyResp.Msg.User.Email,
		Name:      verifyResp.Msg.User.Name,
		IpAddress: &ipAddress,
		UserAgent: &userAgent,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.SignInResponse{
		User:         convertInternalUser(verifyResp.Msg.User),
		AccessToken:  tokensResp.Msg.AccessToken,
		RefreshToken: tokensResp.Msg.RefreshToken,
	}), nil
}

// SignOut revokes the refresh token.
func (h *AuthHandler) SignOut(
	ctx context.Context,
	req *connect.Request[authv1.SignOutRequest],
) (*connect.Response[authv1.SignOutResponse], error) {
	if req.Msg.RefreshToken == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("refresh_token is required"))
	}

	// Revoke refresh token
	_, err := h.client.RevokeRefreshToken(ctx, connect.NewRequest(&authbackend.RevokeRefreshTokenRequest{
		RefreshToken: req.Msg.RefreshToken,
	}))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&authv1.SignOutResponse{
		Success: true,
	}), nil
}

// GetOAuthUrl returns the OAuth authorization URL for a provider.
func (h *AuthHandler) GetOAuthUrl(
	ctx context.Context,
	req *connect.Request[authv1.GetOAuthUrlRequest],
) (*connect.Response[authv1.GetOAuthUrlResponse], error) {
	resp, err := h.client.GetOAuthUrl(ctx, connect.NewRequest(&authbackend.GetOAuthUrlRequest{
		Provider:    convertInternalProvider(req.Msg.Provider),
		CallbackUrl: req.Msg.CallbackUrl,
		State:       req.Msg.State,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.GetOAuthUrlResponse{
		Url:   resp.Msg.Url,
		State: resp.Msg.State,
	}), nil
}

// HandleOAuthCallback handles the OAuth callback and creates tokens.
func (h *AuthHandler) HandleOAuthCallback(
	ctx context.Context,
	req *connect.Request[authv1.HandleOAuthCallbackRequest],
) (*connect.Response[authv1.HandleOAuthCallbackResponse], error) {
	ipAddress, userAgent := getClientInfo(req.Header())

	resp, err := h.client.ExchangeOAuthCode(ctx, connect.NewRequest(&authbackend.ExchangeOAuthCodeRequest{
		Provider:  convertInternalProvider(req.Msg.Provider),
		Code:      req.Msg.Code,
		State:     req.Msg.State,
		IpAddress: &ipAddress,
		UserAgent: &userAgent,
	}))
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&authv1.HandleOAuthCallbackResponse{
		User:         convertInternalUser(resp.Msg.User),
		AccessToken:  resp.Msg.AccessToken,
		RefreshToken: resp.Msg.RefreshToken,
		IsNewUser:    resp.Msg.IsNewUser,
	}), nil
}

// RefreshToken refreshes the access token using a refresh token.
func (h *AuthHandler) RefreshToken(
	ctx context.Context,
	req *connect.Request[authv1.RefreshTokenRequest],
) (*connect.Response[authv1.RefreshTokenResponse], error) {
	if req.Msg.RefreshToken == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("refresh_token is required"))
	}

	resp, err := h.client.RefreshTokens(ctx, connect.NewRequest(&authbackend.RefreshTokensRequest{
		RefreshToken: req.Msg.RefreshToken,
	}))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired refresh token"))
	}

	return connect.NewResponse(&authv1.RefreshTokenResponse{
		AccessToken:  resp.Msg.AccessToken,
		RefreshToken: resp.Msg.RefreshToken,
	}), nil
}

// GetMe returns the current authenticated user from the JWT claims.
func (h *AuthHandler) GetMe(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[authv1.GetMeResponse], error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	// Get full user info from BetterAuth
	resp, err := h.client.GetUser(ctx, connect.NewRequest(&authbackend.GetUserRequest{
		UserId: userID,
	}))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if resp.Msg.User == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}

	return connect.NewResponse(&authv1.GetMeResponse{
		User: convertInternalUser(resp.Msg.User),
	}), nil
}

// GetLinkedAccounts returns the linked accounts for the authenticated user.
func (h *AuthHandler) GetLinkedAccounts(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[authv1.GetLinkedAccountsResponse], error) {
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}

	resp, err := h.client.GetAccountsByUserId(ctx, connect.NewRequest(&authbackend.GetAccountsByUserIdRequest{
		UserId: userID,
	}))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accounts := make([]*authv1.LinkedAccount, 0, len(resp.Msg.Accounts))
	for _, acc := range resp.Msg.Accounts {
		accounts = append(accounts, &authv1.LinkedAccount{
			Provider:          authv1.AuthProvider(acc.Provider),
			ProviderAccountId: acc.ProviderAccountId,
			LinkedAt:          acc.CreatedAt,
		})
	}

	return connect.NewResponse(&authv1.GetLinkedAccountsResponse{
		Accounts:    accounts,
		HasPassword: resp.Msg.HasPassword,
	}), nil
}
