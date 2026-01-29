//nolint:revive // exported
package expression

import (
	"context"
	"fmt"
	"iter"
	"reflect"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/parser"
	"github.com/expr-lang/expr/vm"
)

// Eval evaluates a pure expr-lang expression and returns the result.
// This is the fast path for condition fields - NO {{ }} interpolation.
// Use Interpolate() for text fields that need {{ }} support.
func (e *UnifiedEnv) Eval(ctx context.Context, exprStr string) (any, error) {
	if e == nil {
		return nil, ErrNilEnv
	}

	// Track variable reads before evaluation (for expr-lang expressions)
	e.trackExprReads(exprStr)

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

	// Track variable reads before evaluation (for expr-lang expressions)
	e.trackExprReads(exprStr)

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

	// Track variable reads before evaluation (for expr-lang expressions)
	e.trackExprReads(exprStr)

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

// trackExprReads extracts variable paths from an expr-lang expression and calls Get()
// on each path to trigger tracking. This ensures that pure expr-lang expressions
// (like "node.response.status == 200") have their variable reads tracked.
func (e *UnifiedEnv) trackExprReads(exprStr string) {
	if e.tracker == nil {
		return
	}

	paths := ExtractExprPaths(exprStr)
	for _, path := range paths {
		// Get() calls tracker.TrackRead() if the value exists
		e.Get(path)
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

// ExtractExprIdentifiers extracts top-level identifiers from a pure expr-lang expression.
// It performs a simple lexical scan to find variable references without full AST parsing.
// Returns identifiers like "node", "env" from expressions like "node.response.status == 200".
func ExtractExprIdentifiers(exprStr string) []string {
	if exprStr == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var result []string

	// Simple lexical scan for identifiers
	// Identifiers start with letter/underscore, followed by alphanumerics/underscores
	i := 0
	for i < len(exprStr) {
		// Skip non-identifier characters
		if !isIdentStart(exprStr[i]) {
			i++
			continue
		}

		// Found start of identifier
		start := i
		for i < len(exprStr) && isIdentChar(exprStr[i]) {
			i++
		}
		ident := exprStr[start:i]

		// Skip keywords and built-in functions
		if isKeyword(ident) {
			continue
		}

		// Add unique identifiers
		if _, exists := seen[ident]; !exists {
			seen[ident] = struct{}{}
			result = append(result, ident)
		}
	}

	return result
}

// isIdentStart returns true if c can start an identifier (letter or underscore).
func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// isIdentChar returns true if c can be part of an identifier.
func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// isKeyword returns true if s is a reserved keyword or built-in function.
func isKeyword(s string) bool {
	keywords := map[string]bool{
		// Boolean literals
		"true": true, "false": true, "nil": true, "null": true,
		// Logical operators
		"and": true, "or": true, "not": true, "in": true,
		// Built-in functions
		"len": true, "all": true, "any": true, "one": true, "none": true,
		"map": true, "filter": true, "find": true, "findIndex": true,
		"count": true, "sum": true, "mean": true, "min": true, "max": true,
		"first": true, "last": true, "take": true, "keys": true, "values": true,
		"sort": true, "sortBy": true, "groupBy": true, "reduce": true,
		"abs": true, "ceil": true, "floor": true, "round": true,
		"int": true, "float": true, "string": true, "toJSON": true, "fromJSON": true,
		"trim": true, "trimPrefix": true, "trimSuffix": true,
		"upper": true, "lower": true, "split": true, "replace": true,
		"contains": true, "startsWith": true, "endsWith": true,
		"now": true, "date": true, "duration": true,
		// Custom helper functions
		"get": true, "has": true, "ai": true,
	}
	return keywords[s]
}

// ExtractExprPaths extracts full dot-notation paths from expr-lang expressions.
// Uses AST parsing for robust extraction.
// Example: "node.response.status == 200" → ["node.response.status"]
func ExtractExprPaths(exprStr string) []string {
	if exprStr == "" {
		return nil
	}

	tree, err := parser.Parse(exprStr)
	if err != nil {
		return nil
	}

	extractor := &pathExtractor{
		paths: make(map[string]struct{}),
		bases: make(map[ast.Node]struct{}),
	}

	// First pass: collect all nodes that are bases of MemberNodes
	ast.Walk(&tree.Node, &baseCollector{bases: extractor.bases})

	// Second pass: extract paths only from "root" nodes (not bases of other MemberNodes)
	ast.Walk(&tree.Node, extractor)

	return extractor.result()
}

// baseCollector collects nodes that are used as bases in MemberNodes (property/bracket access).
type baseCollector struct {
	bases map[ast.Node]struct{}
}

// Visit marks nodes that are bases of MemberNodes.
func (b *baseCollector) Visit(node *ast.Node) {
	if node == nil {
		return
	}
	if mn, ok := (*node).(*ast.MemberNode); ok {
		// Mark the base node as being part of a longer chain
		// But only if this MemberNode represents a property access (not array indexing)
		// Array indexing uses IntegerNode as Property, which breaks the path chain
		if _, isArrayIndex := mn.Property.(*ast.IntegerNode); !isArrayIndex {
			b.bases[mn.Node] = struct{}{}
		}
	}
}

// pathExtractor implements ast.Visitor to extract variable paths from AST nodes.
type pathExtractor struct {
	paths map[string]struct{}
	bases map[ast.Node]struct{} // nodes that are bases of MemberNodes (should be skipped)
}

// Visit is called for each node in the AST during traversal.
func (p *pathExtractor) Visit(node *ast.Node) {
	if node == nil {
		return
	}

	// Skip nodes that are bases of other MemberNodes (they're part of a longer chain)
	if _, isBase := p.bases[*node]; isBase {
		return
	}

	// For MemberNodes with array indexing, extract the array base path
	// This handles cases like items[0].id where we want to track "items"
	if mn, ok := (*node).(*ast.MemberNode); ok {
		if _, isArrayIndex := mn.Property.(*ast.IntegerNode); isArrayIndex {
			if path := p.buildPath(mn.Node); path != "" {
				topLevel := strings.Split(path, ".")[0]
				if !isKeyword(topLevel) {
					p.paths[path] = struct{}{}
				}
			}
			return // Don't process further - array indexing breaks the path chain
		}
	}

	// Build full path from MemberNode chain or IdentifierNode
	if path := p.buildPath(*node); path != "" {
		// Skip keywords and built-in functions
		topLevel := strings.Split(path, ".")[0]
		if !isKeyword(topLevel) {
			p.paths[path] = struct{}{}
		}
	}
}

// buildPath recursively builds a dot-notation path from AST nodes.
// Returns the longest static path that can be built.
// Array indexing (items[0]) terminates path building by returning "" since we can't
// statically determine what's beyond the index.
func (p *pathExtractor) buildPath(node ast.Node) string {
	switch n := node.(type) {
	case *ast.IdentifierNode:
		return n.Value
	case *ast.MemberNode:
		// Check if this is array indexing - if so, return "" to break chain
		// The array base path is extracted separately in Visit()
		if _, isArrayIndex := n.Property.(*ast.IntegerNode); isArrayIndex {
			return ""
		}

		base := p.buildPath(n.Node)
		if base == "" {
			return ""
		}
		// Handle property access (e.g., node.response or node["key"])
		switch prop := n.Property.(type) {
		case *ast.StringNode:
			return base + "." + prop.Value
		case *ast.IdentifierNode:
			return base + "." + prop.Value
		}
		// Dynamic property access (e.g., node[someVar]) - can't extract static path
		return ""
	}
	return ""
}

// result returns the collected paths as a slice.
func (p *pathExtractor) result() []string {
	result := make([]string, 0, len(p.paths))
	for path := range p.paths {
		result = append(result, path)
	}
	return result
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
