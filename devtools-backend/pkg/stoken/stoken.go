package stoken

import (
	"github.com/golang-jwt/jwt/v5"
)

type DefaultClaims struct {
	jwt.RegisteredClaims
	CustomClaims string `json:"custom_claims"`
}

func NewJWT(id string, secret []byte) (string, error) {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, DefaultClaims{})

	tokenString, err := t.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func ValidateJWT(tokenString string, secret []byte) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if err != nil {
		return nil, err
	}

	return token, nil
}
