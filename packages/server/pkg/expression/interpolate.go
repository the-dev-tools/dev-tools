//nolint:revive // exported
package expression

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
)

// InterpolationResult holds the result of interpolation along with tracked reads.
type InterpolationResult struct {
	Value    string         // The interpolated string
	ReadVars map[string]any // Variables that were read during interpolation
}

// Interpolate replaces {{ varKey }} patterns with resolved values from the environment.
// Supports:
//   - {{ path.to.value }} - Nested path resolution
//   - {{ #env:VAR_NAME }} - Environment variables
//   - {{ #file:/path/to/file }} - File contents
//   - {{ items[0].id }} - Array access
//
// The context parameter is reserved for future use (cancellation, timeouts).
func (e *UnifiedEnv) Interpolate(raw string) (string, error) {
	result, err := e.InterpolateWithResult(raw)
	if err != nil {
		return "", err
	}
	return result.Value, nil
}

// InterpolateCtx is like Interpolate but accepts a context for cancellation.
func (e *UnifiedEnv) InterpolateCtx(ctx context.Context, raw string) (string, error) {
	// Check context before starting
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return e.Interpolate(raw)
}

// InterpolateWithResult replaces {{ varKey }} patterns and returns both the result
// and a map of all variables that were read during interpolation.
func (e *UnifiedEnv) InterpolateWithResult(raw string) (InterpolationResult, error) {
	if e == nil {
		return InterpolationResult{Value: raw, ReadVars: make(map[string]any)}, nil
	}

	readVars := make(map[string]any)
	var result strings.Builder
	remaining := raw

	for {
		startIndex := strings.Index(remaining, menv.Prefix)
		if startIndex == -1 {
			result.WriteString(remaining)
			break
		}

		endIndex := strings.Index(remaining[startIndex:], menv.Suffix)
		if endIndex == -1 {
			// No closing suffix, append rest and stop
			result.WriteString(remaining)
			break
		}

		// Write text before the variable
		result.WriteString(remaining[:startIndex])

		// Extract the variable reference (without braces)
		varRef := remaining[startIndex+menv.PrefixSize : startIndex+endIndex]
		varRef = strings.TrimSpace(varRef)

		// Resolve the variable/expression
		_, strVal, err := e.resolveVar(varRef, readVars)
		if err != nil {
			return InterpolationResult{}, &InterpolationError{
				Input:  raw,
				VarRef: varRef,
				Cause:  err,
			}
		}

		result.WriteString(strVal)

		// Move past this variable reference
		remaining = remaining[startIndex+endIndex+menv.SuffixSize:]
	}

	return InterpolationResult{
		Value:    result.String(),
		ReadVars: readVars,
	}, nil
}

// ResolveValue resolves an input that may contain {{ expr }} patterns.
// - If input is exactly "{{ expr }}", returns the typed value (bool, int, etc.)
// - If input has text around {{ }}, returns interpolated string
// - If input has no {{ }}, returns the input string as-is
func (e *UnifiedEnv) ResolveValue(raw string) (any, error) {
	if e == nil {
		return raw, nil
	}

	raw = strings.TrimSpace(raw)

	// Check if it's exactly "{{ expr }}" (single expression, no surrounding text)
	if strings.HasPrefix(raw, menv.Prefix) && strings.HasSuffix(raw, menv.Suffix) {
		// Check there's only one {{ }} pair
		inner := raw[menv.PrefixSize : len(raw)-menv.SuffixSize]
		if !strings.Contains(inner, menv.Prefix) && !strings.Contains(inner, menv.Suffix) {
			// Single expression - return typed value
			readVars := make(map[string]any)
			val, _, err := e.resolveVar(strings.TrimSpace(inner), readVars)
			return val, err
		}
	}

	// Multiple {{ }} or text around them - return interpolated string
	if HasVars(raw) {
		return e.Interpolate(raw)
	}

	// No {{ }} - return as-is
	return raw, nil
}

// resolveVar resolves a single variable/expression reference and tracks the read.
// Returns the resolved value (typed) and its string representation.
func (e *UnifiedEnv) resolveVar(varRef string, readVars map[string]any) (any, string, error) {
	switch {
	case isEnvReference(varRef):
		str, err := e.resolveEnvVar(varRef, readVars)
		return str, str, err
	case isFileReference(varRef):
		str, err := e.resolveFileVar(varRef, readVars)
		return str, str, err
	default:
		// Use expr-lang - supports paths AND expressions like now(), a > 5
		val, err := e.resolveExprVar(varRef, readVars)
		if err != nil {
			return nil, "", err
		}
		return val, anyToString(val), nil
	}
}

// resolveEnvVar resolves an environment variable reference (#env:VAR_NAME).
func (e *UnifiedEnv) resolveEnvVar(varRef string, readVars map[string]any) (string, error) {
	envName := strings.TrimPrefix(varRef, EnvRefPrefix)
	envName = strings.TrimSpace(envName)

	if envName == "" {
		return "", &EnvReferenceError{VarName: varRef, Cause: ErrEmptyPath}
	}

	value, ok := os.LookupEnv(envName)
	if !ok {
		return "", &EnvReferenceError{VarName: envName}
	}

	readVars[varRef] = value
	if e.tracker != nil {
		e.tracker.TrackRead(varRef, value)
	}

	return value, nil
}

// resolveFileVar resolves a file reference (#file:/path/to/file).
func (e *UnifiedEnv) resolveFileVar(varRef string, readVars map[string]any) (string, error) {
	filePath := strings.TrimPrefix(varRef, FileRefPrefix)
	filePath = strings.TrimSpace(filePath)

	if filePath == "" {
		return "", &FileReferenceError{Path: varRef, Cause: ErrEmptyPath}
	}

	data, err := os.ReadFile(filePath) //nolint:gosec // G304: Intentional file inclusion for #file: variable references
	if err != nil {
		return "", &FileReferenceError{Path: filePath, Cause: err}
	}

	value := string(data)
	readVars[varRef] = value
	if e.tracker != nil {
		e.tracker.TrackRead(varRef, value)
	}

	return value, nil
}

// resolveExprVar evaluates an expression using expr-lang.
// This supports both simple paths (a.b.c) AND expressions (now(), a > 5).
func (e *UnifiedEnv) resolveExprVar(varRef string, readVars map[string]any) (any, error) {
	// Build environment with data and custom functions
	env := e.buildExprEnv()

	// Compile and run with expr-lang
	program, err := e.compileExpr(varRef, compileModeAny, env)
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return nil, wrapExpressionError(varRef, expressionPhaseRun, err)
	}

	readVars[varRef] = output
	if e.tracker != nil {
		e.tracker.TrackRead(varRef, output)
	}

	return output, nil
}

// isEnvReference checks if a variable reference is an environment variable.
func isEnvReference(varRef string) bool {
	return strings.HasPrefix(strings.TrimSpace(varRef), EnvRefPrefix)
}

// isFileReference checks if a variable reference is a file reference.
func isFileReference(varRef string) bool {
	return strings.HasPrefix(strings.TrimSpace(varRef), FileRefPrefix)
}

// anyToString converts any value to its string representation.
func anyToString(v any) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		// Handle integers stored as float64 (common with JSON)
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// HasVars checks if a string contains any {{ }} variable references.
func HasVars(raw string) bool {
	return strings.Contains(raw, menv.Prefix) && strings.Contains(raw, menv.Suffix)
}

// ExtractVarRefs extracts all variable references from a string without resolving them.
// Returns a deduplicated list of variable references (including #env: and #file: refs).
func ExtractVarRefs(raw string) []string {
	if raw == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var result []string
	remaining := raw

	for {
		startIndex := strings.Index(remaining, menv.Prefix)
		if startIndex == -1 {
			break
		}

		endIndex := strings.Index(remaining[startIndex:], menv.Suffix)
		if endIndex == -1 {
			break
		}

		varRef := remaining[startIndex+menv.PrefixSize : startIndex+endIndex]
		varRef = strings.TrimSpace(varRef)

		if varRef != "" {
			if _, exists := seen[varRef]; !exists {
				seen[varRef] = struct{}{}
				result = append(result, varRef)
			}
		}

		remaining = remaining[startIndex+endIndex+menv.SuffixSize:]
	}

	return result
}
