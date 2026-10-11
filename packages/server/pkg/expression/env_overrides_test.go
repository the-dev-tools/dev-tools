package expression

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"
)

func TestInterpolateEnvOverrides(t *testing.T) {
	t.Setenv("STUDIO_ENV_BOTH", "from-process")

	vars := map[string]any{"name": "flow"}
	varsystem.WithEnvOverrides(vars, varsystem.NewEnvOverrides(map[string]string{
		"STUDIO_ENV_ONLY_OVERRIDE": "https://app.test",
		"STUDIO_ENV_BOTH":          "from-override",
	}))
	env := NewUnifiedEnv(vars)

	got, err := env.Interpolate("{{ #env:STUDIO_ENV_ONLY_OVERRIDE }}/api")
	require.NoError(t, err)
	require.Equal(t, "https://app.test/api", got)

	got, err = env.Interpolate("{{ #env:STUDIO_ENV_BOTH }}")
	require.NoError(t, err)
	require.Equal(t, "from-override", got, "overrides win over the process environment")

	// A clone of the run's variables (as nodes make) keeps the overrides.
	got, err = env.Clone().Interpolate("{{ #env:STUDIO_ENV_ONLY_OVERRIDE }}")
	require.NoError(t, err)
	require.Equal(t, "https://app.test", got)

	// Without overrides the process environment is used, and a missing one fails.
	plain := NewUnifiedEnv(map[string]any{})
	got, err = plain.Interpolate("{{ #env:STUDIO_ENV_BOTH }}")
	require.NoError(t, err)
	require.Equal(t, "from-process", got)
	_, err = plain.Interpolate("{{ #env:STUDIO_ENV_ONLY_OVERRIDE }}")
	require.Error(t, err)
}

func TestEnvOverridesStayOutOfVariables(t *testing.T) {
	vars := map[string]any{}
	varsystem.WithEnvOverrides(vars, varsystem.NewEnvOverrides(map[string]string{"SECRET": "s3cr3t"}))

	data, err := json.Marshal(vars)
	require.NoError(t, err)
	require.NotContains(t, string(data), "s3cr3t")
	require.NotContains(t, fmt.Sprint(vars), "s3cr3t")

	// Expressions cannot name the key.
	_, err = NewUnifiedEnv(vars).Interpolate("{{ SECRET }}")
	require.Error(t, err)
}
