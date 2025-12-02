package delta

import (
	"sort"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

// ResolveHTTPInput holds the base and delta information required for resolution.
// This replaces the legacy MergeExamplesInput.
type ResolveHTTPInput struct {
	Base, Delta               mhttp.HTTP
	BaseQueries, DeltaQueries []mhttp.HTTPSearchParam
	BaseHeaders, DeltaHeaders []mhttp.HTTPHeader

	// Bodies
	BaseRawBody, DeltaRawBody               mhttp.HTTPBodyRaw
	BaseFormBody, DeltaFormBody             []mhttp.HTTPBodyForm
	BaseUrlEncodedBody, DeltaUrlEncodedBody []mhttp.HTTPBodyUrlencoded
	BaseAsserts, DeltaAsserts               []mhttp.HTTPAssert
}

// ResolveHTTPOutput holds the fully resolved HTTP request.
// This replaces the legacy MergeExamplesOutput.
type ResolveHTTPOutput struct {
	Resolved               mhttp.HTTP
	ResolvedQueries        []mhttp.HTTPSearchParam
	ResolvedHeaders        []mhttp.HTTPHeader
	ResolvedRawBody        mhttp.HTTPBodyRaw
	ResolvedFormBody       []mhttp.HTTPBodyForm
	ResolvedUrlEncodedBody []mhttp.HTTPBodyUrlencoded
	ResolvedAsserts        []mhttp.HTTPAssert
}

// ResolveHTTP merges a base request with a delta request, applying overrides
// based on the Delta System architecture (Overlay Pattern).
func ResolveHTTP(input ResolveHTTPInput) ResolveHTTPOutput {
	output := ResolveHTTPOutput{}

	// 1. Resolve Root HTTP Entity
	output.Resolved = resolveHTTPScalar(input.Base, input.Delta)

	// 2. Resolve Collections
	output.ResolvedQueries = resolveQueries(input.BaseQueries, input.DeltaQueries)
	output.ResolvedHeaders = resolveHeaders(input.BaseHeaders, input.DeltaHeaders)

	// 3. Resolve Body
	output.ResolvedRawBody = resolveRawBody(input.BaseRawBody, input.DeltaRawBody)
	output.ResolvedFormBody = resolveFormBody(input.BaseFormBody, input.DeltaFormBody)
	output.ResolvedUrlEncodedBody = resolveUrlEncodedBody(input.BaseUrlEncodedBody, input.DeltaUrlEncodedBody)

	// 4. Resolve Asserts
	// Asserts have specific ordering logic (Linked List)
	output.ResolvedAsserts = resolveAsserts(input.BaseAsserts, input.DeltaAsserts)

	return output
}

// resolveHTTPScalar applies delta scalar overrides to the base entity.
func resolveHTTPScalar(base, delta mhttp.HTTP) mhttp.HTTP {
	resolved := base

	// Explicitly set ID to Base ID (The "Identity" remains the Base)
	resolved.ID = base.ID
	resolved.IsDelta = false // The resolved object is a "Live" representation

	// Apply Overrides if Delta* fields are present (non-nil)
	if delta.DeltaName != nil {
		resolved.Name = *delta.DeltaName
	}
	if delta.DeltaUrl != nil {
		resolved.Url = *delta.DeltaUrl
	}
	if delta.DeltaMethod != nil {
		resolved.Method = *delta.DeltaMethod
	}
	if delta.DeltaDescription != nil {
		resolved.Description = *delta.DeltaDescription
	}
	if delta.DeltaBodyKind != nil {
		resolved.BodyKind = *delta.DeltaBodyKind
	}

	// Clear delta fields in the resolved object to avoid ambiguity
	resolved.DeltaName = nil
	resolved.DeltaUrl = nil
	resolved.DeltaMethod = nil
	resolved.DeltaDescription = nil
	resolved.DeltaBodyKind = nil

	return resolved
}

// resolveQueries resolves Search Params.
func resolveQueries(base []mhttp.HTTPSearchParam, delta []mhttp.HTTPSearchParam) []mhttp.HTTPSearchParam {
	// Map ParentID -> DeltaItem for overrides
	overrideMap := make(map[idwrap.IDWrap]mhttp.HTTPSearchParam)
	additions := make([]mhttp.HTTPSearchParam, 0)

	for _, d := range delta {
		if d.ParentHttpSearchParamID != nil {
			overrideMap[*d.ParentHttpSearchParamID] = d
		} else {
			additions = append(additions, d)
		}
	}

	resolved := make([]mhttp.HTTPSearchParam, 0, len(base)+len(additions))

	// Process Base items (preserving base order)
	for _, b := range base {
		if override, ok := overrideMap[b.ID]; ok {
			merged := b
			if override.DeltaKey != nil {
				merged.Key = *override.DeltaKey
			}
			if override.DeltaValue != nil {
				merged.Value = *override.DeltaValue
			}
			if override.DeltaDescription != nil {
				merged.Description = *override.DeltaDescription
			}
			if override.DeltaEnabled != nil {
				merged.Enabled = *override.DeltaEnabled
			}

			// Cleanup
			merged.IsDelta = false
			merged.ParentHttpSearchParamID = nil
			merged.DeltaKey = nil
			merged.DeltaValue = nil
			merged.DeltaDescription = nil
			merged.DeltaEnabled = nil

			resolved = append(resolved, merged)
		} else {
			resolved = append(resolved, b)
		}
	}

	// Append Additions
	for _, a := range additions {
		item := a
		item.IsDelta = false
		resolved = append(resolved, item)
	}

	return resolved
}

// resolveHeaders resolves HTTP Headers.
func resolveHeaders(base []mhttp.HTTPHeader, delta []mhttp.HTTPHeader) []mhttp.HTTPHeader {
	overrideMap := make(map[idwrap.IDWrap]mhttp.HTTPHeader)
	additions := make([]mhttp.HTTPHeader, 0)

	for _, d := range delta {
		if d.ParentHttpHeaderID != nil {
			overrideMap[*d.ParentHttpHeaderID] = d
		} else {
			additions = append(additions, d)
		}
	}

	resolved := make([]mhttp.HTTPHeader, 0, len(base)+len(additions))

	for _, b := range base {
		if override, ok := overrideMap[b.ID]; ok {
			merged := b
			if override.DeltaKey != nil {
				merged.Key = *override.DeltaKey
			}
			if override.DeltaValue != nil {
				merged.Value = *override.DeltaValue
			}
			if override.DeltaDescription != nil {
				merged.Description = *override.DeltaDescription
			}
			if override.DeltaEnabled != nil {
				merged.Enabled = *override.DeltaEnabled
			}

			merged.IsDelta = false
			merged.ParentHttpHeaderID = nil
			merged.DeltaKey = nil
			merged.DeltaValue = nil
			merged.DeltaDescription = nil
			merged.DeltaEnabled = nil

			resolved = append(resolved, merged)
		} else {
			resolved = append(resolved, b)
		}
	}

	for _, a := range additions {
		item := a
		item.IsDelta = false
		resolved = append(resolved, item)
	}

	return resolved
}

// resolveFormBody resolves Multipart Form Data.
func resolveFormBody(base []mhttp.HTTPBodyForm, delta []mhttp.HTTPBodyForm) []mhttp.HTTPBodyForm {
	overrideMap := make(map[idwrap.IDWrap]mhttp.HTTPBodyForm)
	additions := make([]mhttp.HTTPBodyForm, 0)

	for _, d := range delta {
		if d.ParentHttpBodyFormID != nil {
			overrideMap[*d.ParentHttpBodyFormID] = d
		} else {
			additions = append(additions, d)
		}
	}

	resolved := make([]mhttp.HTTPBodyForm, 0, len(base)+len(additions))

	for _, b := range base {
		if override, ok := overrideMap[b.ID]; ok {
			merged := b
			if override.DeltaKey != nil {
				merged.Key = *override.DeltaKey
			}
			if override.DeltaValue != nil {
				merged.Value = *override.DeltaValue
			}
			if override.DeltaDescription != nil {
				merged.Description = *override.DeltaDescription
			}
			if override.DeltaEnabled != nil {
				merged.Enabled = *override.DeltaEnabled
			}

			merged.IsDelta = false
			merged.ParentHttpBodyFormID = nil
			merged.DeltaKey = nil
			merged.DeltaValue = nil
			merged.DeltaDescription = nil
			merged.DeltaEnabled = nil

			resolved = append(resolved, merged)
		} else {
			resolved = append(resolved, b)
		}
	}

	for _, a := range additions {
		item := a
		item.IsDelta = false
		resolved = append(resolved, item)
	}

	return resolved
}

// resolveUrlEncodedBody resolves URL Encoded Body.
func resolveUrlEncodedBody(base []mhttp.HTTPBodyUrlencoded, delta []mhttp.HTTPBodyUrlencoded) []mhttp.HTTPBodyUrlencoded {
	overrideMap := make(map[idwrap.IDWrap]mhttp.HTTPBodyUrlencoded)
	additions := make([]mhttp.HTTPBodyUrlencoded, 0)

	for _, d := range delta {
		if d.ParentHttpBodyUrlEncodedID != nil {
			overrideMap[*d.ParentHttpBodyUrlEncodedID] = d
		} else {
			additions = append(additions, d)
		}
	}

	resolved := make([]mhttp.HTTPBodyUrlencoded, 0, len(base)+len(additions))

	for _, b := range base {
		if override, ok := overrideMap[b.ID]; ok {
			merged := b
			if override.DeltaKey != nil {
				merged.Key = *override.DeltaKey
			}
			if override.DeltaValue != nil {
				merged.Value = *override.DeltaValue
			}
			if override.DeltaDescription != nil {
				merged.Description = *override.DeltaDescription
			}
			if override.DeltaEnabled != nil {
				merged.Enabled = *override.DeltaEnabled
			}

			merged.IsDelta = false
			merged.ParentHttpBodyUrlEncodedID = nil
			merged.DeltaKey = nil
			merged.DeltaValue = nil
			merged.DeltaDescription = nil
			merged.DeltaEnabled = nil

			resolved = append(resolved, merged)
		} else {
			resolved = append(resolved, b)
		}
	}

	for _, a := range additions {
		item := a
		item.IsDelta = false
		resolved = append(resolved, item)
	}

	return resolved
}

// resolveRawBody resolves the Raw Body.
// Note: RawBody is singular, so we just overlay the Delta if present.
func resolveRawBody(base, delta mhttp.HTTPBodyRaw) mhttp.HTTPBodyRaw {
	resolved := base
	resolved.IsDelta = false
	resolved.ParentBodyRawID = nil

	// Check if Delta has data to override
	if len(delta.DeltaRawData) > 0 {
		resolved.RawData = delta.DeltaRawData
	} else if len(delta.RawData) > 0 {
		resolved.RawData = delta.RawData
	}

	if delta.DeltaContentType != nil {
		if v, ok := delta.DeltaContentType.(string); ok {
			resolved.ContentType = v
		}
	}

	if delta.DeltaCompressionType != nil {
		// Handle numeric types safely
		switch v := delta.DeltaCompressionType.(type) {
		case int8:
			resolved.CompressionType = v
		case int:
			if v >= -128 && v <= 127 {
				resolved.CompressionType = int8(v)
			}
		case float64:
			if v >= -128 && v <= 127 {
				resolved.CompressionType = int8(v)
			}
		}
	}

	// Cleanup
	resolved.DeltaRawData = nil
	resolved.DeltaContentType = nil
	resolved.DeltaCompressionType = nil

	return resolved
}

// resolveAsserts resolves Asserts using specific Linked List ordering logic.
func resolveAsserts(base, delta []mhttp.HTTPAssert) []mhttp.HTTPAssert {
	// 1. Order the inputs first to ensure we process them in the correct logical order
	orderedBase := orderAsserts(base)
	if len(delta) == 0 {
		return orderedBase
	}
	orderedDelta := orderAsserts(delta)

	// 2. Map Base items
	baseMap := make(map[idwrap.IDWrap]mhttp.HTTPAssert, len(orderedBase))
	baseOrder := make([]idwrap.IDWrap, 0, len(orderedBase))
	for _, assert := range orderedBase {
		baseMap[assert.ID] = assert
		baseOrder = append(baseOrder, assert.ID)
	}

	// 3. Process Deltas (Overrides and Additions)
	additions := make([]mhttp.HTTPAssert, 0)
	for _, d := range orderedDelta {
		if d.ParentHttpAssertID != nil {
			if b, exists := baseMap[*d.ParentHttpAssertID]; exists {
				// Apply Overrides
				merged := b
				if d.DeltaKey != nil {
					merged.Key = *d.DeltaKey
				}
				if d.DeltaValue != nil {
					merged.Value = *d.DeltaValue
				}
				if d.DeltaDescription != nil {
					merged.Description = *d.DeltaDescription
				}
				if d.DeltaEnabled != nil {
					merged.Enabled = *d.DeltaEnabled
				}

				merged.IsDelta = false
				merged.ParentHttpAssertID = nil
				merged.DeltaKey = nil
				merged.DeltaValue = nil
				merged.DeltaDescription = nil
				merged.DeltaEnabled = nil

				baseMap[*d.ParentHttpAssertID] = merged
			}
		} else {
			// New Addition
			item := d
			item.IsDelta = false
			additions = append(additions, item)
		}
	}

	// 4. Reconstruct the list
	merged := make([]mhttp.HTTPAssert, 0, len(baseMap)+len(additions))

	// Add base items (which may be merged/updated) in original order
	for _, id := range baseOrder {
		if assert, exists := baseMap[id]; exists {
			merged = append(merged, assert)
		}
	}

	// Append additions (ensure they are also ordered relative to each other if possible)
	if len(additions) > 0 {
		merged = append(merged, orderAsserts(additions)...)
	}

	return merged
}

// orderAsserts orders asserts by Order field.
func orderAsserts(asserts []mhttp.HTTPAssert) []mhttp.HTTPAssert {
	if len(asserts) <= 1 {
		return append([]mhttp.HTTPAssert(nil), asserts...)
	}

	// Create a copy and sort by Order field
	ordered := make([]mhttp.HTTPAssert, len(asserts))
	copy(ordered, asserts)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Order < ordered[j].Order
	})

	return ordered
}
