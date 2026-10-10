package aicheck

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// StepRun is one step in one run, as the engine executed it.
type StepRun struct {
	// Ran: the engine executed it (Success or Failure).
	Ran        bool
	DurationMS *float64
	// Data is the node's output: {"request": {...}, "response": {...}}.
	Data map[string]any
}

// Run is one run of a flow: every step by name.
type Run struct {
	Index int
	Steps map[string]StepRun
}

// JSONShape turns a node's output into plain JSON values (float64 numbers, map[string]any),
// the shape a JSON report holds, so checks behave the same on engine values and on reports.
func JSONShape(v any) (map[string]any, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil, false
	}
	return m, m != nil
}

// pathTokens splits a.b[0].c into a, b, 0, c (array indexes as ints).
func pathTokens(path string) ([]any, error) {
	var out []any
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		name := part
		var idx []int
		if i := strings.Index(part, "["); i >= 0 {
			name = part[:i]
			rest := part[i:]
			for rest != "" {
				end := strings.Index(rest, "]")
				if !strings.HasPrefix(rest, "[") || end < 0 {
					return nil, fmt.Errorf("%q is not a path", path)
				}
				n, err := strconv.Atoi(rest[1:end])
				if err != nil {
					return nil, fmt.Errorf("%q is not a path", path)
				}
				idx = append(idx, n)
				rest = rest[end+1:]
			}
		}
		if name == "" && len(idx) == 0 {
			return nil, fmt.Errorf("%q is not a path", path)
		}
		if name != "" {
			out = append(out, name)
		}
		for _, n := range idx {
			out = append(out, n)
		}
	}
	return out, nil
}

// walk follows tokens from v. A JSON string met on the way (a request body) is parsed.
func walk(v any, tokens []any) (any, bool) {
	for _, t := range tokens {
		if s, ok := v.(string); ok {
			var parsed any
			if json.Unmarshal([]byte(s), &parsed) != nil {
				return nil, false
			}
			v = parsed
		}
		switch k := t.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil, false
			}
			if v, ok = m[k]; !ok {
				return nil, false
			}
		case int:
			a, ok := v.([]any)
			if !ok || k < 0 || k >= len(a) {
				return nil, false
			}
			v = a[k]
		}
	}
	return v, true
}

// Resolve reads a path in a run: response.… / request.… (the current step) or
// Step.response.… / Step.request.… (another step).
func Resolve(path, step string, run Run) (any, bool) {
	tokens, err := pathTokens(path)
	if err != nil || len(tokens) == 0 {
		return nil, false
	}
	first, _ := tokens[0].(string)
	name := step
	if first != "response" && first != "request" {
		name, tokens = first, tokens[1:]
	}
	sr, ok := run.Steps[name]
	if !ok || sr.Data == nil {
		return nil, false
	}
	return walk(sr.Data, tokens)
}

// Text renders a value: strings as they are, everything else as JSON.
func Text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

var templateRef = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)

// Render fills every {{ path }} of a template. missing is the first path that did not resolve.
func Render(tmpl, step string, run Run) (out string, missing string) {
	out = templateRef.ReplaceAllStringFunc(tmpl, func(m string) string {
		path := templateRef.FindStringSubmatch(m)[1]
		v, ok := Resolve(path, step, run)
		if !ok {
			if missing == "" {
				missing = path
			}
			return m
		}
		return Text(v)
	})
	return out, missing
}
