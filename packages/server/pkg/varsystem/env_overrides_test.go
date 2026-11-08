package varsystem

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVarMapEnvOverrides(t *testing.T) {
	t.Setenv("VARSYSTEM_ENV_BOTH", "from-process")

	vars := map[string]any{"base": "#env:VARSYSTEM_ENV_ONLY_OVERRIDE"}
	WithEnvOverrides(vars, NewEnvOverrides(map[string]string{
		"VARSYSTEM_ENV_ONLY_OVERRIDE": "https://app.test",
		"VARSYSTEM_ENV_BOTH":          "from-override",
	}))
	vm := NewVarMapFromAnyMap(vars)

	got, err := vm.ReplaceVars("{{ #env:VARSYSTEM_ENV_ONLY_OVERRIDE }}/x {{ #env:VARSYSTEM_ENV_BOTH }}")
	require.NoError(t, err)
	require.Equal(t, "https://app.test/x from-override", got)

	// A variable whose value is a #env: reference resolves through the overrides too.
	got, err = vm.ReplaceVars("{{ base }}")
	require.NoError(t, err)
	require.Equal(t, "https://app.test", got)

	tracker := NewVarMapTracker(vm)
	got, err = tracker.ReplaceVars("{{ #env:VARSYSTEM_ENV_BOTH }}")
	require.NoError(t, err)
	require.Equal(t, "from-override", got)

	got, err = NewVarMap(nil).ReplaceVars("{{ #env:VARSYSTEM_ENV_BOTH }}")
	require.NoError(t, err)
	require.Equal(t, "from-process", got)
}

func TestEnvReferenceFallsBackToVariable(t *testing.T) {
	// Not set in the process: a Studio environment variable of the same name answers #env:.
	vars := map[string]any{"VARSYSTEM_ENV_FALLBACK_ONLY": "http://localhost:4567"}
	got, ok := LookupEnvVars("VARSYSTEM_ENV_FALLBACK_ONLY", vars)
	require.True(t, ok)
	require.Equal(t, "http://localhost:4567", got)

	got, err := NewVarMapFromAnyMap(vars).ReplaceVars("{{ #env:VARSYSTEM_ENV_FALLBACK_ONLY }}/api")
	require.NoError(t, err)
	require.Equal(t, "http://localhost:4567/api", got)

	// The process environment and run overrides still win over the variable.
	t.Setenv("VARSYSTEM_ENV_FALLBACK_BOTH", "from-process")
	vars = map[string]any{"VARSYSTEM_ENV_FALLBACK_BOTH": "from-variable"}
	got, _ = LookupEnvVars("VARSYSTEM_ENV_FALLBACK_BOTH", vars)
	require.Equal(t, "from-process", got)
	WithEnvOverrides(vars, NewEnvOverrides(map[string]string{"VARSYSTEM_ENV_FALLBACK_BOTH": "from-override"}))
	got, _ = LookupEnvVars("VARSYSTEM_ENV_FALLBACK_BOTH", vars)
	require.Equal(t, "from-override", got)

	// An empty variable is not a value.
	_, ok = LookupEnvVars("VARSYSTEM_ENV_FALLBACK_EMPTY", map[string]any{"VARSYSTEM_ENV_FALLBACK_EMPTY": ""})
	require.False(t, ok)
}
