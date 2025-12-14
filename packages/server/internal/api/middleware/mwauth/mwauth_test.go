package mwauth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/stoken"

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
			// If we expect no ID, GetContextUserID should return error
			if err == nil {
				// But some tests might not check for ID if it's not set.
				// However, GetContextUserID returns error if not found.
				// So if expectedID is empty, we probably expect it NOT to be there?
				// Actually, GetContextUserID returns error if key is missing.
			}
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
		// connect.NewRequest creates a request where Spec().IsClient is false by default?
		// Actually let's wrap it to be sure.
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
		// If IsClient is true, it should skip the defer recover logic.
		// If next panics, it should propagate (test crashes).
		// We use assert.Panics to verify it propagates.
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
