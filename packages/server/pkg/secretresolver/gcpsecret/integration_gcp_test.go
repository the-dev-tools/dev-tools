//go:build gcp_integration

package gcpsecret

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGCPResolver_AccessSecret(t *testing.T) {
	if os.Getenv("RUN_GCP_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping GCP integration test: RUN_GCP_INTEGRATION_TESTS != true")
	}

	secretName := os.Getenv("GCP_TEST_SECRET_NAME")
	if secretName == "" {
		t.Skip("GCP_TEST_SECRET_NAME not set")
	}

	ctx := context.Background()

	resolver, err := New(ctx)
	require.NoError(t, err)
	defer resolver.Close()

	value, err := resolver.ResolveSecret(ctx, "gcp", secretName, "")
	require.NoError(t, err)
	require.NotEmpty(t, value, "expected non-empty secret value")
}

func TestGCPResolver_AccessSecretWithFragment(t *testing.T) {
	if os.Getenv("RUN_GCP_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping GCP integration test: RUN_GCP_INTEGRATION_TESTS != true")
	}

	secretName := os.Getenv("GCP_TEST_JSON_SECRET_NAME")
	if secretName == "" {
		t.Skip("GCP_TEST_JSON_SECRET_NAME not set")
	}

	fragmentKey := os.Getenv("GCP_TEST_FRAGMENT_KEY")
	if fragmentKey == "" {
		t.Skip("GCP_TEST_FRAGMENT_KEY not set")
	}

	ctx := context.Background()

	resolver, err := New(ctx)
	require.NoError(t, err)
	defer resolver.Close()

	value, err := resolver.ResolveSecret(ctx, "gcp", secretName, fragmentKey)
	require.NoError(t, err)
	require.NotEmpty(t, value, "expected non-empty fragment value")
}
