package stoken_test

import (
	"devtools-backend/pkg/stoken"
	"testing"
)

func TestNewJWT(t *testing.T) {
	someID := "someID"
	secret := []byte("someSecret")

	jwtToken, err := stoken.NewJWT(someID, secret)
	if err != nil {
		t.Fatalf("NewJWT() failed: %v", err)
	}

	token, err := stoken.ValidateJWT(jwtToken, secret)
	if err != nil {
		t.Fatalf("ValidateJWT() failed: %v", err)
	}

	if !token.Valid {
		t.Fatalf("Token is not valid")
	}
}

func TestFailValidate(t *testing.T) {
	someID := "someID"
	secret := []byte("someSecret")
	wrongSecret := []byte("wrongSecret")

	jwtToken, err := stoken.NewJWT(someID, secret)
	if err != nil {
		t.Fatalf("NewJWT() failed: %v", err)
	}

	token, err := stoken.ValidateJWT(jwtToken, wrongSecret)
	if err == nil {
		t.Fatalf("ValidateJWT() should have failed")
	}

	if token != nil {
		t.Fatalf("Token should be nil")
	}
}
