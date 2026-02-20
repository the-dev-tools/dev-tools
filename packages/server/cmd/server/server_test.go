package main

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
)

func TestParseAuthConfig_Default(t *testing.T) {
	cfg, err := parseAuthConfig()
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, AuthModeLocal, cfg.Mode)
	testutil.Assert(t, "", cfg.JWKSURL)
	testutil.Assert(t, true, cfg.IsLocal())
	testutil.Assert(t, false, cfg.IsBetterAuth())
}

func TestParseAuthConfig_ExplicitLocal(t *testing.T) {
	t.Setenv("AUTH_MODE", "local")
	cfg, err := parseAuthConfig()
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, AuthModeLocal, cfg.Mode)
	testutil.Assert(t, true, cfg.IsLocal())
}

func TestParseAuthConfig_BetterAuthWithJWKSURL(t *testing.T) {
	t.Setenv("AUTH_MODE", "betterauth")
	t.Setenv("JWKS_URL", "https://auth.example.com/.well-known/jwks.json")

	cfg, err := parseAuthConfig()
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, AuthModeBetterAuth, cfg.Mode)
	testutil.Assert(t, "https://auth.example.com/.well-known/jwks.json", cfg.JWKSURL)
	testutil.Assert(t, true, cfg.IsBetterAuth())
	testutil.Assert(t, false, cfg.IsLocal())
}

func TestParseAuthConfig_BetterAuthDeriveFromBetterAuthURL(t *testing.T) {
	t.Setenv("AUTH_MODE", "betterauth")
	t.Setenv("BETTERAUTH_URL", "https://auth.example.com")

	cfg, err := parseAuthConfig()
	testutil.AssertFatal(t, nil, err)
	testutil.Assert(t, AuthModeBetterAuth, cfg.Mode)
	testutil.Assert(t, "https://auth.example.com/api/auth/jwks", cfg.JWKSURL)
}

func TestParseAuthConfig_BetterAuthNoURL(t *testing.T) {
	t.Setenv("AUTH_MODE", "betterauth")

	_, err := parseAuthConfig()
	testutil.AssertNotFatal(t, nil, err)
}

func TestParseAuthConfig_InvalidMode(t *testing.T) {
	t.Setenv("AUTH_MODE", "invalid")

	_, err := parseAuthConfig()
	testutil.AssertNotFatal(t, nil, err)
}
