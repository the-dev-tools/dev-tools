package referencecompletion

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"the-dev-tools/server/pkg/fuzzyfinder"
	"the-dev-tools/server/pkg/reference"
)

type ReferenceCompletionCreator struct {
	PathMap map[string]any
}

type ReferenceCompletionLookUp struct {
	LookUpMap map[string]any
}

func NewReferenceCompletionCreator() ReferenceCompletionCreator {
	return ReferenceCompletionCreator{
		PathMap: make(map[string]any, 0),
	}
}

func NewReferenceCompletionLookup() ReferenceCompletionLookUp {
	return ReferenceCompletionLookUp{
		LookUpMap: make(map[string]any, 0),
	}
}

func (c ReferenceCompletionCreator) Add(value any) {
	addPaths("", value, c.PathMap)
}

func (c ReferenceCompletionCreator) AddWithKey(value any, key string) {
	addPaths("", value, c.PathMap)
}

func addPaths(currentPath string, value any, pathMap map[string]any) {
	// Use reflection to inspect the value's type and structure.
	v := reflect.ValueOf(value)

	// Handle pointers: dereference them to get the actual value.
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return // Stop traversal if the pointer is nil.
		}
		v = v.Elem() // Get the value pointed to.
	}

	// Based on the kind of the value, decide how to proceed.
	switch v.Kind() {
	case reflect.Map:
		// Iterate through the key-value pairs of the map.
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()     // The map key.
			val := iter.Value() // The map value.

			// Convert the map key to a string representation.
			// Using fmt.Sprintf ensures broad compatibility but might not be ideal for all key types.
			keyStr := fmt.Sprintf("%v", k.Interface())

			// Construct the path for the map entry.
			var nextPath string
			if currentPath == "" {
				// If at the root, the path is just the key.
				nextPath = keyStr
			} else {
				// Otherwise, append the key with a dot separator.
				nextPath = currentPath + "." + keyStr
			}

			// Add the constructed path to the map. Store nil as we only need the path keys.
			pathMap[nextPath] = nil

			// Recursively call addPaths for the map value, but only if it's valid and potentially traversable.
			if val.IsValid() && val.CanInterface() {
				valInterface := val.Interface()
				valReflect := reflect.ValueOf(valInterface)
				// Dereference pointers again for the next level
				if valReflect.Kind() == reflect.Ptr {
					if valReflect.IsNil() {
						continue // Skip nil pointers in recursion
					}
					valReflect = valReflect.Elem()
				}
				// Recurse only for nested maps, slices, or arrays.
				switch valReflect.Kind() {
				case reflect.Map, reflect.Slice, reflect.Array:
					addPaths(nextPath, valInterface, pathMap)
				}
			}
		}
	case reflect.Slice, reflect.Array:
		// Iterate through the elements of the slice or array.
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i) // The element at index i.

			// Construct the path for the array/slice element using bracket notation.
			indexStr := strconv.Itoa(i)
			var nextPath string
			if currentPath == "" {
				// Path for a root-level array/slice element: "[index]"
				nextPath = "[" + indexStr + "]"
			} else {
				// Path for a nested array/slice element: "parent[index]"
				nextPath = currentPath + "[" + indexStr + "]"
			}

			// Add the constructed path to the map.
			pathMap[nextPath] = nil

			// Recursively call addPaths for the element, similar to map values.
			if elem.IsValid() && elem.CanInterface() {
				elemInterface := elem.Interface()
				elemReflect := reflect.ValueOf(elemInterface)
				// Dereference pointers again for the next level
				if elemReflect.Kind() == reflect.Ptr {
					if elemReflect.IsNil() {
						continue // Skip nil pointers in recursion
					}
					elemReflect = elemReflect.Elem()
				}
				// Recurse only for nested maps, slices, or arrays.
				switch elemReflect.Kind() {
				case reflect.Map, reflect.Slice, reflect.Array:
					addPaths(nextPath, elemInterface, pathMap)
				}
			}
		}
		// No default case needed: If the value is not a map, slice, or array,
		// we don't need to traverse further down. The path *leading* to this value
		// (if any) was already added by the caller.
	}
}

func (c ReferenceCompletionCreator) FindMatch(query string) []fuzzyfinder.Rank {
	// Return empty array for empty queries
	if query == "" {
		return []fuzzyfinder.Rank{}
	}

	keys := make([]string, 0, len(c.PathMap))
	for k := range c.PathMap {
		keys = append(keys, k)
	}

	ranks := fuzzyfinder.RankFind(keys, query)
	return ranks
}

func (c ReferenceCompletionCreator) FindMatchAndCalcCompletionData(query string) []ReferenceCompletionItem {
	ranks := c.FindMatch(query)

	referenceCompletionItems := make([]ReferenceCompletionItem, len(ranks))
	for i, rank := range ranks {
		matchedPath := rank.Target                               // The full path that matched, e.g., "data.users[0].name"
		pathKind := reference.ReferenceKind_REFERENCE_KIND_VALUE // Default kind

		// Attempt to get the actual kind stored during Add
		if kindVal, ok := c.PathMap[matchedPath]; ok {
			if storedKind, ok := kindVal.(reflect.Kind); ok {
				switch storedKind {
				case reflect.Map, reflect.Struct: // Treat Structs like Maps for completion
					pathKind = reference.ReferenceKind_REFERENCE_KIND_MAP
				case reflect.Slice, reflect.Array:
					pathKind = reference.ReferenceKind_REFERENCE_KIND_ARRAY
				default:
					pathKind = reference.ReferenceKind_REFERENCE_KIND_VALUE
				}
			}
		}

		// --- Calculate endToken and endIndex ---
		// endToken: The part of the path after the last separator ('.' or '[')
		// For autocomplete, we want only the suffix to be added after the query
		var fullEndToken string
		lastDot := strings.LastIndex(matchedPath, ".")
		lastBracket := strings.LastIndex(matchedPath, "[")
		sepIndex := -1
		if lastDot > lastBracket {
			sepIndex = lastDot
		} else if lastBracket > lastDot {
			// Need to handle the closing bracket as well for array indices
			closingBracket := strings.LastIndex(matchedPath, "]")
			if closingBracket > lastBracket {
				sepIndex = lastBracket // Use the opening bracket position
			} else {
				sepIndex = lastBracket // Fallback if no closing bracket found (shouldn't happen with valid paths)
			}
		}

		if sepIndex != -1 {
			fullEndToken = matchedPath[sepIndex+1:]
			// Adjust for array index token format "[N]" -> "N]" -> N
			if matchedPath[sepIndex] == '[' && strings.HasSuffix(fullEndToken, "]") {
				fullEndToken = fullEndToken[:len(fullEndToken)-1]
			}
		} else {
			fullEndToken = matchedPath
		}

		// Find where the query appears in the fullEndToken
		queryIndex := strings.Index(strings.ToLower(fullEndToken), strings.ToLower(query))
		endIndex := int32(0)
		endToken := ""

		if queryIndex != -1 {
			// The query exists in the end token
			// Set endToken to be only the part that should be completed (after the query)
			queryEndPos := queryIndex + len(query)
			if queryEndPos < len(fullEndToken) {
				endToken = fullEndToken[queryEndPos:]
			}
			// endIndex is 0 since we're appending directly after the query
		} else {
			// If query doesn't match directly, keep the old behavior
			// This is a fallback case - shouldn't happen with good fuzzy matching
			endToken = fullEndToken
			endIndex = 0
		}
		referenceCompletionItems[i] = ReferenceCompletionItem{
			Kind:     pathKind,
			EndToken: endToken,
			EndIndex: endIndex,
			// itemCount and environments would require storing more data in PathMap
			// or looking up the original structure, which isn't available here.
			ItemCount:    nil,
			Environments: nil,
		}
	}
	return referenceCompletionItems
}

type ReferenceCompletionItem struct {
	Kind reference.ReferenceKind

	/** End token of the string to be completed, i.e. 'body' in 'response.bo|dy' */
	EndToken string
	/** Index of the completion start in the end token, i.e. 2 in 'bo|dy' of 'response.bo|dy' */
	EndIndex int32
	/** Number of items when reference is a map or an array */
	ItemCount *int32
	/** Environment names when reference is a variable */
	Environments []string
}

func (c ReferenceCompletionLookUp) GetValue(path string) (any, error) {
	if path == "" {
		return c.LookUpMap[""], nil
	}

	// Direct lookup - check if we have the exact path in the map
	if value, exists := c.LookUpMap[path]; exists {
		return value, nil
	}

	// If not found directly, traverse the path step by step
	segments := parsePath(path)
	currentValue, exists := c.LookUpMap[""]
	if !exists {
		return nil, fmt.Errorf("no root data available in lookup map")
	}

	for _, segment := range segments {
		// Handle array/slice index access [n]
		if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
			indexStr := segment[1 : len(segment)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index in path: %s", segment)
			}

			v := reflect.ValueOf(currentValue)
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}

			if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
				return nil, fmt.Errorf("cannot use index on non-array/slice value at path segment: %s", segment)
			}

			if index < 0 || index >= v.Len() {
				return nil, fmt.Errorf("array index out of bounds at path segment: %s", segment)
			}

			currentValue = v.Index(index).Interface()
		} else {
			// Handle map/struct field access
			v := reflect.ValueOf(currentValue)
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}

			if v.Kind() == reflect.Map {
				mapKey := reflect.ValueOf(segment)
				value := v.MapIndex(mapKey)
				if !value.IsValid() {
					return nil, fmt.Errorf("key not found in map at path segment: %s", segment)
				}
				currentValue = value.Interface()
			} else {
				return nil, fmt.Errorf("cannot access property on non-map value at path segment: %s", segment)
			}
		}
	}

	return currentValue, nil
}

func (c ReferenceCompletionLookUp) Add(value any) {
	// Store the root value
	c.LookUpMap[""] = value

	// Add all paths from the value
	addPathsWithValues("", value, c.LookUpMap)
}

func (c ReferenceCompletionLookUp) AddWithKey(key string, value any) {
	// Store the value at the specified key
	c.LookUpMap[key] = value

	// Add all paths from this key
	addPathsWithValues(key, value, c.LookUpMap)
}

// addPathsWithValues is similar to addPaths but stores the actual values
func addPathsWithValues(currentPath string, value any, lookupMap map[string]any) {
	// Store the current value at its path
	lookupMap[currentPath] = value

	// Use reflection to inspect the value's type and structure
	v := reflect.ValueOf(value)

	// Handle pointers: dereference them to get the actual value
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return // Stop traversal if the pointer is nil
		}
		v = v.Elem() // Get the value pointed to
	}

	// Based on the kind of the value, decide how to proceed
	switch v.Kind() {
	case reflect.Map:
		// Iterate through the key-value pairs of the map
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()     // The map key
			val := iter.Value() // The map value

			// Convert the map key to a string representation
			keyStr := fmt.Sprintf("%v", k.Interface())

			// Construct the path for the map entry
			var nextPath string
			if currentPath == "" {
				// If at the root, the path is just the key
				nextPath = keyStr
			} else {
				// Otherwise, append the key with a dot separator
				nextPath = currentPath + "." + keyStr
			}

			// Recursively call addPathsWithValues for the map value
			if val.IsValid() && val.CanInterface() {
				valInterface := val.Interface()
				addPathsWithValues(nextPath, valInterface, lookupMap)
			}
		}
	case reflect.Slice, reflect.Array:
		// Iterate through the elements of the slice or array
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i) // The element at index i

			// Construct the path for the array/slice element using bracket notation
			indexStr := strconv.Itoa(i)
			var nextPath string
			if currentPath == "" {
				// Path for a root-level array/slice element: "[index]"
				nextPath = "[" + indexStr + "]"
			} else {
				// Path for a nested array/slice element: "parent[index]"
				nextPath = currentPath + "[" + indexStr + "]"
			}

			// Recursively call addPathsWithValues for the element
			if elem.IsValid() && elem.CanInterface() {
				elemInterface := elem.Interface()
				addPathsWithValues(nextPath, elemInterface, lookupMap)
			}
		}
	}
}

// parsePath splits a path string like "users[0].name" into segments ["users", "[0]", "name"]
func parsePath(path string) []string {
	var segments []string
	var current strings.Builder

	inBracket := false

	for _, char := range path {
		switch {
		case char == '.' && !inBracket:
			// If we encounter a dot and we're not inside brackets, end current segment
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		case char == '[' && !inBracket:
			// Start of an array index
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
			current.WriteRune(char)
			inBracket = true
		case char == ']' && inBracket:
			// End of an array index
			current.WriteRune(char)
			segments = append(segments, current.String())
			current.Reset()
			inBracket = false
		default:
			// Add character to current segment
			current.WriteRune(char)
		}
	}

	// Add the last segment if any
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments
}
