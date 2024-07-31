package stoken

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	AccessToken  TokenType = "access_token"
	RefreshToken TokenType = "refresh_token"
)

type DefaultClaims struct {
	jwt.RegisteredClaims
	TokenType TokenType `json:"type"`
}

func NewJWT(id string, tokenType TokenType, duration time.Duration, secret []byte) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, DefaultClaims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "devtools-backend",
			Subject:   "devtools-backend",
			Audience:  jwt.ClaimStrings{"devtools-backend"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        id,
		},
	})

	tokenString, err := t.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ValidateJWT(tokenString string, tokenType TokenType, secret []byte) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return secret, nil
	})

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claimType, ok := claims["type"].(TokenType)
	if !ok || claimType != tokenType {
		return nil, fmt.Errorf("invalid token type")
	}

	if err != nil {
		return nil, err
	}

	return token, nil
}
