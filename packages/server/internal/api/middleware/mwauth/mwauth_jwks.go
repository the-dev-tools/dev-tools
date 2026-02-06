package mwauth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// jwksResponse represents a JSON Web Key Set.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey represents a single JSON Web Key.
type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// FetchJWKS fetches and parses JWKS from the given URL, returning RSA public keys indexed by kid.
func FetchJWKS(url string) (map[string]*rsa.PublicKey, error) {
	resp, err := http.Get(url) //nolint:gosec // JWKS URL is configured by the operator
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	return parseJWKS(jwks.Keys)
}

// parseJWKS parses JWK keys into RSA public keys indexed by kid.
func parseJWKS(keys []jwkKey) (map[string]*rsa.PublicKey, error) {
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

func parseRSAPublicKey(key jwkKey) (*rsa.PublicKey, error) {
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
