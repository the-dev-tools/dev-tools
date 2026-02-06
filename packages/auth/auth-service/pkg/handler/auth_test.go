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

	getTokenResp *authinternalv1.GetTokenResponse
	getTokenErr  error

	getOAuthUrlResp *authinternalv1.GetOAuthUrlResponse
	getOAuthUrlErr  error

	exchangeOAuthCodeResp *authinternalv1.ExchangeOAuthCodeResponse
	exchangeOAuthCodeErr  error

	getAccountsByUserIdResp *authinternalv1.GetAccountsByUserIdResponse
	getAccountsByUserIdErr  error

	// Capture request data for assertions
	lastCreateUserWithPasswordReq *authinternalv1.CreateUserWithPasswordRequest
	lastVerifyCredentialsReq      *authinternalv1.VerifyCredentialsRequest
	lastGetTokenReq               *authinternalv1.GetTokenRequest
	lastGetOAuthUrlReq            *authinternalv1.GetOAuthUrlRequest
	lastExchangeOAuthCodeReq      *authinternalv1.ExchangeOAuthCodeRequest
	lastGetAccountsByUserIdReq    *authinternalv1.GetAccountsByUserIdRequest
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

func (m *mockAuthClient) GetToken(_ context.Context, req *connect.Request[authinternalv1.GetTokenRequest]) (*connect.Response[authinternalv1.GetTokenResponse], error) {
	m.lastGetTokenReq = req.Msg
	if m.getTokenErr != nil {
		return nil, m.getTokenErr
	}
	return connect.NewResponse(m.getTokenResp), nil
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

var testSecret = []byte("test-secret")

// newTestHandler creates an AuthHandler with a mock client and HMAC keyfunc for testing.
func newTestHandler(mock *mockAuthClient) *AuthHandler {
	return NewAuthHandlerWithKeyfunc(mock, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return testSecret, nil
	})
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
		{
			name:      "GetSSOUrl is unauthenticated",
			procedure: "/api.auth.v1.AuthService/GetSSOUrl",
			want:      true,
		},
		{
			name:      "HandleSSOCallback is unauthenticated",
			procedure: "/api.auth.v1.AuthService/HandleSSOCallback",
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
	expected := []string{"SignUp", "SignIn", "GetOAuthUrl", "HandleOAuthCallback", "RefreshToken", "GetSSOUrl", "HandleSSOCallback"}

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

func TestNewAuthHandler_RequiresJWKSURL(t *testing.T) {
	// Empty JWKS URL should fail
	_, err := NewAuthHandler(nil, "")
	if err == nil {
		t.Error("expected error for empty JWKS URL")
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
	handler := newTestHandler(&mockAuthClient{})

	t.Run("valid token", func(t *testing.T) {
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
		tokenString, err := token.SignedString(testSecret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

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
		tokenString, err := token.SignedString(testSecret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = handler.validateJWT(tokenString)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
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

func TestContextHelpers(t *testing.T) {
	t.Run("UserIDFromContext", func(t *testing.T) {
		_, ok := UserIDFromContext(t.Context())
		if ok {
			t.Error("expected ok to be false for missing value")
		}
	})

	t.Run("EmailFromContext", func(t *testing.T) {
		_, ok := EmailFromContext(t.Context())
		if ok {
			t.Error("expected ok to be false for missing value")
		}
	})

	t.Run("NameFromContext", func(t *testing.T) {
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
				SessionToken: "session-token-123",
			},
			getTokenResp: &authinternalv1.GetTokenResponse{
				AccessToken: "jwt-access-token-123",
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

		if resp.Msg.User == nil {
			t.Fatal("expected user in response")
		}
		if resp.Msg.User.Id != "user-123" {
			t.Errorf("User.Id = %q, want %q", resp.Msg.User.Id, "user-123")
		}
		if resp.Msg.AccessToken != "jwt-access-token-123" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "jwt-access-token-123")
		}
		if resp.Msg.RefreshToken != "session-token-123" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "session-token-123")
		}

		// Verify GetToken was called with the session token
		if mock.lastGetTokenReq.SessionToken != "session-token-123" {
			t.Errorf("GetToken sessionToken = %q, want %q", mock.lastGetTokenReq.SessionToken, "session-token-123")
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

	t.Run("get token error", func(t *testing.T) {
		mock := &mockAuthClient{
			createUserWithPasswordResp: &authinternalv1.CreateUserWithPasswordResponse{
				User: &authinternalv1.User{
					Id:    "user-123",
					Email: "test@example.com",
					Name:  "Test User",
				},
				SessionToken: "session-token-123",
			},
			getTokenErr: connect.NewError(connect.CodeInternal, errors.New("token creation failed")),
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
		sessionToken := "session-token-456"
		mock := &mockAuthClient{
			verifyCredentialsResp: &authinternalv1.VerifyCredentialsResponse{
				Valid: true,
				User: &authinternalv1.User{
					Id:    "user-123",
					Email: "test@example.com",
					Name:  "Test User",
				},
				SessionToken: &sessionToken,
			},
			getTokenResp: &authinternalv1.GetTokenResponse{
				AccessToken: "jwt-access-token-456",
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
		if resp.Msg.AccessToken != "jwt-access-token-456" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "jwt-access-token-456")
		}
		if resp.Msg.RefreshToken != "session-token-456" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "session-token-456")
		}

		// Verify GetToken was called with session token
		if mock.lastGetTokenReq.SessionToken != "session-token-456" {
			t.Errorf("GetToken sessionToken = %q, want %q", mock.lastGetTokenReq.SessionToken, "session-token-456")
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

	t.Run("success - stateless", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		refreshToken := "some-refresh-token"
		req := connect.NewRequest(&authv1.SignOutRequest{
			RefreshToken: &refreshToken,
		})

		resp, err := handler.SignOut(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Msg.Success {
			t.Error("expected success to be true")
		}
	})

	t.Run("success - no refresh token (optional)", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignOutRequest{})

		resp, err := handler.SignOut(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !resp.Msg.Success {
			t.Error("expected success to be true")
		}
	})
}

func TestRefreshToken(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			getTokenResp: &authinternalv1.GetTokenResponse{
				AccessToken: "new-jwt-access-token",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.RefreshTokenRequest{
			RefreshToken: "session-token-789",
		})

		resp, err := handler.RefreshToken(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.Msg.AccessToken != "new-jwt-access-token" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "new-jwt-access-token")
		}
		if resp.Msg.RefreshToken != "session-token-789" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "session-token-789")
		}

		// Verify GetToken was called with session token
		if mock.lastGetTokenReq.SessionToken != "session-token-789" {
			t.Errorf("GetToken sessionToken = %q, want %q", mock.lastGetTokenReq.SessionToken, "session-token-789")
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

	t.Run("invalid session token", func(t *testing.T) {
		mock := &mockAuthClient{
			getTokenErr: connect.NewError(connect.CodeUnauthenticated, errors.New("session invalid")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.RefreshTokenRequest{
			RefreshToken: "invalid-session-token",
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
	t.Run("success - serves from JWT claims", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		ctx := context.WithValue(context.Background(), userIDKey, "user-123")
		ctx = context.WithValue(ctx, emailKey, "test@example.com")
		ctx = context.WithValue(ctx, nameKey, "Test User")

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
		if resp.Msg.User.Name != "Test User" {
			t.Errorf("User.Name = %q, want %q", resp.Msg.User.Name, "Test User")
		}
	})

	t.Run("not authenticated", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

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
				SessionToken: "oauth-session-token",
				IsNewUser:    true,
			},
			getTokenResp: &authinternalv1.GetTokenResponse{
				AccessToken: "oauth-jwt-access-token",
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
		if resp.Msg.AccessToken != "oauth-jwt-access-token" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "oauth-jwt-access-token")
		}
		if resp.Msg.RefreshToken != "oauth-session-token" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "oauth-session-token")
		}

		// Verify GetToken was called
		if mock.lastGetTokenReq.SessionToken != "oauth-session-token" {
			t.Errorf("GetToken sessionToken = %q, want %q", mock.lastGetTokenReq.SessionToken, "oauth-session-token")
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
				SessionToken: "existing-session-token",
				IsNewUser:    false,
			},
			getTokenResp: &authinternalv1.GetTokenResponse{
				AccessToken: "existing-jwt-access-token",
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

func TestGetSSOUrl(t *testing.T) {
	ctx := context.Background()

	mock := &mockAuthClient{}
	handler := newTestHandler(mock)

	req := connect.NewRequest(&authv1.GetSSOUrlRequest{
		OrganizationId: "org-123",
		CallbackUrl:    "https://example.com/sso/callback",
	})

	_, err := handler.GetSSOUrl(ctx, req)
	if err == nil {
		t.Fatal("expected error (unimplemented)")
	}

	connectErr := new(connect.Error)
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeUnimplemented {
		t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeUnimplemented)
	}
}

func TestHandleSSOCallback(t *testing.T) {
	ctx := context.Background()

	mock := &mockAuthClient{}
	handler := newTestHandler(mock)

	req := connect.NewRequest(&authv1.HandleSSOCallbackRequest{
		Code: "sso-code-123",
	})

	_, err := handler.HandleSSOCallback(ctx, req)
	if err == nil {
		t.Fatal("expected error (unimplemented)")
	}

	connectErr := new(connect.Error)
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeUnimplemented {
		t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeUnimplemented)
	}
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
