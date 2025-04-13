package assertv2

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"unicode"

	"github.com/PaesslerAG/gval"
)

type AssertType int8

type AssertTargetType int16

const (
	AssertTypeUndefined AssertType = iota
	AssertTypeEqual
	AssertTypeNotEqual
	AssertTypeContains
	AssertTypeNotContains
	AssertTypeGreater
	AssertTypeLess
	AssertTypeGreaterOrEqual
	AssertTypeLessOrEqual
)

const (
	AssertTypeEqualStr          = "=="
	AssertTypeNotEqualStr       = "!="
	AssertTypeContainsStr       = "in"
	AssertTypeNotContainsStr    = "in"
	AssertTypeGreaterStr        = ">"
	AssertTypeLessStr           = "<"
	AssertTypeGreaterOrEqualStr = ">="
	AssertTypeLessOrEqualStr    = "<="
)

const (
	AssertTargetTypeUndefined AssertTargetType = iota
	AssertTargetTypeBody
	AssertTargetTypeHeader
	AssertTargetTypeResponse
	AssertTargetTypeQuery
	AssertTargetTypeNode
)

const (
	AssertPathResponse = "response"
	AssertPathBody     = "body"
	AssertPathHeader   = "header"
	AssertPathQuery    = "query"
	AssertPathNode     = "node"
)

// TODO: refactor this module
type AssertSystem struct {
	root *AssertRoot
}

func NewAssertSystem(root *AssertRoot) *AssertSystem {
	return &AssertSystem{
		root: root,
	}
}

type AssertRoot struct {
	Leaf interface{}
}

func NewAssertRoot(leaf interface{}) *AssertRoot {
	return &AssertRoot{
		Leaf: leaf,
	}
}

func NewAssertMapLeaf(leaf map[string]interface{}) AssertRoot {
	return AssertRoot{
		Leaf: leaf,
	}
}

type AssertLeaf interface {
	Get(ctx context.Context, k string) (AssertLeafResponse, error)
}

type AssertLeafResponse struct {
	Result *interface{}
	Leaf   *AssertLeaf
}

func NewAssertLeafResponse(result *interface{}, leaf *AssertLeaf) AssertLeafResponse {
	return AssertLeafResponse{
		Result: result,
		Leaf:   leaf,
	}
}

func NotinArray(a, b interface{}) (interface{}, error) {
	col, ok := b.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected type []interface{} for in operator but got %T", b)
	}
	for _, value := range col {
		if reflect.DeepEqual(a, value) {
			return false, nil
		}
	}
	return true, nil
}

func (s AssertSystem) EvalBool(ctx context.Context, expr string, extensions ...gval.Language) (bool, error) {
	ln := gval.Full(extensions...)
	eval, err := ln.NewEvaluable(expr)
	if err != nil {
		return false, err
	}
	return eval.EvalBool(ctx, s.root.Leaf)
}

var Regex *regexp.Regexp = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

var Langs []gval.Language = []gval.Language{
	gval.NewLanguage(
		gval.Init(func(ctx context.Context, p *gval.Parser) (gval.Evaluable, error) {
			p.SetIsIdentRuneFunc(func(r rune, pos int) bool {
				return unicode.IsLetter(r) || r == '_' || (pos > 0 && unicode.IsDigit(r)) || (pos > 0 && r == '-')
			})
			return p.ParseExpression(ctx)
		})),
	gval.InfixOperator("notin", NotinArray),
}

func ToSliceOfAny[T any](s []T) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

func (s AssertSystem) EvalArray(ctx context.Context, expr string, extensions ...gval.Language) ([]interface{}, error) {
	ln := gval.Full(extensions...)
	a, err := ln.EvaluateWithContext(ctx, expr, s.root.Leaf)
	if err != nil {
		return nil, err
	}

	// Check if a is slice
	arr, ok := a.([]interface{})
	if !ok {
		// Try to convert to []interface{}
		switch v := a.(type) {
		case []string:
			arr = make([]interface{}, len(v))
			for i, val := range v {
				arr[i] = val
			}
		case []int:
			arr = make([]interface{}, len(v))
			for i, val := range v {
				arr[i] = val
			}
		case []float64:
			arr = make([]interface{}, len(v))
			for i, val := range v {
				arr[i] = val
			}
		default:
			return nil, fmt.Errorf("expected type []interface{} but got %T", a)
		}
	}

	return arr, nil
}

func (s AssertSystem) EvalMap(ctx context.Context, expr string, extensions ...gval.Language) (map[string]interface{}, error) {
	ln := gval.Full(extensions...)
	a, err := ln.EvaluateWithContext(ctx, expr, s.root.Leaf)
	if err != nil {
		return nil, err
	}

	arr, ok := a.(map[string]interface{})
	if !ok {
		// Try to convert to []interface{}
		switch v := a.(type) {
		case map[string]string:
			arr = make(map[string]interface{}, len(v))
			for k, val := range v {
				arr[k] = val
			}
		case map[string]int:
			arr = make(map[string]interface{}, len(v))
			for k, val := range v {
				arr[k] = val
			}
		case map[string]float64:
			arr = make(map[string]interface{}, len(v))
			for k, val := range v {
				arr[k] = val
			}
		case map[string]float32:
			arr = make(map[string]interface{}, len(v))
			for k, val := range v {
				arr[k] = val
			}
		default:
			return nil, fmt.Errorf("expected type []interface{} but got %T", a)
		}
	}

	return arr, nil
}

func (s AssertSystem) AssertSimple(ctx context.Context, assertType AssertType, path string, value any) (bool, error) {
	// Regex should not contain any special characters
	// only dot (.) is allowed
	// TODO: change the regex to allow only dot (.), underscore (_), and hyphen (-)
	if !Regex.MatchString(path) {
		return false, fmt.Errorf("invalid path: %s", path)
	}

	assertTypeStr := ConvertAssertTypeToExpr(assertType)

	constLang := gval.Constant("y", value)
	tempLangs := append(Langs, constLang)

	expr := fmt.Sprintf("y %s %s", assertTypeStr, path)
	a, err := s.EvalBool(ctx, expr, tempLangs...)

	if assertType == AssertTypeNotContains {
		a = !a
	}
	return a, err
}

func (s AssertSystem) AssertComplex(ctx context.Context, expr string) (bool, error) {
	return s.EvalBool(ctx, expr)
}

func (s AssertSystem) AssertAny(ctx context.Context, expr string, extensions ...gval.Language) (any, error) {
	ln := gval.Full(extensions...)
	return ln.EvaluateWithContext(ctx, expr, s.root.Leaf)
}
