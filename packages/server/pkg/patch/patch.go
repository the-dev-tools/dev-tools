package patch

// HTTPSearchParamPatch represents sparse updates to search parameter delta fields.
//
// Semantics:
//   - Field.IsSet() == false = field not changed (omitted from update)
//   - Field.IsUnset() == true = field explicitly UNSET/cleared
//   - Field.HasValue() == true = field set to that value
//
// Note: Order uses float32 for sync compatibility, though DB stores as float64.
type HTTPSearchParamPatch struct {
	Key         Optional[string]
	Value       Optional[string]
	Enabled     Optional[bool]
	Description Optional[string]
	Order       Optional[float32] // Must be float32 for sync converter
}

// HasChanges returns true if any field in the patch has been set
func (p HTTPSearchParamPatch) HasChanges() bool {
	return p.Key.IsSet() || p.Value.IsSet() || p.Enabled.IsSet() ||
		p.Description.IsSet() || p.Order.IsSet()
}

// HTTPHeaderPatch represents sparse updates to header delta fields
type HTTPHeaderPatch struct {
	Key         Optional[string]
	Value       Optional[string]
	Enabled     Optional[bool]
	Description Optional[string]
	Order       Optional[float32]
}

// HasChanges returns true if any field in the patch has been set
func (p HTTPHeaderPatch) HasChanges() bool {
	return p.Key.IsSet() || p.Value.IsSet() || p.Enabled.IsSet() ||
		p.Description.IsSet() || p.Order.IsSet()
}

// HTTPAssertPatch represents sparse updates to assert delta fields.
//
// Note: HTTPAssert does not have Key or Description fields, only Value.
type HTTPAssertPatch struct {
	Value   Optional[string]
	Enabled Optional[bool]
	Order   Optional[float32]
}

// HasChanges returns true if any field in the patch has been set
func (p HTTPAssertPatch) HasChanges() bool {
	return p.Value.IsSet() || p.Enabled.IsSet() || p.Order.IsSet()
}

// HTTPBodyRawPatch represents sparse updates to body raw delta fields.
//
// Note: Data is stored as []byte in DB but transmitted as string for JSON compatibility.
type HTTPBodyRawPatch struct {
	Data Optional[string]
}

// HasChanges returns true if any field in the patch has been set
func (p HTTPBodyRawPatch) HasChanges() bool {
	return p.Data.IsSet()
}

// HTTPBodyFormPatch represents sparse updates to body form delta fields
type HTTPBodyFormPatch struct {
	Key         Optional[string]
	Value       Optional[string]
	Enabled     Optional[bool]
	Description Optional[string]
	Order       Optional[float32]
}

// HasChanges returns true if any field in the patch has been set
func (p HTTPBodyFormPatch) HasChanges() bool {
	return p.Key.IsSet() || p.Value.IsSet() || p.Enabled.IsSet() ||
		p.Description.IsSet() || p.Order.IsSet()
}

// HTTPBodyUrlEncodedPatch represents sparse updates to body URL-encoded delta fields
type HTTPBodyUrlEncodedPatch struct {
	Key         Optional[string]
	Value       Optional[string]
	Enabled     Optional[bool]
	Description Optional[string]
	Order       Optional[float32]
}

// HasChanges returns true if any field in the patch has been set
func (p HTTPBodyUrlEncodedPatch) HasChanges() bool {
	return p.Key.IsSet() || p.Value.IsSet() || p.Enabled.IsSet() ||
		p.Description.IsSet() || p.Order.IsSet()
}

// HTTPDeltaPatch represents sparse updates to HTTP delta fields.
//
// Semantics:
//   - Field.IsSet() == false = field not changed (omitted from update)
//   - Field.IsUnset() == true = field explicitly UNSET/cleared
//   - Field.HasValue() == true = field set to that value
type HTTPDeltaPatch struct {
	Name   Optional[string]
	Method Optional[string]
	Url    Optional[string]
}

// HasChanges returns true if any field in the patch has been set
func (p HTTPDeltaPatch) HasChanges() bool {
	return p.Name.IsSet() || p.Method.IsSet() || p.Url.IsSet()
}
