package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	authv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth/v1"
	authinternalv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/authutil"
	"google.golang.org/protobuf/types/known/emptypb"
)

// mockAuthClient implements auth_internalv1connect.AuthInternalServiceClient for testing.
type mockAuthClient struct {
	auth_internalv1connect.UnimplementedAuthInternalServiceHandler

	// Response stubs
	createUserWithPasswordResp *authinternalv1.CreateUserWithPasswordResponse
	createUserWithPasswordErr  error

	verifyCredentialsResp *authinternalv1.VerifyCredentialsResponse
	verifyCredentialsErr  error

	createTokensResp *authinternalv1.CreateTokensResponse
	createTokensErr  error

	refreshTokensResp *authinternalv1.RefreshTokensResponse
	refreshTokensErr  error

	revokeRefreshTokenResp *authinternalv1.RevokeRefreshTokenResponse
	revokeRefreshTokenErr  error

	getUserResp *authinternalv1.GetUserResponse
	getUserErr  error

	getOAuthUrlResp *authinternalv1.GetOAuthUrlResponse
	getOAuthUrlErr  error

	exchangeOAuthCodeResp *authinternalv1.ExchangeOAuthCodeResponse
	exchangeOAuthCodeErr  error

	getAccountsByUserIdResp *authinternalv1.GetAccountsByUserIdResponse
	getAccountsByUserIdErr  error

	// Capture request data for assertions
	lastCreateUserWithPasswordReq *authinternalv1.CreateUserWithPasswordRequest
	lastVerifyCredentialsReq      *authinternalv1.VerifyCredentialsRequest
	lastCreateTokensReq           *authinternalv1.CreateTokensRequest
	lastRefreshTokensReq          *authinternalv1.RefreshTokensRequest
	lastRevokeRefreshTokenReq     *authinternalv1.RevokeRefreshTokenRequest
	lastGetUserReq                *authinternalv1.GetUserRequest
	lastGetOAuthUrlReq            *authinternalv1.GetOAuthUrlRequest
	lastExchangeOAuthCodeReq      *authinternalv1.ExchangeOAuthCodeRequest
	lastGetAccountsByUserIdReq    *authinternalv1.GetAccountsByUserIdRequest
}

func (m *mockAuthClient) CreateUser(_ context.Context, _ *connect.Request[authinternalv1.CreateUserRequest]) (*connect.Response[authinternalv1.CreateUserResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (m *mockAuthClient) CreateUserWithPassword(_ context.Context, req *connect.Request[authinternalv1.CreateUserWithPasswordRequest]) (*connect.Response[authinternalv1.CreateUserWithPasswordResponse], error) {
	m.lastCreateUserWithPasswordReq = req.Msg
	if m.createUserWithPasswordErr != nil {
		return nil, m.createUserWithPasswordErr
	}
	return connect.NewResponse(m.createUserWithPasswordResp), nil
}

func (m *mockAuthClient) VerifyCredentials(_ context.Context, req *connect.Request[authinternalv1.VerifyCredentialsRequest]) (*connect.Response[authinternalv1.VerifyCredentialsResponse], error) {
	m.lastVerifyCredentialsReq = req.Msg
	if m.verifyCredentialsErr != nil {
		return nil, m.verifyCredentialsErr
	}
	return connect.NewResponse(m.verifyCredentialsResp), nil
}

func (m *mockAuthClient) GetUser(_ context.Context, req *connect.Request[authinternalv1.GetUserRequest]) (*connect.Response[authinternalv1.GetUserResponse], error) {
	m.lastGetUserReq = req.Msg
	if m.getUserErr != nil {
		return nil, m.getUserErr
	}
	return connect.NewResponse(m.getUserResp), nil
}

func (m *mockAuthClient) CreateTokens(_ context.Context, req *connect.Request[authinternalv1.CreateTokensRequest]) (*connect.Response[authinternalv1.CreateTokensResponse], error) {
	m.lastCreateTokensReq = req.Msg
	if m.createTokensErr != nil {
		return nil, m.createTokensErr
	}
	return connect.NewResponse(m.createTokensResp), nil
}

func (m *mockAuthClient) RefreshTokens(_ context.Context, req *connect.Request[authinternalv1.RefreshTokensRequest]) (*connect.Response[authinternalv1.RefreshTokensResponse], error) {
	m.lastRefreshTokensReq = req.Msg
	if m.refreshTokensErr != nil {
		return nil, m.refreshTokensErr
	}
	return connect.NewResponse(m.refreshTokensResp), nil
}

func (m *mockAuthClient) RevokeRefreshToken(_ context.Context, req *connect.Request[authinternalv1.RevokeRefreshTokenRequest]) (*connect.Response[authinternalv1.RevokeRefreshTokenResponse], error) {
	m.lastRevokeRefreshTokenReq = req.Msg
	if m.revokeRefreshTokenErr != nil {
		return nil, m.revokeRefreshTokenErr
	}
	return connect.NewResponse(m.revokeRefreshTokenResp), nil
}

func (m *mockAuthClient) GetOAuthUrl(_ context.Context, req *connect.Request[authinternalv1.GetOAuthUrlRequest]) (*connect.Response[authinternalv1.GetOAuthUrlResponse], error) {
	m.lastGetOAuthUrlReq = req.Msg
	if m.getOAuthUrlErr != nil {
		return nil, m.getOAuthUrlErr
	}
	return connect.NewResponse(m.getOAuthUrlResp), nil
}

func (m *mockAuthClient) ExchangeOAuthCode(_ context.Context, req *connect.Request[authinternalv1.ExchangeOAuthCodeRequest]) (*connect.Response[authinternalv1.ExchangeOAuthCodeResponse], error) {
	m.lastExchangeOAuthCodeReq = req.Msg
	if m.exchangeOAuthCodeErr != nil {
		return nil, m.exchangeOAuthCodeErr
	}
	return connect.NewResponse(m.exchangeOAuthCodeResp), nil
}

func (m *mockAuthClient) GetAccountsByUserId(_ context.Context, req *connect.Request[authinternalv1.GetAccountsByUserIdRequest]) (*connect.Response[authinternalv1.GetAccountsByUserIdResponse], error) {
	m.lastGetAccountsByUserIdReq = req.Msg
	if m.getAccountsByUserIdErr != nil {
		return nil, m.getAccountsByUserIdErr
	}
	return connect.NewResponse(m.getAccountsByUserIdResp), nil
}

func (m *mockAuthClient) CreateAccount(_ context.Context, _ *connect.Request[authinternalv1.CreateAccountRequest]) (*connect.Response[authinternalv1.CreateAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

// newTestHandler creates an AuthHandler with a mock client for testing.
func newTestHandler(mock *mockAuthClient) *AuthHandler {
	return &AuthHandler{
		client:    mock,
		jwtSecret: []byte("test-secret"),
	}
}

func TestIsUnauthenticated(t *testing.T) {
	tests := []struct {
		name      string
		procedure string
		want      bool
	}{
		// Unauthenticated endpoints (should return true)
		{
			name:      "SignUp is unauthenticated",
			procedure: "/api.auth.v1.AuthService/SignUp",
			want:      true,
		},
		{
			name:      "SignIn is unauthenticated",
			procedure: "/api.auth.v1.AuthService/SignIn",
			want:      true,
		},
		{
			name:      "GetOAuthUrl is unauthenticated",
			procedure: "/api.auth.v1.AuthService/GetOAuthUrl",
			want:      true,
		},
		{
			name:      "HandleOAuthCallback is unauthenticated",
			procedure: "/api.auth.v1.AuthService/HandleOAuthCallback",
			want:      true,
		},
		{
			name:      "RefreshToken is unauthenticated",
			procedure: "/api.auth.v1.AuthService/RefreshToken",
			want:      true,
		},
		// Authenticated endpoints (should return false)
		{
			name:      "GetMe requires authentication",
			procedure: "/api.auth.v1.AuthService/GetMe",
			want:      false,
		},
		{
			name:      "SignOut requires authentication",
			procedure: "/api.auth.v1.AuthService/SignOut",
			want:      false,
		},
		// Edge cases
		{
			name:      "empty procedure",
			procedure: "",
			want:      false,
		},
		{
			name:      "partial match should not work",
			procedure: "/api.auth.v1.AuthService/SignUpExtra",
			want:      false,
		},
		{
			name:      "different service with same method name",
			procedure: "/api.other.v1.OtherService/SignUp",
			want:      true, // Still matches because we check suffix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := authutil.IsUnauthenticated(tt.procedure)
			if got != tt.want {
				t.Errorf("IsUnauthenticated(%q) = %v, want %v", tt.procedure, got, tt.want)
			}
		})
	}
}

func TestUnauthenticatedEndpoints(t *testing.T) {
	// Verify the expected endpoints are in the map
	expected := []string{"SignUp", "SignIn", "GetOAuthUrl", "HandleOAuthCallback", "RefreshToken"}

	for _, endpoint := range expected {
		if _, ok := authutil.UnauthenticatedEndpoints[endpoint]; !ok {
			t.Errorf("expected %q to be in UnauthenticatedEndpoints", endpoint)
		}
	}

	// Verify GetMe is NOT in the map (requires auth)
	if _, ok := authutil.UnauthenticatedEndpoints["GetMe"]; ok {
		t.Error("GetMe should NOT be in UnauthenticatedEndpoints")
	}

	// Verify SignOut is NOT in the map (requires auth)
	if _, ok := authutil.UnauthenticatedEndpoints["SignOut"]; ok {
		t.Error("SignOut should NOT be in UnauthenticatedEndpoints")
	}
}

func TestNewAuthHandler_RequiresSecret(t *testing.T) {
	// Empty secret should fail
	_, err := NewAuthHandler(nil, []byte{})
	if err == nil {
		t.Error("expected error for empty secret")
	}

	// Nil secret should fail
	_, err = NewAuthHandler(nil, nil)
	if err == nil {
		t.Error("expected error for nil secret")
	}

	// Valid secret should succeed
	handler, err := NewAuthHandler(nil, []byte("valid-secret"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if handler == nil {
		t.Error("expected handler to be created")
	}
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		want   string
	}{
		{
			name:   "valid bearer token",
			header: http.Header{"Authorization": []string{"Bearer abc123"}},
			want:   "abc123",
		},
		{
			name:   "bearer case insensitive",
			header: http.Header{"Authorization": []string{"bearer abc123"}},
			want:   "abc123",
		},
		{
			name:   "BEARER uppercase",
			header: http.Header{"Authorization": []string{"BEARER abc123"}},
			want:   "abc123",
		},
		{
			name:   "no authorization header",
			header: http.Header{},
			want:   "",
		},
		{
			name:   "empty authorization header",
			header: http.Header{"Authorization": []string{""}},
			want:   "",
		},
		{
			name:   "basic auth (not bearer)",
			header: http.Header{"Authorization": []string{"Basic abc123"}},
			want:   "",
		},
		{
			name:   "just bearer no token",
			header: http.Header{"Authorization": []string{"Bearer"}},
			want:   "",
		},
		{
			name:   "token with spaces",
			header: http.Header{"Authorization": []string{"Bearer token with spaces"}},
			want:   "token with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToken(tt.header)
			if got != tt.want {
				t.Errorf("extractToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateJWT(t *testing.T) {
	secret := []byte("test-secret-key-for-testing")
	handler, err := NewAuthHandler(nil, secret)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	t.Run("valid token", func(t *testing.T) {
		// Create a valid token
		now := time.Now()
		claims := &JWTClaims{
			Email: "test@example.com",
			Name:  "Test User",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		// Validate the token
		gotClaims, err := handler.validateJWT(tokenString)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if gotClaims == nil {
			t.Fatal("expected claims, got nil")
		}
		if gotClaims.Subject != "user-123" {
			t.Errorf("Subject = %q, want %q", gotClaims.Subject, "user-123")
		}
		if gotClaims.Email != "test@example.com" {
			t.Errorf("Email = %q, want %q", gotClaims.Email, "test@example.com")
		}
		if gotClaims.Name != "Test User" {
			t.Errorf("Name = %q, want %q", gotClaims.Name, "Test User")
		}
	})

	t.Run("expired token", func(t *testing.T) {
		// Create an expired token
		now := time.Now()
		claims := &JWTClaims{
			Email: "test@example.com",
			Name:  "Test User",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				IssuedAt:  jwt.NewNumericDate(now.Add(-1 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-30 * time.Minute)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		// Validate should fail
		_, err = handler.validateJWT(tokenString)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		// Create a token with different secret
		wrongSecret := []byte("wrong-secret")
		claims := &JWTClaims{
			Email: "test@example.com",
			Name:  "Test User",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "user-123",
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(wrongSecret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		// Validate should fail
		_, err = handler.validateJWT(tokenString)
		if err == nil {
			t.Error("expected error for invalid signature")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := handler.validateJWT("not-a-valid-jwt")
		if err == nil {
			t.Error("expected error for malformed token")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := handler.validateJWT("")
		if err == nil {
			t.Error("expected error for empty token")
		}
	})
}

func TestGetClientInfo(t *testing.T) {
	tests := []struct {
		name          string
		header        http.Header
		wantIP        string
		wantUserAgent string
	}{
		{
			name: "X-Forwarded-For present",
			header: http.Header{
				"X-Forwarded-For": []string{"192.168.1.1"},
				"User-Agent":      []string{"TestClient/1.0"},
			},
			wantIP:        "192.168.1.1",
			wantUserAgent: "TestClient/1.0",
		},
		{
			name: "X-Real-IP fallback",
			header: http.Header{
				"X-Real-Ip":  []string{"10.0.0.1"},
				"User-Agent": []string{"TestClient/2.0"},
			},
			wantIP:        "10.0.0.1",
			wantUserAgent: "TestClient/2.0",
		},
		{
			name: "X-Forwarded-For takes precedence",
			header: http.Header{
				"X-Forwarded-For": []string{"192.168.1.1"},
				"X-Real-Ip":       []string{"10.0.0.1"},
				"User-Agent":      []string{"TestClient/3.0"},
			},
			wantIP:        "192.168.1.1",
			wantUserAgent: "TestClient/3.0",
		},
		{
			name:          "no headers",
			header:        http.Header{},
			wantIP:        "",
			wantUserAgent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIP, gotUserAgent := getClientInfo(tt.header)
			if gotIP != tt.wantIP {
				t.Errorf("IP = %q, want %q", gotIP, tt.wantIP)
			}
			if gotUserAgent != tt.wantUserAgent {
				t.Errorf("UserAgent = %q, want %q", gotUserAgent, tt.wantUserAgent)
			}
		})
	}
}

func TestContextHelpers(t *testing.T) {
	t.Run("UserIDFromContext", func(t *testing.T) {
		// Test with no value
		_, ok := UserIDFromContext(t.Context())
		if ok {
			t.Error("expected ok to be false for missing value")
		}
	})

	t.Run("EmailFromContext", func(t *testing.T) {
		// Test with no value
		_, ok := EmailFromContext(t.Context())
		if ok {
			t.Error("expected ok to be false for missing value")
		}
	})

	t.Run("NameFromContext", func(t *testing.T) {
		// Test with no value
		_, ok := NameFromContext(t.Context())
		if ok {
			t.Error("expected ok to be false for missing value")
		}
	})
}

// =============================================================================
// RPC Handler Tests
// =============================================================================

func TestSignUp(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			createUserWithPasswordResp: &authinternalv1.CreateUserWithPasswordResponse{
				User: &authinternalv1.User{
					Id:            "user-123",
					Email:         "test@example.com",
					Name:          "Test User",
					EmailVerified: false,
					CreatedAt:     "2024-01-01T00:00:00Z",
					UpdatedAt:     "2024-01-01T00:00:00Z",
				},
			},
			createTokensResp: &authinternalv1.CreateTokensResponse{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-123",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignUpRequest{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User",
		})

		resp, err := handler.SignUp(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify response
		if resp.Msg.User == nil {
			t.Fatal("expected user in response")
		}
		if resp.Msg.User.Id != "user-123" {
			t.Errorf("User.Id = %q, want %q", resp.Msg.User.Id, "user-123")
		}
		if resp.Msg.AccessToken != "access-token-123" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "access-token-123")
		}
		if resp.Msg.RefreshToken != "refresh-token-123" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "refresh-token-123")
		}

		// Verify request was forwarded correctly
		if mock.lastCreateUserWithPasswordReq.Email != "test@example.com" {
			t.Errorf("CreateUserWithPassword email = %q, want %q", mock.lastCreateUserWithPasswordReq.Email, "test@example.com")
		}
		if mock.lastCreateUserWithPasswordReq.Password != "password123" {
			t.Errorf("CreateUserWithPassword password = %q, want %q", mock.lastCreateUserWithPasswordReq.Password, "password123")
		}
		if mock.lastCreateUserWithPasswordReq.Name != "Test User" {
			t.Errorf("CreateUserWithPassword name = %q, want %q", mock.lastCreateUserWithPasswordReq.Name, "Test User")
		}
	})

	t.Run("create user error", func(t *testing.T) {
		mock := &mockAuthClient{
			createUserWithPasswordErr: connect.NewError(connect.CodeAlreadyExists, errors.New("email already exists")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignUpRequest{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User",
		})

		_, err := handler.SignUp(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeAlreadyExists {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeAlreadyExists)
		}
	})

	t.Run("create tokens error", func(t *testing.T) {
		mock := &mockAuthClient{
			createUserWithPasswordResp: &authinternalv1.CreateUserWithPasswordResponse{
				User: &authinternalv1.User{
					Id:    "user-123",
					Email: "test@example.com",
					Name:  "Test User",
				},
			},
			createTokensErr: connect.NewError(connect.CodeInternal, errors.New("token creation failed")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignUpRequest{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User",
		})

		_, err := handler.SignUp(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nil user in response", func(t *testing.T) {
		mock := &mockAuthClient{
			createUserWithPasswordResp: &authinternalv1.CreateUserWithPasswordResponse{
				User: nil,
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignUpRequest{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User",
		})

		_, err := handler.SignUp(ctx, req)
		if err == nil {
			t.Fatal("expected error for nil user")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeInternal {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeInternal)
		}
	})
}

func TestSignIn(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			verifyCredentialsResp: &authinternalv1.VerifyCredentialsResponse{
				Valid: true,
				User: &authinternalv1.User{
					Id:    "user-123",
					Email: "test@example.com",
					Name:  "Test User",
				},
			},
			createTokensResp: &authinternalv1.CreateTokensResponse{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-123",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignInRequest{
			Email:    "test@example.com",
			Password: "password123",
		})

		resp, err := handler.SignIn(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Msg.User == nil {
			t.Fatal("expected user in response")
		}
		if resp.Msg.User.Id != "user-123" {
			t.Errorf("User.Id = %q, want %q", resp.Msg.User.Id, "user-123")
		}
		if resp.Msg.AccessToken != "access-token-123" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "access-token-123")
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		mock := &mockAuthClient{
			verifyCredentialsResp: &authinternalv1.VerifyCredentialsResponse{
				Valid: false,
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignInRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		})

		_, err := handler.SignIn(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeUnauthenticated)
		}
	})

	t.Run("verify credentials error", func(t *testing.T) {
		mock := &mockAuthClient{
			verifyCredentialsErr: connect.NewError(connect.CodeInternal, errors.New("database error")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignInRequest{
			Email:    "test@example.com",
			Password: "password123",
		})

		_, err := handler.SignIn(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		// Should return unauthenticated to not leak info
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeUnauthenticated)
		}
	})

	t.Run("valid but nil user", func(t *testing.T) {
		mock := &mockAuthClient{
			verifyCredentialsResp: &authinternalv1.VerifyCredentialsResponse{
				Valid: true,
				User:  nil,
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignInRequest{
			Email:    "test@example.com",
			Password: "password123",
		})

		_, err := handler.SignIn(ctx, req)
		if err == nil {
			t.Fatal("expected error for nil user")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeInternal {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeInternal)
		}
	})
}

func TestSignOut(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			revokeRefreshTokenResp: &authinternalv1.RevokeRefreshTokenResponse{
				Success: true,
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignOutRequest{
			RefreshToken: "refresh-token-123",
		})

		resp, err := handler.SignOut(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Msg.Success {
			t.Error("expected success to be true")
		}

		if mock.lastRevokeRefreshTokenReq.RefreshToken != "refresh-token-123" {
			t.Errorf("RevokeRefreshToken token = %q, want %q", mock.lastRevokeRefreshTokenReq.RefreshToken, "refresh-token-123")
		}
	})

	t.Run("empty refresh token", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignOutRequest{
			RefreshToken: "",
		})

		_, err := handler.SignOut(ctx, req)
		if err == nil {
			t.Fatal("expected error for empty refresh token")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeInvalidArgument)
		}
	})

	t.Run("revoke error", func(t *testing.T) {
		mock := &mockAuthClient{
			revokeRefreshTokenErr: connect.NewError(connect.CodeInternal, errors.New("revoke failed")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignOutRequest{
			RefreshToken: "refresh-token-123",
		})

		_, err := handler.SignOut(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRefreshToken(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			refreshTokensResp: &authinternalv1.RefreshTokensResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.RefreshTokenRequest{
			RefreshToken: "old-refresh-token",
		})

		resp, err := handler.RefreshToken(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Msg.AccessToken != "new-access-token" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "new-access-token")
		}
		if resp.Msg.RefreshToken != "new-refresh-token" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "new-refresh-token")
		}
	})

	t.Run("empty refresh token", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.RefreshTokenRequest{
			RefreshToken: "",
		})

		_, err := handler.RefreshToken(ctx, req)
		if err == nil {
			t.Fatal("expected error for empty refresh token")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeInvalidArgument {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeInvalidArgument)
		}
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		mock := &mockAuthClient{
			refreshTokensErr: connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.RefreshTokenRequest{
			RefreshToken: "invalid-token",
		})

		_, err := handler.RefreshToken(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeUnauthenticated)
		}
	})
}

func TestGetMe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			getUserResp: &authinternalv1.GetUserResponse{
				User: &authinternalv1.User{
					Id:            "user-123",
					Email:         "test@example.com",
					Name:          "Test User",
					EmailVerified: true,
					CreatedAt:     "2024-01-01T00:00:00Z",
					UpdatedAt:     "2024-01-01T00:00:00Z",
				},
			},
		}
		handler := newTestHandler(mock)

		// Create context with user ID (simulates authenticated request)
		ctx := context.WithValue(context.Background(), userIDKey, "user-123")

		req := connect.NewRequest(&emptypb.Empty{})

		resp, err := handler.GetMe(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Msg.User == nil {
			t.Fatal("expected user in response")
		}
		if resp.Msg.User.Id != "user-123" {
			t.Errorf("User.Id = %q, want %q", resp.Msg.User.Id, "user-123")
		}
		if resp.Msg.User.Email != "test@example.com" {
			t.Errorf("User.Email = %q, want %q", resp.Msg.User.Email, "test@example.com")
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		// No user ID in context
		ctx := context.Background()

		req := connect.NewRequest(&emptypb.Empty{})

		_, err := handler.GetMe(ctx, req)
		if err == nil {
			t.Fatal("expected error for unauthenticated request")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeUnauthenticated)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mock := &mockAuthClient{
			getUserResp: &authinternalv1.GetUserResponse{
				User: nil,
			},
		}
		handler := newTestHandler(mock)

		ctx := context.WithValue(context.Background(), userIDKey, "user-123")

		req := connect.NewRequest(&emptypb.Empty{})

		_, err := handler.GetMe(ctx, req)
		if err == nil {
			t.Fatal("expected error for user not found")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeNotFound {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeNotFound)
		}
	})
}

func TestGetLinkedAccounts(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			getAccountsByUserIdResp: &authinternalv1.GetAccountsByUserIdResponse{
				Accounts: []*authinternalv1.Account{
					{
						Id:                "acc-1",
						Provider:          authinternalv1.AuthProvider_AUTH_PROVIDER_EMAIL,
						ProviderAccountId: "email-id",
						CreatedAt:         "2024-01-01T00:00:00Z",
					},
					{
						Id:                "acc-2",
						Provider:          authinternalv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
						ProviderAccountId: "google-id",
						CreatedAt:         "2024-02-01T00:00:00Z",
					},
				},
				HasPassword: true,
			},
		}
		handler := newTestHandler(mock)

		ctx := context.WithValue(context.Background(), userIDKey, "user-123")

		req := connect.NewRequest(&emptypb.Empty{})

		resp, err := handler.GetLinkedAccounts(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Msg.Accounts) != 2 {
			t.Fatalf("expected 2 accounts, got %d", len(resp.Msg.Accounts))
		}
		if !resp.Msg.HasPassword {
			t.Error("expected HasPassword to be true")
		}
		if resp.Msg.Accounts[0].Provider != authv1.AuthProvider_AUTH_PROVIDER_EMAIL {
			t.Errorf("first account provider = %v, want EMAIL", resp.Msg.Accounts[0].Provider)
		}
		if resp.Msg.Accounts[1].Provider != authv1.AuthProvider_AUTH_PROVIDER_GOOGLE {
			t.Errorf("second account provider = %v, want GOOGLE", resp.Msg.Accounts[1].Provider)
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		ctx := context.Background()

		req := connect.NewRequest(&emptypb.Empty{})

		_, err := handler.GetLinkedAccounts(ctx, req)
		if err == nil {
			t.Fatal("expected error for unauthenticated request")
		}

		connectErr := new(connect.Error)
		if !errors.As(err, &connectErr) {
			t.Fatalf("expected connect.Error, got %T", err)
		}
		if connectErr.Code() != connect.CodeUnauthenticated {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeUnauthenticated)
		}
	})
}

func TestGetOAuthUrl(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			getOAuthUrlResp: &authinternalv1.GetOAuthUrlResponse{
				Url:   "https://accounts.google.com/oauth?...",
				State: "random-state-123",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.GetOAuthUrlRequest{
			Provider:    authv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			CallbackUrl: "https://example.com/callback",
		})

		resp, err := handler.GetOAuthUrl(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Msg.Url != "https://accounts.google.com/oauth?..." {
			t.Errorf("Url = %q, want %q", resp.Msg.Url, "https://accounts.google.com/oauth?...")
		}
		if resp.Msg.State != "random-state-123" {
			t.Errorf("State = %q, want %q", resp.Msg.State, "random-state-123")
		}

		// Verify request was forwarded correctly
		if mock.lastGetOAuthUrlReq.Provider != authinternalv1.AuthProvider_AUTH_PROVIDER_GOOGLE {
			t.Errorf("GetOAuthUrl provider = %v, want GOOGLE", mock.lastGetOAuthUrlReq.Provider)
		}
		if mock.lastGetOAuthUrlReq.CallbackUrl != "https://example.com/callback" {
			t.Errorf("GetOAuthUrl callback = %q, want %q", mock.lastGetOAuthUrlReq.CallbackUrl, "https://example.com/callback")
		}
	})

	t.Run("error", func(t *testing.T) {
		mock := &mockAuthClient{
			getOAuthUrlErr: connect.NewError(connect.CodeInvalidArgument, errors.New("unsupported provider")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.GetOAuthUrlRequest{
			Provider:    authv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			CallbackUrl: "https://example.com/callback",
		})

		_, err := handler.GetOAuthUrl(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleOAuthCallback(t *testing.T) {
	ctx := context.Background()

	t.Run("success - new user", func(t *testing.T) {
		mock := &mockAuthClient{
			exchangeOAuthCodeResp: &authinternalv1.ExchangeOAuthCodeResponse{
				User: &authinternalv1.User{
					Id:    "user-new",
					Email: "newuser@gmail.com",
					Name:  "New User",
				},
				AccessToken:  "oauth-access-token",
				RefreshToken: "oauth-refresh-token",
				IsNewUser:    true,
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.HandleOAuthCallbackRequest{
			Provider: authv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			Code:     "auth-code-123",
			State:    func() *string { s := "state-123"; return &s }(),
		})

		resp, err := handler.HandleOAuthCallback(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Msg.User == nil {
			t.Fatal("expected user in response")
		}
		if !resp.Msg.IsNewUser {
			t.Error("expected IsNewUser to be true")
		}
		if resp.Msg.AccessToken != "oauth-access-token" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "oauth-access-token")
		}
	})

	t.Run("success - existing user", func(t *testing.T) {
		mock := &mockAuthClient{
			exchangeOAuthCodeResp: &authinternalv1.ExchangeOAuthCodeResponse{
				User: &authinternalv1.User{
					Id:    "user-existing",
					Email: "existing@gmail.com",
					Name:  "Existing User",
				},
				AccessToken:  "oauth-access-token",
				RefreshToken: "oauth-refresh-token",
				IsNewUser:    false,
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.HandleOAuthCallbackRequest{
			Provider: authv1.AuthProvider_AUTH_PROVIDER_GIT_HUB,
			Code:     "auth-code-456",
		})

		resp, err := handler.HandleOAuthCallback(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Msg.IsNewUser {
			t.Error("expected IsNewUser to be false")
		}
	})

	t.Run("error", func(t *testing.T) {
		mock := &mockAuthClient{
			exchangeOAuthCodeErr: connect.NewError(connect.CodeUnauthenticated, errors.New("invalid code")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.HandleOAuthCallbackRequest{
			Provider: authv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			Code:     "invalid-code",
		})

		_, err := handler.HandleOAuthCallback(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestConvertInternalUser(t *testing.T) {
	t.Run("nil user", func(t *testing.T) {
		result := convertInternalUser(nil)
		if result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("full user", func(t *testing.T) {
		internal := &authinternalv1.User{
			Id:            "user-123",
			Email:         "test@example.com",
			Name:          "Test User",
			Image:         func() *string { s := "https://example.com/avatar.png"; return &s }(),
			EmailVerified: true,
			CreatedAt:     "2024-01-01T00:00:00Z",
			UpdatedAt:     "2024-01-02T00:00:00Z",
		}

		result := convertInternalUser(internal)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Id != "user-123" {
			t.Errorf("Id = %q, want %q", result.Id, "user-123")
		}
		if result.Email != "test@example.com" {
			t.Errorf("Email = %q, want %q", result.Email, "test@example.com")
		}
		if result.Name != "Test User" {
			t.Errorf("Name = %q, want %q", result.Name, "Test User")
		}
		if result.Image == nil || *result.Image != "https://example.com/avatar.png" {
			t.Errorf("Image = %v, want %q", result.Image, "https://example.com/avatar.png")
		}
		if !result.EmailVerified {
			t.Error("expected EmailVerified to be true")
		}
	})
}
