package patch

// Optional represents a field that may or may not be set in a patch.
// Distinguishes between:
//   - Not set (not in patch) - zero value, set=false
//   - Set to nil (UNSET/clear the field) - value=nil, set=true
//   - Set to value (VALUE) - value=&T, set=true
type Optional[T any] struct {
	value *T
	set   bool
}

// NewOptional creates an Optional with a value
func NewOptional[T any](val T) Optional[T] {
	return Optional[T]{value: &val, set: true}
}

// NewOptionalPtr creates an Optional from a pointer
// If the pointer is nil, returns an explicitly unset Optional
func NewOptionalPtr[T any](val *T) Optional[T] {
	if val == nil {
		return Unset[T]()
	}
	return Optional[T]{value: val, set: true}
}

// Unset creates an explicitly unset Optional (set to nil)
func Unset[T any]() Optional[T] {
	return Optional[T]{value: nil, set: true}
}

// NotSet creates a not-set Optional (zero value, not in patch)
// This is the zero value of Optional[T], but provided as a named constructor for clarity
func NotSet[T any]() Optional[T] {
	return Optional[T]{value: nil, set: false}
}

// IsSet returns true if this field was explicitly set (even if to nil)
func (o Optional[T]) IsSet() bool {
	return o.set
}

// Value returns the pointer value (nil if UNSET)
func (o Optional[T]) Value() *T {
	return o.value
}

// IsUnset returns true if field was set but value is nil (UNSET semantics)
func (o Optional[T]) IsUnset() bool {
	return o.set && o.value == nil
}

// HasValue returns true if set and has non-nil value
func (o Optional[T]) HasValue() bool {
	return o.set && o.value != nil
}
