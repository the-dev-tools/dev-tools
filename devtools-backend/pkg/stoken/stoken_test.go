package stoken_test

import (
	"devtools-backend/pkg/stoken"
	"testing"
	"time"
)

func TestNewJWT(t *testing.T) {
	someID := "someID"
	secret := []byte("someSecret")
	someEmail := "someEmail"

	jwtToken, err := stoken.NewJWT(someID, someEmail, stoken.AccessToken, time.Hour, secret)
	if err != nil {
		t.Fatalf("NewJWT() failed: %v", err)
	}

	token, err := stoken.ValidateJWT(jwtToken, stoken.AccessToken, secret)
	if err != nil {
		t.Fatalf("ValidateJWT() failed: %v", err)
	}

	if !token.Valid {
		t.Fatalf("Token is not valid")
	}
}

func TestFailNotValidSecretValidate(t *testing.T) {
	someID := "someID"
	secret := []byte("someSecret")
	wrongSecret := []byte("wrongSecret")
	someEmail := "someEmail"

	jwtToken, err := stoken.NewJWT(someID, someEmail, stoken.AccessToken, time.Hour, secret)
	if err != nil {
		t.Fatalf("NewJWT() failed: %v", err)
	}

	token, err := stoken.ValidateJWT(jwtToken, stoken.AccessToken, wrongSecret)
	if err == nil {
		t.Fatalf("ValidateJWT() should have failed")
	}

	if token != nil {
		t.Fatalf("Token should be nil")
	}
}

func TestFailNotValidTokenType(t *testing.T) {
	someID := "someID"
	secret := []byte("someSecret")
	someEmail := "someEmail"

	t.Run("FailNotValidTokenType AccessToken", func(t *testing.T) {
		jwtToken, err := stoken.NewJWT(someID, someEmail, stoken.AccessToken, time.Hour, secret)
		if err != nil {
			t.Fatalf("NewJWT() failed: %v", err)
		}

		_, err = stoken.ValidateJWT(jwtToken, stoken.RefreshToken, secret)
		if err == nil {
			t.Fatalf("ValidateJWT() didn't give error: %v", err)
		}
	})

	t.Run("FailNotValidTokenType RefreshToken", func(t *testing.T) {
		jwtToken, err := stoken.NewJWT(someID, someEmail, stoken.RefreshToken, time.Hour, secret)
		if err != nil {
			t.Fatalf("NewJWT() failed: %v", err)
		}

		_, err = stoken.ValidateJWT(jwtToken, stoken.AccessToken, secret)
		if err == nil {
			t.Fatalf("ValidateJWT() didn't give error: %v", err)
		}
	})
}
