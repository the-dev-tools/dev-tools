package stoken

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func NewJWT(id string, secret []byte) (string, error) {
	now := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  id,
		"iat": now.Unix(),
		"nbf": now.Add(24 * time.Hour).Unix(),
		"exp": now.Add(48 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
