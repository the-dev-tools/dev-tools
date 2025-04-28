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

func NewReferenceCompletionCreator() ReferenceCompletionCreator {
	return ReferenceCompletionCreator{
		PathMap: make(map[string]any, 0),
	}
}

func (c ReferenceCompletionCreator) Add(value any) {
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
		// endIndex: Where the query starts within the endToken
		endToken := matchedPath
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
			endToken = matchedPath[sepIndex+1:]
			// Adjust for array index token format "[N]" -> "N]" -> N
			if matchedPath[sepIndex] == '[' && strings.HasSuffix(endToken, "]") {
				endToken = endToken[:len(endToken)-1]
			}
		}

		// Find where the query part starts in the endToken.
		// This is a simplified approach; real-world might need fuzzy matching here too.
		// We assume the query is a prefix or contained within the last segment.
		endIndex := int32(strings.LastIndex(endToken, query)) // Find last occurrence in case of repetition
		if endIndex == -1 {
			// If query not directly in endToken (e.g., query="user", path="users[0]"),
			// maybe set index relative to the start of the token? Or 0?
			// For simplicity, let's default to 0 if not found.
			endIndex = 0
		} else {
			// endIndex should be the start position *within* the endToken
			// Example: path="data.userList", query="user", endToken="userList", endIndex=0
			// Example: path="data.userList", query="List", endToken="userList", endIndex=4
		}
		// A more robust endIndex calculation might involve comparing the query against
		// the *specific part* of the path that the fuzzy matcher considered the match.
		// fuzzyfinder.Rank doesn't directly provide this sub-match info.

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
	Environments *[]string
}
