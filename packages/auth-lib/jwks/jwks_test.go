package jwks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKey(t *testing.T, kid string) (*rsa.PrivateKey, Key) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return priv, Key{
		Kty: "RSA",
		Kid: kid,
		N:   base64RawURL(priv.N.Bytes()),
		E:   base64RawURL(big.NewInt(int64(priv.E)).Bytes()),
		Alg: "RS256",
		Use: "sig",
	}
}

func base64RawURL(data []byte) string {
	const encodeURL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	buf := make([]byte, 0, (len(data)*8+5)/6)
	for i := 0; i < len(data); i += 3 {
		val := uint(data[i]) << 16
		if i+1 < len(data) {
			val |= uint(data[i+1]) << 8
		}
		if i+2 < len(data) {
			val |= uint(data[i+2])
		}

		remaining := len(data) - i
		switch {
		case remaining >= 3:
			buf = append(buf, encodeURL[(val>>18)&0x3F], encodeURL[(val>>12)&0x3F], encodeURL[(val>>6)&0x3F], encodeURL[val&0x3F])
		case remaining == 2:
			buf = append(buf, encodeURL[(val>>18)&0x3F], encodeURL[(val>>12)&0x3F], encodeURL[(val>>6)&0x3F])
		case remaining == 1:
			buf = append(buf, encodeURL[(val>>18)&0x3F], encodeURL[(val>>12)&0x3F])
		}
	}
	return string(buf)
}

func serveJWKS(t *testing.T, keys []Key) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response{Keys: keys})
	}))
}

func TestProvider_InitialFetch(t *testing.T) {
	priv, jwk := generateTestKey(t, "key-1")
	srv := serveJWKS(t, []Key{jwk})
	defer srv.Close()

	p, err := NewProvider(srv.URL)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	// Sign a token with the test key and validate
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &Claims{
		Email: "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	token.Header["kid"] = "key-1"
	tokenStr, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	claims, err := ValidateJWT(tokenStr, p.Keyfunc())
	if err != nil {
		t.Fatalf("ValidateJWT: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-1")
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "test@example.com")
	}
}

func TestProvider_InitialFetchFailure(t *testing.T) {
	_, err := NewProvider("http://127.0.0.1:1") // unreachable
	if err == nil {
		t.Fatal("expected error for unreachable JWKS URL")
	}
}

func TestProvider_BackgroundRefresh(t *testing.T) {
	// Start with key-1
	priv1, jwk1 := generateTestKey(t, "key-1")
	var mu sync.Mutex
	currentKeys := []Key{jwk1}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys := currentKeys
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response{Keys: keys})
	}))
	defer srv.Close()

	p, err := NewProvider(srv.URL, WithRefreshInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p.Start(ctx)

	// Token signed with key-1 should work
	tok1 := signTestToken(t, priv1, "key-1", "user-1")
	if _, err := ValidateJWT(tok1, p.Keyfunc()); err != nil {
		t.Fatalf("key-1 should validate: %v", err)
	}

	// Rotate: add key-2, remove key-1
	priv2, jwk2 := generateTestKey(t, "key-2")
	mu.Lock()
	currentKeys = []Key{jwk2}
	mu.Unlock()

	// Wait for refresh
	time.Sleep(150 * time.Millisecond)

	// Token signed with key-2 should work now
	tok2 := signTestToken(t, priv2, "key-2", "user-2")
	if _, err := ValidateJWT(tok2, p.Keyfunc()); err != nil {
		t.Fatalf("key-2 should validate after rotation: %v", err)
	}

	// Token signed with old key-1 should fail
	if _, err := ValidateJWT(tok1, p.Keyfunc()); err == nil {
		t.Fatal("key-1 should fail after rotation")
	}
}

func TestProvider_RefreshFailureKeepsOldKeys(t *testing.T) {
	priv, jwk := generateTestKey(t, "key-1")

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		if callCount.Load() > 1 {
			// Fail on refresh
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Response{Keys: []Key{jwk}})
	}))
	defer srv.Close()

	p, err := NewProvider(srv.URL, WithRefreshInterval(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p.Start(ctx)

	// Wait for a failed refresh
	time.Sleep(150 * time.Millisecond)

	// Old keys should still work
	tok := signTestToken(t, priv, "key-1", "user-1")
	if _, err := ValidateJWT(tok, p.Keyfunc()); err != nil {
		t.Fatalf("old keys should still work after refresh failure: %v", err)
	}
}

func TestProvider_ContextCancellation(t *testing.T) {
	_, jwk := generateTestKey(t, "key-1")
	srv := serveJWKS(t, []Key{jwk})
	defer srv.Close()

	p, err := NewProvider(srv.URL, WithRefreshInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	p.Start(ctx)

	// Cancel and verify goroutine exits (no panic/leak)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func signTestToken(t *testing.T, priv *rsa.PrivateKey, kid, sub string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	token.Header["kid"] = kid
	s, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}
