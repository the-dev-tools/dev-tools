package patch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHTTPSearchParamPatch_HasChanges tests the HasChanges method
func TestHTTPSearchParamPatch_HasChanges(t *testing.T) {
	// Empty patch should have no changes
	var patch HTTPSearchParamPatch
	require.False(t, patch.HasChanges())

	// Patch with one field set should have changes
	patch.Key = NewOptional("test")
	require.True(t, patch.HasChanges())

	// Patch with multiple fields set should have changes
	patch.Value = Unset[string]()
	require.True(t, patch.HasChanges())
}

// TestHTTPSearchParamPatch_OptionalFields tests all optional fields
func TestHTTPSearchParamPatch_OptionalFields(t *testing.T) {
	patch := HTTPSearchParamPatch{
		Key:         NewOptional("key1"),
		Value:       Unset[string](),
		Order:       NewOptional[float32](1.5),
		Description: NewOptional("desc"),
		Enabled:     NewOptional(true),
	}

	// Key field
	require.True(t, patch.Key.HasValue())
	require.Equal(t, "key1", *patch.Key.Value())

	// Value field (unset)
	require.True(t, patch.Value.IsUnset())
	require.Nil(t, patch.Value.Value())

	// Order field
	require.True(t, patch.Order.HasValue())
	require.Equal(t, float32(1.5), *patch.Order.Value())

	// Description field
	require.True(t, patch.Description.HasValue())
	require.Equal(t, "desc", *patch.Description.Value())

	// Enabled field
	require.True(t, patch.Enabled.HasValue())
	require.Equal(t, true, *patch.Enabled.Value())
}

// TestHTTPHeaderPatch_HasChanges tests the HasChanges method for header patch
func TestHTTPHeaderPatch_HasChanges(t *testing.T) {
	var patch HTTPHeaderPatch
	require.False(t, patch.HasChanges())

	patch.Enabled = NewOptional(false)
	require.True(t, patch.HasChanges())
}

// TestHTTPHeaderPatch_AllFieldsNotSet tests that zero value has all fields not set
func TestHTTPHeaderPatch_AllFieldsNotSet(t *testing.T) {
	var patch HTTPHeaderPatch

	require.False(t, patch.Key.IsSet())
	require.False(t, patch.Value.IsSet())
	require.False(t, patch.Enabled.IsSet())
	require.False(t, patch.Description.IsSet())
	require.False(t, patch.Order.IsSet())
}

// TestHTTPAssertPatch_HasChanges tests the HasChanges method for assert patch
func TestHTTPAssertPatch_HasChanges(t *testing.T) {
	var patch HTTPAssertPatch
	require.False(t, patch.HasChanges())

	patch.Value = NewOptional("assert value")
	require.True(t, patch.HasChanges())
}

// TestHTTPAssertPatch_ThreeFields tests that assert patch has only 3 fields
func TestHTTPAssertPatch_ThreeFields(t *testing.T) {
	patch := HTTPAssertPatch{
		Value:   NewOptional("value"),
		Enabled: Unset[bool](),
		Order:   NewOptional[float32](2.0),
	}

	require.True(t, patch.Value.HasValue())
	require.True(t, patch.Enabled.IsUnset())
	require.True(t, patch.Order.HasValue())
	require.True(t, patch.HasChanges())
}

// TestHTTPBodyRawPatch_HasChanges tests the HasChanges method for body raw patch
func TestHTTPBodyRawPatch_HasChanges(t *testing.T) {
	var patch HTTPBodyRawPatch
	require.False(t, patch.HasChanges())

	patch.Data = NewOptional("raw data")
	require.True(t, patch.HasChanges())
}

// TestHTTPBodyRawPatch_SingleField tests that body raw patch has only 1 field
func TestHTTPBodyRawPatch_SingleField(t *testing.T) {
	// Test with value
	patchWithValue := HTTPBodyRawPatch{
		Data: NewOptional("test data"),
	}
	require.True(t, patchWithValue.Data.HasValue())
	require.Equal(t, "test data", *patchWithValue.Data.Value())

	// Test with unset
	patchUnset := HTTPBodyRawPatch{
		Data: Unset[string](),
	}
	require.True(t, patchUnset.Data.IsUnset())
	require.Nil(t, patchUnset.Data.Value())

	// Test not set
	var patchNotSet HTTPBodyRawPatch
	require.False(t, patchNotSet.Data.IsSet())
}

// TestHTTPBodyFormPatch_HasChanges tests the HasChanges method for body form patch
func TestHTTPBodyFormPatch_HasChanges(t *testing.T) {
	var patch HTTPBodyFormPatch
	require.False(t, patch.HasChanges())

	patch.Key = NewOptional("form-key")
	require.True(t, patch.HasChanges())
}

// TestHTTPBodyFormPatch_OptionalFields tests all optional fields for body form
func TestHTTPBodyFormPatch_OptionalFields(t *testing.T) {
	patch := HTTPBodyFormPatch{
		Key:         NewOptional("key"),
		Value:       NewOptional("value"),
		Enabled:     NewOptional(true),
		Description: Unset[string](),
		Order:       NewOptional[float32](3.0),
	}

	require.True(t, patch.Key.HasValue())
	require.True(t, patch.Value.HasValue())
	require.True(t, patch.Enabled.HasValue())
	require.True(t, patch.Description.IsUnset())
	require.True(t, patch.Order.HasValue())
}

// TestHTTPBodyUrlEncodedPatch_HasChanges tests the HasChanges method for URL encoded patch
func TestHTTPBodyUrlEncodedPatch_HasChanges(t *testing.T) {
	var patch HTTPBodyUrlEncodedPatch
	require.False(t, patch.HasChanges())

	patch.Order = NewOptional[float32](1.0)
	require.True(t, patch.HasChanges())
}

// TestHTTPBodyUrlEncodedPatch_OptionalFields tests all optional fields for URL encoded
func TestHTTPBodyUrlEncodedPatch_OptionalFields(t *testing.T) {
	patch := HTTPBodyUrlEncodedPatch{
		Key:         Unset[string](),
		Value:       Unset[string](),
		Enabled:     NewOptional(false),
		Description: NewOptional("url encoded field"),
		Order:       Unset[float32](),
	}

	require.True(t, patch.Key.IsUnset())
	require.True(t, patch.Value.IsUnset())
	require.True(t, patch.Enabled.HasValue())
	require.False(t, *patch.Enabled.Value())
	require.True(t, patch.Description.HasValue())
	require.True(t, patch.Order.IsUnset())
}

// TestPatch_MixedStates tests a patch with mixed field states (not set, unset, has value)
func TestPatch_MixedStates(t *testing.T) {
	patch := HTTPSearchParamPatch{
		Key:   NewOptional("key"),     // Has value
		Value: Unset[string](),        // Explicitly unset
		// Enabled, Description, Order not set (zero value)
	}

	// Key is set with a value
	require.True(t, patch.Key.IsSet())
	require.True(t, patch.Key.HasValue())
	require.False(t, patch.Key.IsUnset())

	// Value is explicitly unset
	require.True(t, patch.Value.IsSet())
	require.False(t, patch.Value.HasValue())
	require.True(t, patch.Value.IsUnset())

	// Enabled is not set at all
	require.False(t, patch.Enabled.IsSet())
	require.False(t, patch.Enabled.HasValue())
	require.False(t, patch.Enabled.IsUnset())

	// HasChanges should return true (Key and Value are set)
	require.True(t, patch.HasChanges())
}
