// Package jwks provides JWKS (JSON Web Key Set) fetching and parsing utilities
// for validating JWT tokens signed with RSA keys.
package jwks

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims from BetterAuth tokens.
type Claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// ValidateJWT validates a JWT token using the given keyfunc and returns the claims.
func ValidateJWT(tokenString string, keyfunc jwt.Keyfunc) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, keyfunc)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// Response represents a JSON Web Key Set.
type Response struct {
	Keys []Key `json:"keys"`
}

// Key represents a single JSON Web Key.
type Key struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

var jwksHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

// FetchJWKS fetches and parses JWKS from the given URL, returning RSA public keys indexed by kid.
func FetchJWKS(url string) (map[string]*rsa.PublicKey, error) {
	resp, err := jwksHTTPClient.Get(url) //nolint:gosec // JWKS URL is configured by the operator
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Best-effort close on read-only response

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks Response
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	return ParseJWKS(jwks.Keys)
}

// ParseJWKS parses JWK keys into RSA public keys indexed by kid.
func ParseJWKS(keys []Key) (map[string]*rsa.PublicKey, error) {
	result := make(map[string]*rsa.PublicKey, len(keys))

	for _, key := range keys {
		if key.Kty != "RSA" {
			continue
		}

		pubKey, err := parseRSAPublicKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse key %s: %w", key.Kid, err)
		}

		result[key.Kid] = pubKey
	}

	if len(result) == 0 {
		return nil, errors.New("no RSA keys found in JWKS")
	}

	return result, nil
}

func parseRSAPublicKey(key Key) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// NewJWKSKeyfunc creates a jwt.Keyfunc that validates tokens using the given RSA public keys.
func NewJWKSKeyfunc(keys map[string]*rsa.PublicKey) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is RSA
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Look up key by kid
		kid, ok := token.Header["kid"].(string)
		if !ok {
			// If no kid, try the first key
			for _, key := range keys {
				return key, nil
			}
			return nil, errors.New("no kid in token header and no keys available")
		}

		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("key %s not found in JWKS", kid)
		}

		return key, nil
	}
}

// Provider fetches JWKS keys and refreshes them in the background.
type Provider struct {
	url            string
	keys           atomic.Pointer[map[string]*rsa.PublicKey]
	interval       time.Duration
	initialRetries int
}

// ProviderOption configures a Provider.
type ProviderOption func(*Provider)

// WithRefreshInterval sets the JWKS refresh interval (default 5 minutes).
func WithRefreshInterval(d time.Duration) ProviderOption {
	return func(p *Provider) {
		p.interval = d
	}
}

// WithInitialRetries configures retry attempts for the initial JWKS fetch (default 0, fail fast).
// Useful for development where services start concurrently.
func WithInitialRetries(attempts int) ProviderOption {
	return func(p *Provider) {
		p.initialRetries = attempts
	}
}

// NewProvider creates a Provider that fetches JWKS from the given URL.
// It performs an initial fetch and fails if the endpoint is unreachable after retries.
func NewProvider(url string, opts ...ProviderOption) (*Provider, error) {
	p := &Provider{
		url:      url,
		interval: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(p)
	}

	var keys map[string]*rsa.PublicKey
	var err error
	for attempt := range p.initialRetries + 1 {
		keys, err = FetchJWKS(url)
		if err == nil {
			break
		}
		if attempt < p.initialRetries {
			slog.Info("JWKS fetch failed, retrying...", "url", url, "attempt", attempt+1, "error", err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("initial JWKS fetch from %s: %w", url, err)
	}
	p.keys.Store(&keys)

	return p, nil
}

// Start launches a background goroutine that refreshes JWKS keys at the configured interval.
// The goroutine exits when ctx is cancelled.
func (p *Provider) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				keys, err := FetchJWKS(p.url)
				if err != nil {
					slog.Warn("JWKS refresh failed, keeping old keys", "url", p.url, "error", err)
					continue
				}
				p.keys.Store(&keys)
				slog.Debug("JWKS keys refreshed", "url", p.url, "key_count", len(keys))
			}
		}
	}()
}

// Keyfunc returns a jwt.Keyfunc that reads keys from the Provider's atomic pointer (lock-free).
func (p *Provider) Keyfunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		keysPtr := p.keys.Load()
		if keysPtr == nil {
			return nil, errors.New("JWKS keys not loaded")
		}
		return NewJWKSKeyfunc(*keysPtr)(token)
	}
}
