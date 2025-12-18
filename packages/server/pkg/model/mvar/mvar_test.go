package mvar

import (
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/sort/sortenabled"

	"github.com/stretchr/testify/assert"
)

// TestIsEnabled tests the IsEnabled method implementation
func TestIsEnabled(t *testing.T) {
	t.Run("IsEnabled returns true for enabled variable", func(t *testing.T) {
		v := Var{
			ID:      idwrap.NewNow(),
			EnvID:   idwrap.NewNow(),
			VarKey:  "TEST_VAR",
			Value:   "test_value",
			Enabled: true,
		}

		assert.True(t, v.IsEnabled(), "Enabled variable should return true")
	})

	t.Run("IsEnabled returns false for disabled variable", func(t *testing.T) {
		v := Var{
			ID:      idwrap.NewNow(),
			EnvID:   idwrap.NewNow(),
			VarKey:  "TEST_VAR",
			Value:   "test_value",
			Enabled: false,
		}

		assert.False(t, v.IsEnabled(), "Disabled variable should return false")
	})
}

// TestSortEnabledFiltering tests that sortenabled package works with Var type
func TestSortEnabledFiltering(t *testing.T) {
	vars := []Var{
		{
			ID:      idwrap.NewNow(),
			EnvID:   idwrap.NewNow(),
			VarKey:  "ENABLED_1",
			Value:   "value1",
			Enabled: true,
		},
		{
			ID:      idwrap.NewNow(),
			EnvID:   idwrap.NewNow(),
			VarKey:  "DISABLED_1",
			Value:   "value2",
			Enabled: false,
		},
		{
			ID:      idwrap.NewNow(),
			EnvID:   idwrap.NewNow(),
			VarKey:  "ENABLED_2",
			Value:   "value3",
			Enabled: true,
		},
		{
			ID:      idwrap.NewNow(),
			EnvID:   idwrap.NewNow(),
			VarKey:  "DISABLED_2",
			Value:   "value4",
			Enabled: false,
		},
		{
			ID:      idwrap.NewNow(),
			EnvID:   idwrap.NewNow(),
			VarKey:  "ENABLED_3",
			Value:   "value5",
			Enabled: true,
		},
	}

	t.Run("Filter enabled variables", func(t *testing.T) {
		// Make a copy to avoid modifying original
		varsCopy := make([]Var, len(vars))
		copy(varsCopy, vars)

		// Get only enabled variables (modifies slice in place)
		sortenabled.GetAllWithState(&varsCopy, true)

		assert.Equal(t, 3, len(varsCopy), "Should have 3 enabled variables")

		// Verify all returned variables are enabled
		for _, v := range varsCopy {
			assert.True(t, v.IsEnabled(), "All filtered variables should be enabled")
			assert.Contains(t, []string{"ENABLED_1", "ENABLED_2", "ENABLED_3"}, v.VarKey,
				"Variable key should be one of the enabled ones")
		}
	})

	t.Run("Filter disabled variables", func(t *testing.T) {
		// Reset vars slice since GetAllWithState modifies it
		vars := []Var{
			{
				ID:      idwrap.NewNow(),
				EnvID:   idwrap.NewNow(),
				VarKey:  "ENABLED_1",
				Value:   "value1",
				Enabled: true,
			},
			{
				ID:      idwrap.NewNow(),
				EnvID:   idwrap.NewNow(),
				VarKey:  "DISABLED_1",
				Value:   "value2",
				Enabled: false,
			},
			{
				ID:      idwrap.NewNow(),
				EnvID:   idwrap.NewNow(),
				VarKey:  "ENABLED_2",
				Value:   "value3",
				Enabled: true,
			},
			{
				ID:      idwrap.NewNow(),
				EnvID:   idwrap.NewNow(),
				VarKey:  "DISABLED_2",
				Value:   "value4",
				Enabled: false,
			},
		}

		// Get only disabled variables (modifies slice in place)
		sortenabled.GetAllWithState(&vars, false)

		assert.Equal(t, 2, len(vars), "Should have 2 disabled variables")

		// Verify all returned variables are disabled
		for _, v := range vars {
			assert.False(t, v.IsEnabled(), "All filtered variables should be disabled")
			assert.Contains(t, []string{"DISABLED_1", "DISABLED_2"}, v.VarKey,
				"Variable key should be one of the disabled ones")
		}
	})

	t.Run("Empty slice returns empty", func(t *testing.T) {
		emptyVars := []Var{}
		sortenabled.GetAllWithState(&emptyVars, true)
		assert.Empty(t, emptyVars, "Empty slice should remain empty")
	})

	t.Run("All enabled returns all", func(t *testing.T) {
		allEnabled := []Var{
			{ID: idwrap.NewNow(), VarKey: "VAR1", Enabled: true},
			{ID: idwrap.NewNow(), VarKey: "VAR2", Enabled: true},
			{ID: idwrap.NewNow(), VarKey: "VAR3", Enabled: true},
		}

		sortenabled.GetAllWithState(&allEnabled, true)
		assert.Equal(t, 3, len(allEnabled), "Should keep all 3 enabled variables")
	})

	t.Run("All disabled returns none when filtering for enabled", func(t *testing.T) {
		allDisabled := []Var{
			{ID: idwrap.NewNow(), VarKey: "VAR1", Enabled: false},
			{ID: idwrap.NewNow(), VarKey: "VAR2", Enabled: false},
		}

		sortenabled.GetAllWithState(&allDisabled, true)
		assert.Empty(t, allDisabled, "Should have no variables when filtering for enabled")
	})
}
