package patch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOptional_NotSet tests the zero value of Optional (not set)
func TestOptional_NotSet(t *testing.T) {
	var opt Optional[string] // Zero value
	require.False(t, opt.IsSet())
	require.False(t, opt.IsUnset())
	require.False(t, opt.HasValue())
	require.Nil(t, opt.Value())
}

// TestOptional_Unset tests explicitly unset Optional
func TestOptional_Unset(t *testing.T) {
	opt := Unset[string]()
	require.True(t, opt.IsSet())
	require.True(t, opt.IsUnset())
	require.False(t, opt.HasValue())
	require.Nil(t, opt.Value())
}

// TestOptional_HasValue tests Optional with a value
func TestOptional_HasValue(t *testing.T) {
	opt := NewOptional("test")
	require.True(t, opt.IsSet())
	require.False(t, opt.IsUnset())
	require.True(t, opt.HasValue())
	require.Equal(t, "test", *opt.Value())
}

// TestOptional_NewOptionalPtr_Nil tests NewOptionalPtr with nil pointer
func TestOptional_NewOptionalPtr_Nil(t *testing.T) {
	opt := NewOptionalPtr[string](nil)
	require.True(t, opt.IsSet())
	require.True(t, opt.IsUnset())
	require.False(t, opt.HasValue())
	require.Nil(t, opt.Value())
}

// TestOptional_NewOptionalPtr_Value tests NewOptionalPtr with non-nil pointer
func TestOptional_NewOptionalPtr_Value(t *testing.T) {
	val := "test"
	opt := NewOptionalPtr(&val)
	require.True(t, opt.IsSet())
	require.False(t, opt.IsUnset())
	require.True(t, opt.HasValue())
	require.Equal(t, "test", *opt.Value())
}

// TestOptional_NotSet_Constructor tests NotSet constructor
func TestOptional_NotSet_Constructor(t *testing.T) {
	opt := NotSet[string]()
	require.False(t, opt.IsSet())
	require.False(t, opt.IsUnset())
	require.False(t, opt.HasValue())
	require.Nil(t, opt.Value())
}

// TestOptional_Bool tests Optional with bool type
func TestOptional_Bool(t *testing.T) {
	// Test with true
	optTrue := NewOptional(true)
	require.True(t, optTrue.IsSet())
	require.True(t, optTrue.HasValue())
	require.Equal(t, true, *optTrue.Value())

	// Test with false
	optFalse := NewOptional(false)
	require.True(t, optFalse.IsSet())
	require.True(t, optFalse.HasValue())
	require.Equal(t, false, *optFalse.Value())

	// Test unset
	optUnset := Unset[bool]()
	require.True(t, optUnset.IsSet())
	require.True(t, optUnset.IsUnset())
	require.Nil(t, optUnset.Value())
}

// TestOptional_Float32 tests Optional with float32 type
func TestOptional_Float32(t *testing.T) {
	opt := NewOptional(float32(1.5))
	require.True(t, opt.IsSet())
	require.True(t, opt.HasValue())
	require.Equal(t, float32(1.5), *opt.Value())
}

// TestOptional_Int tests Optional with int type
func TestOptional_Int(t *testing.T) {
	opt := NewOptional(42)
	require.True(t, opt.IsSet())
	require.True(t, opt.HasValue())
	require.Equal(t, 42, *opt.Value())
}

// TestOptional_EmptyString tests Optional with empty string (different from unset)
func TestOptional_EmptyString(t *testing.T) {
	opt := NewOptional("")
	require.True(t, opt.IsSet())
	require.True(t, opt.HasValue())
	require.False(t, opt.IsUnset())
	require.Equal(t, "", *opt.Value())
}

// TestOptional_ZeroValue tests Optional with zero value (different from unset)
func TestOptional_ZeroValue(t *testing.T) {
	optInt := NewOptional(0)
	require.True(t, optInt.IsSet())
	require.True(t, optInt.HasValue())
	require.False(t, optInt.IsUnset())
	require.Equal(t, 0, *optInt.Value())

	optBool := NewOptional(false)
	require.True(t, optBool.IsSet())
	require.True(t, optBool.HasValue())
	require.False(t, optBool.IsUnset())
	require.Equal(t, false, *optBool.Value())
}
