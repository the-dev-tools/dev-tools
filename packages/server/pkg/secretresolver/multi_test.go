package secretresolver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubResolver struct {
	value string
	err   error
}

func (s *stubResolver) ResolveSecret(_ context.Context, _, _, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.value, nil
}

func TestMultiResolver_Dispatch(t *testing.T) {
	multi := NewMultiResolver()
	multi.Register("gcp", &stubResolver{value: "gcp-secret"})
	multi.Register("aws", &stubResolver{value: "aws-secret"})

	ctx := context.Background()

	t.Run("dispatches to gcp", func(t *testing.T) {
		val, err := multi.ResolveSecret(ctx, "gcp", "ref", "")
		require.NoError(t, err)
		require.Equal(t, "gcp-secret", val)
	})

	t.Run("dispatches to aws", func(t *testing.T) {
		val, err := multi.ResolveSecret(ctx, "aws", "ref", "")
		require.NoError(t, err)
		require.Equal(t, "aws-secret", val)
	})

	t.Run("unsupported provider returns error", func(t *testing.T) {
		_, err := multi.ResolveSecret(ctx, "azure", "ref", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported secret provider")
	})

	t.Run("provider error is forwarded", func(t *testing.T) {
		multi2 := NewMultiResolver()
		multi2.Register("gcp", &stubResolver{err: fmt.Errorf("permission denied")})

		_, err := multi2.ResolveSecret(ctx, "gcp", "ref", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "permission denied")
	})
}
