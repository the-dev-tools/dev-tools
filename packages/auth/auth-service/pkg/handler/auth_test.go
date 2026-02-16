package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
	"github.com/the-dev-tools/dev-tools/packages/auth/authlib/jwks"
	authv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth/v1"
	authcommonv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_common/v1"
	authinternalv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/auth_internal/v1/auth_internalv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Test ULID constants — valid ULIDs for use in mocks and assertions.
var (
	testUserULID    = ulid.MustParse("01HGW3KFZJ8V7N4P0R2S5T6X9Y")
	testUserIDBytes = testUserULID[:]

	testNewUserULID    = ulid.MustParse("01HGW3KFZN8V7N4P0R2S5T6X9C")
	testNewUserIDBytes = testNewUserULID[:]

	testExistingUserULID    = ulid.MustParse("01HGW3KFZP8V7N4P0R2S5T6X9D")
	testExistingUserIDBytes = testExistingUserULID[:]
)

// mockAuthClient implements auth_internalv1connect.AuthInternalServiceClient for testing.
type mockAuthClient struct {
	auth_internalv1connect.UnimplementedAuthInternalServiceHandler

	// Response stubs
	createUserWithPasswordResp *authinternalv1.CreateUserWithPasswordResponse
	createUserWithPasswordErr  error

	verifyCredentialsResp *authinternalv1.VerifyCredentialsResponse
	verifyCredentialsErr  error

	tokenResp *authinternalv1.TokenResponse
	tokenErr  error

	oAuthUrlResp *authinternalv1.OAuthUrlResponse
	oAuthUrlErr  error

	exchangeOAuthCodeResp *authinternalv1.ExchangeOAuthCodeResponse
	exchangeOAuthCodeErr  error

	accountsByUserIdResp *authinternalv1.AccountsByUserIdResponse
	accountsByUserIdErr  error

	revokeSessionResp *authinternalv1.RevokeSessionResponse
	revokeSessionErr  error

	// Capture request data for assertions
	lastCreateUserWithPasswordReq *authinternalv1.CreateUserWithPasswordRequest
	lastVerifyCredentialsReq      *authinternalv1.VerifyCredentialsRequest
	lastTokenReq                  *authinternalv1.TokenRequest
	lastOAuthUrlReq               *authinternalv1.OAuthUrlRequest
	lastExchangeOAuthCodeReq      *authinternalv1.ExchangeOAuthCodeRequest
	lastAccountsByUserIdReq       *authinternalv1.AccountsByUserIdRequest
	lastRevokeSessionReq          *authinternalv1.RevokeSessionRequest
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

func (m *mockAuthClient) Token(_ context.Context, req *connect.Request[authinternalv1.TokenRequest]) (*connect.Response[authinternalv1.TokenResponse], error) {
	m.lastTokenReq = req.Msg
	if m.tokenErr != nil {
		return nil, m.tokenErr
	}
	return connect.NewResponse(m.tokenResp), nil
}

func (m *mockAuthClient) OAuthUrl(_ context.Context, req *connect.Request[authinternalv1.OAuthUrlRequest]) (*connect.Response[authinternalv1.OAuthUrlResponse], error) {
	m.lastOAuthUrlReq = req.Msg
	if m.oAuthUrlErr != nil {
		return nil, m.oAuthUrlErr
	}
	return connect.NewResponse(m.oAuthUrlResp), nil
}

func (m *mockAuthClient) ExchangeOAuthCode(_ context.Context, req *connect.Request[authinternalv1.ExchangeOAuthCodeRequest]) (*connect.Response[authinternalv1.ExchangeOAuthCodeResponse], error) {
	m.lastExchangeOAuthCodeReq = req.Msg
	if m.exchangeOAuthCodeErr != nil {
		return nil, m.exchangeOAuthCodeErr
	}
	return connect.NewResponse(m.exchangeOAuthCodeResp), nil
}

func (m *mockAuthClient) AccountsByUserId(_ context.Context, req *connect.Request[authinternalv1.AccountsByUserIdRequest]) (*connect.Response[authinternalv1.AccountsByUserIdResponse], error) {
	m.lastAccountsByUserIdReq = req.Msg
	if m.accountsByUserIdErr != nil {
		return nil, m.accountsByUserIdErr
	}
	return connect.NewResponse(m.accountsByUserIdResp), nil
}

func (m *mockAuthClient) RevokeSession(_ context.Context, req *connect.Request[authinternalv1.RevokeSessionRequest]) (*connect.Response[authinternalv1.RevokeSessionResponse], error) {
	m.lastRevokeSessionReq = req.Msg
	if m.revokeSessionErr != nil {
		return nil, m.revokeSessionErr
	}
	if m.revokeSessionResp != nil {
		return connect.NewResponse(m.revokeSessionResp), nil
	}
	return connect.NewResponse(&authinternalv1.RevokeSessionResponse{}), nil
}

func (m *mockAuthClient) CreateAccount(_ context.Context, _ *connect.Request[authinternalv1.CreateAccountRequest]) (*connect.Response[authinternalv1.CreateAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

var testSecret = []byte("test-secret")

// newTestHandler creates an AuthHandler with a mock client and HMAC keyfunc for testing.
func newTestHandler(mock *mockAuthClient) *AuthHandler {
	return NewAuthHandler(mock, func(token *jwt.Token) (interface{}, error) {
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
			name:      "OAuthUrl is unauthenticated",
			procedure: "/api.auth.v1.AuthService/OAuthUrl",
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
			name:      "SignOut is unauthenticated",
			procedure: "/api.auth.v1.AuthService/SignOut",
			want:      true,
		},
		// Authenticated endpoints (should return false)
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
			want:      false, // Exact match: different service must not match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := unauthenticatedEndpoints[tt.procedure]
			if got != tt.want {
				t.Errorf("unauthenticatedEndpoints[%q] = %v, want %v", tt.procedure, got, tt.want)
			}
		})
	}
}

func TestUnauthenticatedEndpoints(t *testing.T) {
	expected := []string{
		"/api.auth.v1.AuthService/SignUp",
		"/api.auth.v1.AuthService/SignIn",
		"/api.auth.v1.AuthService/OAuthUrl",
		"/api.auth.v1.AuthService/HandleOAuthCallback",
		"/api.auth.v1.AuthService/RefreshToken",
	}

	for _, endpoint := range expected {
		if _, ok := unauthenticatedEndpoints[endpoint]; !ok {
			t.Errorf("expected %q to be in unauthenticatedEndpoints", endpoint)
		}
	}

	// Verify SignOut IS in the map (unauthenticated so expired-token users can revoke sessions)
	if _, ok := unauthenticatedEndpoints["/api.auth.v1.AuthService/SignOut"]; !ok {
		t.Error("SignOut should be in unauthenticatedEndpoints")
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
	kf := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return testSecret, nil
	}

	t.Run("valid token", func(t *testing.T) {
		now := time.Now()
		claims := &jwks.Claims{
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

		gotClaims, err := jwks.ValidateJWT(tokenString, kf)
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
		claims := &jwks.Claims{
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

		_, err = jwks.ValidateJWT(tokenString, kf)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		wrongSecret := []byte("wrong-secret")
		claims := &jwks.Claims{
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

		_, err = jwks.ValidateJWT(tokenString, kf)
		if err == nil {
			t.Error("expected error for invalid signature")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := jwks.ValidateJWT("not-a-valid-jwt", kf)
		if err == nil {
			t.Error("expected error for malformed token")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := jwks.ValidateJWT("", kf)
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
	testTime := timestamppb.Now()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			createUserWithPasswordResp: &authinternalv1.CreateUserWithPasswordResponse{
				User: &authinternalv1.User{
					Id:            testUserIDBytes,
					Email:         "test@example.com",
					Name:          "Test User",
					EmailVerified: false,
					CreatedAt:     testTime,
					UpdatedAt:     testTime,
				},
				SessionToken: "session-token-123",
			},
			tokenResp: &authinternalv1.TokenResponse{
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

		if !bytes.Equal(resp.Msg.UserId, testUserIDBytes) {
			t.Errorf("UserId = %x, want %x", resp.Msg.UserId, testUserIDBytes)
		}
		if resp.Msg.AccessToken != "jwt-access-token-123" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "jwt-access-token-123")
		}
		if resp.Msg.RefreshToken != "session-token-123" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "session-token-123")
		}

		// Verify Token was called with the session token
		if mock.lastTokenReq.SessionToken != "session-token-123" {
			t.Errorf("Token sessionToken = %q, want %q", mock.lastTokenReq.SessionToken, "session-token-123")
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
					Id:    testUserIDBytes,
					Email: "test@example.com",
					Name:  "Test User",
				},
				SessionToken: "session-token-123",
			},
			tokenErr: connect.NewError(connect.CodeInternal, errors.New("token creation failed")),
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
					Id:    testUserIDBytes,
					Email: "test@example.com",
					Name:  "Test User",
				},
				SessionToken: &sessionToken,
			},
			tokenResp: &authinternalv1.TokenResponse{
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

		if !bytes.Equal(resp.Msg.UserId, testUserIDBytes) {
			t.Errorf("UserId = %x, want %x", resp.Msg.UserId, testUserIDBytes)
		}
		if resp.Msg.AccessToken != "jwt-access-token-456" {
			t.Errorf("AccessToken = %q, want %q", resp.Msg.AccessToken, "jwt-access-token-456")
		}
		if resp.Msg.RefreshToken != "session-token-456" {
			t.Errorf("RefreshToken = %q, want %q", resp.Msg.RefreshToken, "session-token-456")
		}

		// Verify Token was called with session token
		if mock.lastTokenReq.SessionToken != "session-token-456" {
			t.Errorf("Token sessionToken = %q, want %q", mock.lastTokenReq.SessionToken, "session-token-456")
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
		if connectErr.Code() != connect.CodeInternal {
			t.Errorf("error code = %v, want %v", connectErr.Code(), connect.CodeInternal)
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

	t.Run("success - revokes session", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		refreshToken := "some-refresh-token"
		req := connect.NewRequest(&authv1.SignOutRequest{
			RefreshToken: &refreshToken,
		})

		_, err := handler.SignOut(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify RevokeSession was called with the refresh token
		if mock.lastRevokeSessionReq == nil {
			t.Fatal("expected RevokeSession to be called")
		}
		if mock.lastRevokeSessionReq.SessionToken != "some-refresh-token" {
			t.Errorf("RevokeSession sessionToken = %q, want %q", mock.lastRevokeSessionReq.SessionToken, "some-refresh-token")
		}
	})

	t.Run("success - no refresh token (optional)", func(t *testing.T) {
		mock := &mockAuthClient{}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.SignOutRequest{})

		_, err := handler.SignOut(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify RevokeSession was NOT called
		if mock.lastRevokeSessionReq != nil {
			t.Error("expected RevokeSession to NOT be called when no refresh token")
		}
	})

	t.Run("success - revoke error is non-fatal", func(t *testing.T) {
		mock := &mockAuthClient{
			revokeSessionErr: connect.NewError(connect.CodeInternal, errors.New("session already expired")),
		}
		handler := newTestHandler(mock)

		refreshToken := "expired-token"
		req := connect.NewRequest(&authv1.SignOutRequest{
			RefreshToken: &refreshToken,
		})

		_, err := handler.SignOut(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRefreshToken(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			tokenResp: &authinternalv1.TokenResponse{
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

		// Verify Token was called with session token
		if mock.lastTokenReq.SessionToken != "session-token-789" {
			t.Errorf("Token sessionToken = %q, want %q", mock.lastTokenReq.SessionToken, "session-token-789")
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
			tokenErr: connect.NewError(connect.CodeUnauthenticated, errors.New("session invalid")),
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

func TestOAuthUrl(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock := &mockAuthClient{
			oAuthUrlResp: &authinternalv1.OAuthUrlResponse{
				Url:   "https://accounts.google.com/oauth?...",
				State: "random-state-123",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.OAuthUrlRequest{
			Provider:    authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			CallbackUrl: "https://example.com/callback",
		})

		resp, err := handler.OAuthUrl(ctx, req)
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
			oAuthUrlErr: connect.NewError(connect.CodeInvalidArgument, errors.New("unsupported provider")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.OAuthUrlRequest{
			Provider:    authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			CallbackUrl: "https://example.com/callback",
		})

		_, err := handler.OAuthUrl(ctx, req)
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
					Id:    testNewUserIDBytes,
					Email: "newuser@gmail.com",
					Name:  "New User",
				},
				SessionToken: "oauth-session-token",
				IsNewUser:    true,
			},
			tokenResp: &authinternalv1.TokenResponse{
				AccessToken: "oauth-jwt-access-token",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.HandleOAuthCallbackRequest{
			Provider: authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			Code:     "auth-code-123",
			State:    func() *string { s := "state-123"; return &s }(),
		})

		resp, err := handler.HandleOAuthCallback(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !bytes.Equal(resp.Msg.UserId, testNewUserIDBytes) {
			t.Errorf("UserId = %x, want %x", resp.Msg.UserId, testNewUserIDBytes)
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

		// Verify Token was called
		if mock.lastTokenReq.SessionToken != "oauth-session-token" {
			t.Errorf("Token sessionToken = %q, want %q", mock.lastTokenReq.SessionToken, "oauth-session-token")
		}
	})

	t.Run("success - existing user", func(t *testing.T) {
		mock := &mockAuthClient{
			exchangeOAuthCodeResp: &authinternalv1.ExchangeOAuthCodeResponse{
				User: &authinternalv1.User{
					Id:    testExistingUserIDBytes,
					Email: "existing@gmail.com",
					Name:  "Existing User",
				},
				SessionToken: "existing-session-token",
				IsNewUser:    false,
			},
			tokenResp: &authinternalv1.TokenResponse{
				AccessToken: "existing-jwt-access-token",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.HandleOAuthCallbackRequest{
			Provider: authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
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

	t.Run("nil user in response", func(t *testing.T) {
		mock := &mockAuthClient{
			exchangeOAuthCodeResp: &authinternalv1.ExchangeOAuthCodeResponse{
				User:         nil,
				SessionToken: "oauth-session-token",
			},
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.HandleOAuthCallbackRequest{
			Provider: authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			Code:     "auth-code-nil-user",
		})

		_, err := handler.HandleOAuthCallback(ctx, req)
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

	t.Run("error", func(t *testing.T) {
		mock := &mockAuthClient{
			exchangeOAuthCodeErr: connect.NewError(connect.CodeUnauthenticated, errors.New("invalid code")),
		}
		handler := newTestHandler(mock)

		req := connect.NewRequest(&authv1.HandleOAuthCallbackRequest{
			Provider: authcommonv1.AuthProvider_AUTH_PROVIDER_GOOGLE,
			Code:     "invalid-code",
		})

		_, err := handler.HandleOAuthCallback(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
