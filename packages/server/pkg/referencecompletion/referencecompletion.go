//nolint:revive // exported
package referencecompletion

import (
	"errors"
	"fmt"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/fuzzyfinder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/reference"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const ArrayStringValuePrefix = "Array"
const MapStringValuePrefix = "Map"

var numericSuffixRegex = regexp.MustCompile(`^(.+?)(\d+)$`)

// smartCompare compares two strings with smart numeric suffix handling
// First by length (shorter first), then alphabetically, then numerically for suffixes
func smartCompare(a, b string) bool {
	// First compare by length
	if len(a) != len(b) {
		return len(a) < len(b)
	}

	// If same length, check for numeric suffixes
	aMatches := numericSuffixRegex.FindStringSubmatch(a)
	bMatches := numericSuffixRegex.FindStringSubmatch(b)

	// If both have numeric suffixes and same prefix, compare numerically
	if len(aMatches) == 3 && len(bMatches) == 3 && aMatches[1] == bMatches[1] {
		aNum, aErr := strconv.Atoi(aMatches[2])
		bNum, bErr := strconv.Atoi(bMatches[2])
		if aErr == nil && bErr == nil {
			return aNum < bNum
		}
	}

	// Otherwise, fall back to alphabetical comparison
	return a < b
}

type ReferenceCompletionDetails struct {
	Count uint
}

type ReferenceCompletionCreator struct {
	PathMap map[string]ReferenceCompletionDetails
}

type ReferenceCompletionLookUp struct {
	LookUpMap map[string]string
}

func NewReferenceCompletionCreator() ReferenceCompletionCreator {
	return ReferenceCompletionCreator{
		PathMap: make(map[string]ReferenceCompletionDetails, 0),
	}
}

func NewReferenceCompletionLookup() ReferenceCompletionLookUp {
	return ReferenceCompletionLookUp{
		LookUpMap: make(map[string]string, 0),
	}
}

func (c ReferenceCompletionCreator) Add(value any) {
	addPaths("", value, c.PathMap)
}

func (c *ReferenceCompletionCreator) AddWithKey(key string, data any) {
	// Add nested paths prefixed with the key
	addPaths(key, data, c.PathMap)
}

func addPaths(currentPath string, value any, pathMap map[string]ReferenceCompletionDetails) {
	// Use reflection to inspect the value's type and structure.
	v := reflect.ValueOf(value)

	// Handle pointers: dereference them to get the actual value.
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return // Stop traversal if the pointer is nil.
		}
		v = v.Elem() // Get the value pointed to.
	}

	var count uint

	// Based on the kind of the value, decide how to proceed.
	switch v.Kind() {
	case reflect.Map:
		// Iterate through the key-value pairs of the map.
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()     // The map key.
			val := iter.Value() // The map value.

			// Convert the map key to a string representation.
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

			// Recursively process the map value
			addPaths(nextPath, val.Interface(), pathMap)
		}
		count = uint(v.Len()) // nolint:gosec // G115

	case reflect.Slice, reflect.Array:
		count = uint(v.Len()) // nolint:gosec // G115

		// Iterate through the elements of the slice or array.
		for i := range v.Len() {
			elem := v.Index(i) // The element at index i.

			// Construct the path for the array/slice element using bracket notation.
			// Path for a nested array/slice element: "parent[index]"
			nextPath := fmt.Sprintf("%s[%d]", currentPath, i)

			// Recursively process the array element
			addPaths(nextPath, elem.Interface(), pathMap)
		}
	}

	if currentPath != "" {
		// Store the details for the current path
		pathMap[currentPath] = ReferenceCompletionDetails{
			Count: count,
		}
	}
}

func (c ReferenceCompletionCreator) FindMatch(query string) []fuzzyfinder.Rank {
	// Return all paths for empty queries
	if query == "" {
		ranks := make([]fuzzyfinder.Rank, 0, len(c.PathMap))
		for path := range c.PathMap {
			ranks = append(ranks, fuzzyfinder.Rank{Target: path})
		}
		// Sort by length, then alphabetically, then numerically for suffixes
		sort.Slice(ranks, func(i, j int) bool {
			return smartCompare(ranks[i].Target, ranks[j].Target)
		})
		return ranks
	}

	// Check for exact matches first
	exactMatches := make(map[string]struct{})
	for path := range c.PathMap {
		if strings.EqualFold(path, query) {
			exactMatches[path] = struct{}{}
		}
	}

	// If we have exact matches, only return those
	if len(exactMatches) > 0 {
		ranks := make([]fuzzyfinder.Rank, 0, len(exactMatches))
		for match := range exactMatches {
			ranks = append(ranks, fuzzyfinder.Rank{Target: match})
		}
		return ranks
	}

	// Otherwise find prefix matches
	completions := make(map[string]struct{})
	for path := range c.PathMap {
		if strings.HasPrefix(strings.ToLower(path), strings.ToLower(query)) {
			completions[path] = struct{}{}
		}
	}

	// Convert completions to ranks
	ranks := make([]fuzzyfinder.Rank, 0, len(completions))
	for completion := range completions {
		ranks = append(ranks, fuzzyfinder.Rank{Target: completion})
	}
	// Sort by length, then alphabetically, then numerically for suffixes
	sort.Slice(ranks, func(i int, j int) bool {
		return smartCompare(ranks[i].Target, ranks[j].Target)
	})

	return ranks
}

func (c ReferenceCompletionCreator) FindMatchAndCalcCompletionData(query string) []ReferenceCompletionItem {
	ranks := c.FindMatch(query)

	referenceCompletionItems := make([]ReferenceCompletionItem, len(ranks))
	for i, rank := range ranks {
		matchedPath := rank.Target                               // The full path that matched
		pathKind := reference.ReferenceKind_REFERENCE_KIND_VALUE // Default kind

		// Determine if the path has children (it's a map)
		prefix := matchedPath + "."
		hasChildren := false
		for path := range c.PathMap {
			if strings.HasPrefix(path, prefix) {
				hasChildren = true
				break
			}
		}
		if hasChildren {
			pathKind = reference.ReferenceKind_REFERENCE_KIND_MAP
		}

		endIndex := len(query)

		Details := c.PathMap[matchedPath]
		itemCount := int32(Details.Count) // nolint:gosec // G115

		referenceCompletionItems[i] = ReferenceCompletionItem{
			Kind:         pathKind,
			EndToken:     matchedPath,
			EndIndex:     int32(endIndex), // nolint:gosec // G115
			ItemCount:    &itemCount,
			Environments: nil,
		}
	}
	return referenceCompletionItems
}

// parseQuerySegments splits a query into the resolved prefix (everything up to
// and including the last delimiter) and the partial segment (text being typed
// after the last delimiter). Delimiters are '.' and '['.
func parseQuerySegments(query string) (resolvedPrefix, partial string) {
	// Find the last '.' or '[' in the query
	lastDot := strings.LastIndex(query, ".")
	lastBracket := strings.LastIndex(query, "[")

	lastDelim := lastDot
	if lastBracket > lastDelim {
		lastDelim = lastBracket
	}

	if lastDelim < 0 {
		// No delimiter found — entire query is a partial at root level
		return "", query
	}

	// Include the delimiter in the prefix
	return query[:lastDelim+1], query[lastDelim+1:]
}

// FindNextLevel returns only the immediate next-level children matching the query.
// For "response." it returns ["response.body", "response.status", "response.headers"]
// rather than all descendants. This enables VS Code-style drill-down completion.
func (c ReferenceCompletionCreator) FindNextLevel(query string) []fuzzyfinder.Rank {
	resolvedPrefix, partial := parseQuerySegments(query)
	lowerPrefix := strings.ToLower(resolvedPrefix)
	lowerPartial := strings.ToLower(partial)

	candidates := make(map[string]struct{})

	for path := range c.PathMap {
		lowerPath := strings.ToLower(path)

		// Path must start with the resolved prefix (case-insensitive)
		if !strings.HasPrefix(lowerPath, lowerPrefix) {
			continue
		}

		// Use the actual path's prefix to preserve original casing
		actualPrefix := path[:len(resolvedPrefix)]

		// Extract the remainder after the prefix
		rest := path[len(actualPrefix):]
		if rest == "" {
			continue
		}

		// Find the next segment boundary (first '.' or '[' after position 0)
		boundary := len(rest)
		for i := range len(rest) {
			if rest[i] == '.' || rest[i] == '[' {
				if i == 0 {
					continue // skip leading delimiter
				}
				boundary = i
				break
			}
			// Include closing bracket ']' as part of the segment for array indices
			if rest[i] == ']' {
				boundary = i + 1 // include the ']'
				break
			}
		}

		nextSegment := rest[:boundary]
		lowerSegment := strings.ToLower(nextSegment)

		// Filter by partial match (case-insensitive)
		if !strings.HasPrefix(lowerSegment, lowerPartial) {
			continue
		}

		candidate := actualPrefix + nextSegment
		candidates[candidate] = struct{}{}
	}

	ranks := make([]fuzzyfinder.Rank, 0, len(candidates))
	for c := range candidates {
		ranks = append(ranks, fuzzyfinder.Rank{Target: c})
	}

	sort.Slice(ranks, func(i, j int) bool {
		return smartCompare(ranks[i].Target, ranks[j].Target)
	})

	return ranks
}

// FindNextLevelCompletionData returns completion items for the next level only,
// with proper Kind detection and EndIndex set to the start of the segment name.
func (c ReferenceCompletionCreator) FindNextLevelCompletionData(query string) []ReferenceCompletionItem {
	ranks := c.FindNextLevel(query)
	resolvedPrefix, _ := parseQuerySegments(query)

	items := make([]ReferenceCompletionItem, len(ranks))
	for i, rank := range ranks {
		matchedPath := rank.Target
		pathKind := reference.ReferenceKind_REFERENCE_KIND_VALUE

		// Check for children with '.' prefix (map children)
		dotPrefix := matchedPath + "."
		bracketPrefix := matchedPath + "["
		hasMapChildren := false
		hasArrayChildren := false

		for path := range c.PathMap {
			if strings.HasPrefix(path, dotPrefix) {
				hasMapChildren = true
			}
			if strings.HasPrefix(path, bracketPrefix) {
				hasArrayChildren = true
			}
			if hasMapChildren && hasArrayChildren {
				break
			}
		}

		if hasArrayChildren {
			pathKind = reference.ReferenceKind_REFERENCE_KIND_ARRAY
		} else if hasMapChildren {
			pathKind = reference.ReferenceKind_REFERENCE_KIND_MAP
		}

		details := c.PathMap[matchedPath]
		itemCount := int32(details.Count) // nolint:gosec // G115

		items[i] = ReferenceCompletionItem{
			Kind:      pathKind,
			EndToken:  matchedPath,
			EndIndex:  int32(len(resolvedPrefix)), // nolint:gosec // G115
			ItemCount: &itemCount,
		}
	}

	return items
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

func (c ReferenceCompletionLookUp) GetValue(path string) (string, error) {
	if path == "" {
		return c.LookUpMap[""], nil
	}

	// Direct lookup - check if we have the exact path in the map
	if value, exists := c.LookUpMap[path]; exists {
		return value, nil
	}

	return "", errors.New("not found")
}

func (c ReferenceCompletionLookUp) Add(value any) {
	// Store the root value
	c.LookUpMap[""] = fmt.Sprint(value)

	// Add all paths from the value
	addPathsWithValues("", value, c.LookUpMap)
}

func (c ReferenceCompletionLookUp) AddWithKey(key string, value any) {
	// Store the value at the specified key
	c.LookUpMap[key] = fmt.Sprint(value)

	// Add all paths from this key
	addPathsWithValues(key, value, c.LookUpMap)
}

// addPathsWithValues is similar to addPaths but stores the actual values
func addPathsWithValues(currentPath string, value any, lookupMap map[string]string) {
	var strValue string
	// Store the current value at its path

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
		// Format map as Map[key_type]value_type
		mapType := v.Type()
		strValue = fmt.Sprintf("%s[%s]%s", MapStringValuePrefix, mapType.Key(), mapType.Elem())

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
		// Format array/slice as Array[size]
		arrayType := v.Type()
		strValue = fmt.Sprintf("%s[%d]", arrayType.Elem(), v.Len())

		// Iterate through the elements of the slice or array
		for i := range v.Len() {
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
	default:
		if !v.IsValid() {
			strValue = ""
		} else {
			strValue = fmt.Sprint(v)
		}
	}
	lookupMap[currentPath] = strValue
}
