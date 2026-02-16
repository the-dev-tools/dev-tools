package mwauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/auth/authlib/jwks"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/stoken"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock helpers

func mockUnaryNext(t *testing.T, expectedID idwrap.IDWrap) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		id, err := GetContextUserID(ctx)
		if expectedID == (idwrap.IDWrap{}) {
			// If we expect no ID, GetContextUserID should return error.
			// We don't check here because some callers want to verify that
			// authentication failed before reaching this point.
			_ = err
		} else {
			require.NoError(t, err)
			assert.Equal(t, expectedID, id)
		}
		return connect.NewResponse(&struct{}{}), nil
	}
}

func mockUnaryPanicNext(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
	panic("test panic")
}

func TestNewAuthInterceptorLocal(t *testing.T) {
	interceptor := NewAuthInterceptorLocal()
	next := mockUnaryNext(t, LocalDummyID)

	req := connect.NewRequest(&struct{}{})
	_, err := interceptor(next)(context.Background(), req)
	assert.NoError(t, err)
}

func TestAuthInterceptorLocalStreamHandlerLocal(t *testing.T) {
	interceptor := AuthInterceptorLocalStreamHandlerLocal(func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		id, err := GetContextUserID(ctx)
		require.NoError(t, err)
		assert.Equal(t, LocalDummyID, id)
		return nil
	})

	err := interceptor(context.Background(), nil)
	assert.NoError(t, err)
}

// Mock request to control Spec().IsClient
type mockRequest struct {
	connect.AnyRequest
	isClient bool
}

func (m mockRequest) Spec() connect.Spec {
	return connect.Spec{
		IsClient: m.isClient,
	}
}

func (m mockRequest) Header() http.Header {
	if m.AnyRequest != nil {
		return m.AnyRequest.Header()
	}
	return http.Header{}
}

func TestCrashInterceptor(t *testing.T) {
	t.Run("NoPanic", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		mReq := &mockRequest{AnyRequest: req, isClient: false}

		resp, err := CrashInterceptor(context.Background(), mReq, func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			return connect.NewResponse(&struct{}{}), nil
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("Panic", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		mReq := &mockRequest{AnyRequest: req, isClient: false}

		resp, err := CrashInterceptor(context.Background(), mReq, mockUnaryPanicNext)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "panic: test panic")
		assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	})

	t.Run("ClientSkip", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		mReq := &mockRequest{AnyRequest: req, isClient: true}

		assert.Panics(t, func() {
			_, _ = CrashInterceptor(context.Background(), mReq, mockUnaryPanicNext)
		})
	})
}

func TestNewAuthInterceptorOne(t *testing.T) {
	secret := []byte("secret")
	interceptor := NewAuthInterceptorOne(secret)

	t.Run("NoToken", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		_, err := interceptor(mockUnaryNext(t, idwrap.IDWrap{}))(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("InvalidTokenFormat", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})
		req.Header().Set(stoken.TokenHeaderKey, "InvalidFormat")
		_, err := interceptor(mockUnaryNext(t, idwrap.IDWrap{}))(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		req := connect.NewRequest(&struct{}{})

		// Create token with different secret
		token, err := stoken.NewJWT(idwrap.NewNow(), "test@example.com", stoken.AccessToken, time.Hour, []byte("wrong"))
		require.NoError(t, err)

		req.Header().Set(stoken.TokenHeaderKey, "Bearer "+token)
		_, err = interceptor(mockUnaryNext(t, idwrap.IDWrap{}))(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("ValidToken", func(t *testing.T) {
		id := idwrap.NewNow()
		token, err := stoken.NewJWT(id, "test@example.com", stoken.AccessToken, time.Hour, secret)
		require.NoError(t, err)

		req := connect.NewRequest(&struct{}{})
		req.Header().Set(stoken.TokenHeaderKey, "Bearer "+token)

		_, err = interceptor(mockUnaryNext(t, id))(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("InvalidSubjectID", func(t *testing.T) {
		// Create token with invalid ULID in subject
		claims := stoken.DefaultClaims{
			TokenType: stoken.AccessToken,
			Email:     "test@example.com",
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "devtools-server",
				Subject:   "not-a-ulid",
				Audience:  jwt.ClaimStrings{"devtools-server"},
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secret)
		require.NoError(t, err)

		req := connect.NewRequest(&struct{}{})
		req.Header().Set(stoken.TokenHeaderKey, "Bearer "+tokenString)

		_, err = interceptor(mockUnaryNext(t, idwrap.IDWrap{}))(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})
}

func TestGetContextUserID(t *testing.T) {
	t.Run("Found", func(t *testing.T) {
		ctx := CreateAuthedContext(context.Background(), LocalDummyID)
		id, err := GetContextUserID(ctx)
		assert.NoError(t, err)
		assert.Equal(t, LocalDummyID, id)
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx := context.Background()
		_, err := GetContextUserID(ctx)
		assert.Error(t, err)
		assert.Equal(t, errors.New("user id not found in context"), err)
	})
}

func TestAuthInterceptor_Methods(t *testing.T) {
	i := NewAuthInterceptor()

	t.Run("WrapUnary", func(t *testing.T) {
		wrapped := i.WrapUnary(mockUnaryNext(t, LocalDummyID))
		req := connect.NewRequest(&struct{}{})
		_, err := wrapped(context.Background(), req)
		assert.NoError(t, err)
	})

	t.Run("WrapStreamingClient", func(t *testing.T) {
		// Mock next function
		next := func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
			id, err := GetContextUserID(ctx)
			require.NoError(t, err)
			assert.Equal(t, LocalDummyID, id)
			return nil
		}
		wrapped := i.WrapStreamingClient(next)
		wrapped(context.Background(), connect.Spec{})
	})

	t.Run("WrapStreamingHandler", func(t *testing.T) {
		next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			id, err := GetContextUserID(ctx)
			require.NoError(t, err)
			assert.Equal(t, LocalDummyID, id)
			return nil
		}
		wrapped := i.WrapStreamingHandler(next)
		err := wrapped(context.Background(), nil)
		assert.NoError(t, err)
	})
}

// =============================================================================
// BetterAuth interceptor tests (JWKS + auto-provisioning)
// =============================================================================

var testHMACSecret = []byte("test-betterauth-secret")

// testHMACKeyfunc returns a jwt.Keyfunc that validates HMAC tokens (for testing).
func testHMACKeyfunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return testHMACSecret, nil
	}
}

// createTestBetterAuthJWT creates a JWT token with BetterAuth-style claims for testing.
func createTestBetterAuthJWT(t *testing.T, sub, email, name string, expired bool) string {
	t.Helper()

	expiry := time.Now().Add(time.Hour)
	if expired {
		expiry = time.Now().Add(-time.Hour)
	}

	claims := jwks.Claims{
		Email: email,
		Name:  name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(testHMACSecret)
	require.NoError(t, err)
	return tokenString
}

// setupTestUserService creates an in-memory SQLite database and returns a UserService for testing.
func setupTestUserService(t *testing.T) (suser.UserService, *sql.DB) {
	t.Helper()

	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)

	return suser.New(gen.New(db)), db
}

func TestBetterAuthInterceptor_ExtractUserID(t *testing.T) {
	us, db := setupTestUserService(t)
	interceptor := NewBetterAuthInterceptor(testHMACKeyfunc(), us)

	t.Run("auto-provisions new user", func(t *testing.T) {
		ctx := context.Background()
		externalID := "betterauth-user-123"
		token := createTestBetterAuthJWT(t, externalID, "alice@test.com", "Alice", false)

		userID, err := interceptor.extractUserID(ctx, "Bearer "+token)
		require.NoError(t, err)
		assert.NotEqual(t, idwrap.IDWrap{}, userID)

		// Verify user was created in DB
		reader := suser.NewReader(db)
		user, err := reader.GetUserByExternalID(ctx, externalID)
		require.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "alice@test.com", user.Email)
		assert.Equal(t, &externalID, user.ExternalID)
	})

	t.Run("returns existing user on subsequent request", func(t *testing.T) {
		ctx := context.Background()
		externalID := "betterauth-user-456"
		token := createTestBetterAuthJWT(t, externalID, "bob@test.com", "Bob", false)

		// First request: auto-provisions
		userID1, err := interceptor.extractUserID(ctx, "Bearer "+token)
		require.NoError(t, err)

		// Second request: returns same user
		userID2, err := interceptor.extractUserID(ctx, "Bearer "+token)
		require.NoError(t, err)

		assert.Equal(t, userID1, userID2, "should return same user ID for same external_id")
	})

	t.Run("no token", func(t *testing.T) {
		_, err := interceptor.extractUserID(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no authentication token")
	})

	t.Run("invalid header format", func(t *testing.T) {
		_, err := interceptor.extractUserID(context.Background(), "NotBearer token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authorization header format")
	})

	t.Run("expired token", func(t *testing.T) {
		token := createTestBetterAuthJWT(t, "expired-user", "expired@test.com", "Expired", true)
		_, err := interceptor.extractUserID(context.Background(), "Bearer "+token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})

	t.Run("invalid signature", func(t *testing.T) {
		// Sign with a different secret
		claims := jwks.Claims{
			Email: "bad@test.com",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "bad-user",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte("wrong-secret"))
		require.NoError(t, err)

		_, err = interceptor.extractUserID(context.Background(), "Bearer "+tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired token")
	})

	t.Run("missing subject claim", func(t *testing.T) {
		token := createTestBetterAuthJWT(t, "", "nosub@test.com", "NoSub", false)
		_, err := interceptor.extractUserID(context.Background(), "Bearer "+token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing subject in token")
	})
}

func TestBetterAuthInterceptor_ConcurrentAutoProvision(t *testing.T) {
	us, db := setupTestUserService(t)
	interceptor := NewBetterAuthInterceptor(testHMACKeyfunc(), us)

	const goroutines = 10
	externalID := "race-condition-user"
	token := createTestBetterAuthJWT(t, externalID, "race@test.com", "Racer", false)

	var wg sync.WaitGroup
	results := make([]idwrap.IDWrap, goroutines)
	errs := make([]error, goroutines)

	// Launch all goroutines at once
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = interceptor.extractUserID(context.Background(), "Bearer "+token)
		}(i)
	}
	wg.Wait()

	// All must succeed
	for i := range goroutines {
		require.NoError(t, errs[i], "goroutine %d failed", i)
		assert.NotEqual(t, idwrap.IDWrap{}, results[i], "goroutine %d returned zero ID", i)
	}

	// All must return the same user ID
	for i := 1; i < goroutines; i++ {
		assert.Equal(t, results[0], results[i],
			"goroutine %d returned different ID than goroutine 0", i)
	}

	// Exactly one user should exist in the DB
	reader := suser.NewReader(db)
	user, err := reader.GetUserByExternalID(context.Background(), externalID)
	require.NoError(t, err)
	assert.Equal(t, results[0], user.ID)
	assert.Equal(t, "race@test.com", user.Email)
}

func TestBetterAuthInterceptor_ConcurrentDifferentUsers(t *testing.T) {
	us, _ := setupTestUserService(t)
	interceptor := NewBetterAuthInterceptor(testHMACKeyfunc(), us)

	const goroutines = 10

	var wg sync.WaitGroup
	results := make([]idwrap.IDWrap, goroutines)
	errs := make([]error, goroutines)

	// Each goroutine provisions a different user
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			externalID := fmt.Sprintf("concurrent-user-%d", idx)
			token := createTestBetterAuthJWT(t, externalID, fmt.Sprintf("user%d@test.com", idx), fmt.Sprintf("User %d", idx), false)
			results[idx], errs[idx] = interceptor.extractUserID(context.Background(), "Bearer "+token)
		}(i)
	}
	wg.Wait()

	// All must succeed
	for i := range goroutines {
		require.NoError(t, errs[i], "goroutine %d failed", i)
		assert.NotEqual(t, idwrap.IDWrap{}, results[i], "goroutine %d returned zero ID", i)
	}

	// All must return different user IDs
	seen := make(map[idwrap.IDWrap]int, goroutines)
	for i, id := range results {
		if prev, exists := seen[id]; exists {
			t.Errorf("goroutine %d and %d returned same ID %s", prev, i, id.String())
		}
		seen[id] = i
	}
}

func TestValidateJWT(t *testing.T) {
	kf := testHMACKeyfunc()

	t.Run("valid token", func(t *testing.T) {
		tokenString := createTestBetterAuthJWT(t, "user-id", "test@test.com", "Test User", false)
		claims, err := jwks.ValidateJWT(tokenString, kf)
		require.NoError(t, err)
		assert.Equal(t, "user-id", claims.Subject)
		assert.Equal(t, "test@test.com", claims.Email)
		assert.Equal(t, "Test User", claims.Name)
	})

	t.Run("expired token", func(t *testing.T) {
		tokenString := createTestBetterAuthJWT(t, "user-id", "test@test.com", "Test", true)
		_, err := jwks.ValidateJWT(tokenString, kf)
		assert.Error(t, err)
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := jwks.ValidateJWT("not.a.jwt", kf)
		assert.Error(t, err)
	})
}
