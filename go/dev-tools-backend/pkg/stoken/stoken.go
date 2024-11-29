package stoken

import (
	"dev-tools-backend/pkg/idwrap"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
)

type TokenType string

const (
	AccessToken  TokenType = "access_token"
	RefreshToken TokenType = "refresh_token"

	TokenHeaderKey string = "Authorization"
)

type DefaultClaims struct {
	jwt.RegisteredClaims
	TokenType TokenType `json:"token_type"`
	Email     string    `json:"email"`
}

func NewJWT(id idwrap.IDWrap, email string, tokenType TokenType, duration time.Duration, secret []byte) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, DefaultClaims{
		TokenType: tokenType,
		Email:     email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "devtools-backend",
			Subject:   id.String(),
			Audience:  jwt.ClaimStrings{"devtools-backend"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        ulid.Make().String(),
		},
	})

	tokenString, err := t.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func keyFunc(secret []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	}
}

func parseClaims(tokenString string, secret []byte) (*DefaultClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &DefaultClaims{}, keyFunc(secret))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return GetClaims(token)
}

func ValidateJWT(tokenString string, tokenType TokenType, secret []byte) (*DefaultClaims, error) {
	claims, err := parseClaims(tokenString, secret)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != tokenType {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}

func GetClaims(token *jwt.Token) (*DefaultClaims, error) {
	claims, ok := token.Claims.(*DefaultClaims)
	if !ok {
		return nil, fmt.Errorf("cannot cast claims")
	}
	return claims, nil
}
