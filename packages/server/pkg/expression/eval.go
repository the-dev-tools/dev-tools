//nolint:revive // exported
package expression

import (
	"context"
	"fmt"
	"iter"
	"reflect"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// Eval evaluates a pure expr-lang expression and returns the result.
// This is the fast path for condition fields - NO {{ }} interpolation.
// Use Interpolate() for text fields that need {{ }} support.
func (e *UnifiedEnv) Eval(ctx context.Context, exprStr string) (any, error) {
	if e == nil {
		return nil, ErrNilEnv
	}

	// Build the environment for expr-lang
	env := e.buildExprEnv()

	// Compile and run the expression
	program, err := e.compileExpr(exprStr, compileModeAny, env)
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return nil, NewRunError(exprStr, err)
	}

	return output, nil
}

// EvalInterpolated first interpolates {{ }} patterns, then evaluates the result.
// Use this when you need both interpolation AND expression evaluation.
func (e *UnifiedEnv) EvalInterpolated(ctx context.Context, exprStr string) (any, error) {
	if e == nil {
		return nil, ErrNilEnv
	}

	// Fast path: skip interpolation if no {{ }} patterns
	interpolated := exprStr
	if HasVars(exprStr) {
		var err error
		interpolated, err = e.Interpolate(exprStr)
		if err != nil {
			return nil, err
		}

		// If the entire string was just a variable reference that got replaced,
		// and the result is not a valid expression, return the interpolated value
		if !looksLikeExpression(interpolated) {
			return interpolated, nil
		}
	}

	return e.Eval(ctx, interpolated)
}

// EvalBool evaluates a pure expr-lang expression and returns the result as a boolean.
// This is the fast path for condition fields (if node) - NO {{ }} interpolation.
func (e *UnifiedEnv) EvalBool(ctx context.Context, exprStr string) (bool, error) {
	if e == nil {
		return false, ErrNilEnv
	}

	// Build environment
	env := e.buildExprEnv()

	// Compile as boolean
	program, err := e.compileExpr(exprStr, compileModeBool, env)
	if err != nil {
		return false, err
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return false, NewRunError(exprStr, err)
	}

	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("expression did not evaluate to bool, got %T", output)
	}

	return result, nil
}

// EvalString evaluates an expression and returns the result as a string.
func (e *UnifiedEnv) EvalString(ctx context.Context, exprStr string) (string, error) {
	if e == nil {
		return "", ErrNilEnv
	}

	result, err := e.Eval(ctx, exprStr)
	if err != nil {
		return "", err
	}

	return anyToString(result), nil
}

// EvalIter evaluates a pure expr-lang expression and returns an iterator.
// Returns iter.Seq[any] for slices/arrays, or iter.Seq2[string, any] for maps.
// This is the fast path for loop fields (for/foreach node) - NO {{ }} interpolation.
func (e *UnifiedEnv) EvalIter(ctx context.Context, exprStr string) (any, error) {
	if e == nil {
		return nil, ErrNilEnv
	}

	// Build environment
	env := e.buildExprEnv()

	// Compile as any
	program, err := e.compileExpr(exprStr, compileModeAny, env)
	if err != nil {
		return nil, err
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return nil, NewRunError(exprStr, err)
	}

	// Handle nil and empty string cases
	if output == nil {
		return iter.Seq[any](func(func(any) bool) {}), nil
	}

	if str, ok := output.(string); ok {
		if strings.TrimSpace(str) == "" {
			return iter.Seq[any](func(func(any) bool) {}), nil
		}
	}

	// Convert to iterator based on type
	val := reflect.ValueOf(output)
	switch val.Kind() {
	case reflect.Map:
		if val.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings for iteration, got %s", val.Type().Key().Kind())
		}
		seq := func(yield func(string, any) bool) {
			for _, key := range val.MapKeys() {
				k := key.String()
				v := val.MapIndex(key).Interface()
				if !yield(k, v) {
					return
				}
			}
		}
		return iter.Seq2[string, any](seq), nil

	case reflect.Slice, reflect.Array:
		seq := func(yield func(any) bool) {
			for i := range val.Len() {
				item := val.Index(i).Interface()
				if !yield(item) {
					return
				}
			}
		}
		return iter.Seq[any](seq), nil

	default:
		return nil, fmt.Errorf("expected iterable (map or slice/array), got %T", output)
	}
}

// buildExprEnv creates the environment map for expr-lang evaluation.
// Includes the data, custom functions, and built-in helper functions.
//
// Data structure:
//   - env: environment/flow variables (access via env.apiKey or env["key.with.dots"])
//   - nodeName: node outputs (access via nodeName.response.body)
func (e *UnifiedEnv) buildExprEnv() map[string]any {
	env := make(map[string]any, len(e.data)+len(e.customFuncs)+10)

	// Copy data directly - no unflattening needed
	// Environment variables are namespaced under "env" key
	// Keys with dots can be accessed via bracket notation: env["key.with.dots"]
	for k, v := range e.data {
		env[k] = v
	}

	// Add custom functions
	for k, v := range e.customFuncs {
		env[k] = v
	}

	// Add built-in helper functions
	env["get"] = e.helperGet
	env["has"] = e.helperHas

	// Add built-in AI helper function (closure that captures 'e' for variable lookup)
	env["ai"] = e.helperAI

	return env
}

// helperGet is a helper function available in expressions for dynamic path lookup.
// Usage in expressions: get("dynamic.path")
func (e *UnifiedEnv) helperGet(path string) any {
	value, ok := e.Get(path)
	if !ok {
		return nil
	}
	return value
}

// helperHas is a helper function available in expressions for checking path existence.
// Usage in expressions: has("path.to.check")
func (e *UnifiedEnv) helperHas(path string) bool {
	return e.Has(path)
}

// compileExpr compiles an expression with caching support.
func (e *UnifiedEnv) compileExpr(exprStr string, mode compileMode, env map[string]any) (*vm.Program, error) {
	// Try cache first
	key := programCacheKey{expression: exprStr, mode: mode}
	if cached, ok := programCache.Load(key); ok {
		return cached.(*vm.Program), nil
	}

	// Compile options
	options := []expr.Option{expr.Env(env)}
	switch mode {
	case compileModeBool:
		options = append(options, expr.AsBool())
	default:
		options = append(options, expr.AsAny())
	}

	program, err := expr.Compile(exprStr, options...)
	if err != nil {
		return nil, NewCompileError(exprStr, err)
	}

	programCache.Store(key, program)
	return program, nil
}

// looksLikeExpression checks if a string looks like a valid expr-lang expression.
// Used to determine if interpolation result should be evaluated or returned as-is.
func looksLikeExpression(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// Check for obvious expression patterns
	for _, op := range []string{"==", "!=", ">=", "<=", ">", "<", "&&", "||", "+", "-", "*", "/", "%", "!", "(", "[", "."} {
		if strings.Contains(s, op) {
			return true
		}
	}

	// Check if it starts with keywords
	keywords := []string{"true", "false", "nil", "null", "not ", "and ", "or "}
	lower := strings.ToLower(s)
	for _, kw := range keywords {
		if strings.HasPrefix(lower, kw) || lower == strings.TrimSpace(kw) {
			return true
		}
	}

	// Check if it's a function call
	if strings.Contains(s, "(") && strings.Contains(s, ")") {
		return true
	}

	return false
}
