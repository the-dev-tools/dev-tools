package expression

import (
	"encoding/json"
	"math"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
)

// jsonNumberFunc is the hidden function every member access is wrapped in at compile time.
const jsonNumberFunc = "__jsonNumber"

// Response bodies are decoded with json.Decoder.UseNumber so ids keep their exact text when
// templated into later requests ({{ Login.response.body.id }}). json.Number is a string type,
// though, so without this "response.body.qty == 3" was false and "price > 100" a type error.
// Converting at the point of access keeps interpolation untouched and avoids copying the
// environment on every evaluation.

// jsonNumberPatcher wraps each member access (a.b, a[0]) in jsonNumberFunc.
type jsonNumberPatcher struct{}

func (jsonNumberPatcher) Visit(node *ast.Node) {
	if member, ok := (*node).(*ast.MemberNode); ok {
		ast.Patch(node, &ast.CallNode{
			Callee:    &ast.IdentifierNode{Value: jsonNumberFunc},
			Arguments: []ast.Node{member},
		})
	}
}

// jsonNumberToNumber returns an int when the number is integral and fits, a float64 otherwise,
// and any other value unchanged.
func jsonNumberToNumber(v any) any {
	n, ok := v.(json.Number)
	if !ok {
		return v
	}
	if i, err := n.Int64(); err == nil && i >= math.MinInt && i <= math.MaxInt {
		return int(i)
	}
	if f, err := n.Float64(); err == nil {
		return f
	}
	return v
}

// jsonNumberOptions are the compile options that make JSON numbers behave as numbers.
func jsonNumberOptions() []expr.Option {
	return []expr.Option{
		expr.Function(jsonNumberFunc, func(params ...any) (any, error) {
			return jsonNumberToNumber(params[0]), nil
		}),
		expr.Patch(jsonNumberPatcher{}),
	}
}
