package expression

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// maxDescribedValue bounds one value in a description, so a long string doesn't flood the
// console.
const maxDescribedValue = 80

// sensitiveNameParts mark a path whose value must not be printed: assertion failures end up
// in CI logs.
var sensitiveNameParts = []string{"token", "secret", "password", "passwd", "authorization", "cookie", "apikey", "api_key", "api-key"}

// DescribeValues lists each variable path the expression reads with its current value, in
// the order the paths appear, e.g. `response.duration = 5120`. It reads without tracking.
// Objects and arrays are summarised, values under sensitive names are redacted, and long
// strings are truncated.
func (e *UnifiedEnv) DescribeValues(exprStr string) []string {
	if e == nil {
		return nil
	}
	paths := ExtractExprPaths(exprStr)
	sort.SliceStable(paths, func(i, j int) bool {
		pi, pj := strings.Index(exprStr, paths[i]), strings.Index(exprStr, paths[j])
		if pi != pj {
			return pi < pj
		}
		return paths[i] < paths[j]
	})
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		value, ok := ResolvePath(e.data, path)
		out = append(out, path+" = "+describeValue(path, value, ok))
	}
	return out
}

func describeValue(path string, value any, found bool) string {
	if !found {
		return "<missing>"
	}
	if isSensitivePath(path) && value != nil {
		return "<redacted>"
	}
	switch v := value.(type) {
	case nil:
		return "nil"
	case string:
		if runes := []rune(v); len(runes) > maxDescribedValue {
			return strconv.Quote(string(runes[:maxDescribedValue])) + "…"
		}
		return strconv.Quote(v)
	case json.Number:
		return v.String()
	case map[string]any:
		return fmt.Sprintf("{object with %d keys}", len(v))
	case map[string]string:
		return fmt.Sprintf("{object with %d keys}", len(v))
	case []any:
		return fmt.Sprintf("[array of %d items]", len(v))
	case bool, int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("<%T>", v)
	}
}

func isSensitivePath(path string) bool {
	leaf := strings.ToLower(path[strings.LastIndex(path, ".")+1:])
	for _, part := range sensitiveNameParts {
		if strings.Contains(leaf, part) {
			return true
		}
	}
	return false
}
