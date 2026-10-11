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
