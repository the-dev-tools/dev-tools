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

func (s AssertSystem) AssertSimple(ctx context.Context, assertType AssertType, path string, value interface{}) (bool, error) {
	// Regex should not contain any special characters
	// only dot (.) is allowed
	// TODO: change the regex to allow only dot (.), underscore (_), and hyphen (-)
	re := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	if !re.MatchString(path) {
		return false, fmt.Errorf("invalid path: %s", path)
	}

	assertTypeStr := ConvertAssertTypeToExpr(assertType)

	langs := []gval.Language{
		gval.NewLanguage(
			gval.Init(func(ctx context.Context, p *gval.Parser) (gval.Evaluable, error) {
				p.SetIsIdentRuneFunc(func(r rune, pos int) bool {
					return unicode.IsLetter(r) || r == '_' || (pos > 0 && unicode.IsDigit(r)) || (pos > 0 && r == '-')
				})
				return p.ParseExpression(ctx)
			})),
		gval.Constant("y", value),
		gval.InfixOperator("notin", NotinArray),
	}

	expr := fmt.Sprintf("y %s %s", assertTypeStr, path)
	fmt.Println("expr", expr)
	a, err := s.EvalBool(ctx, expr, langs...)

	if assertType == AssertTypeNotContains {
		a = !a
	}
	return a, err
}

func (s AssertSystem) AssertComplex(ctx context.Context, expr string) (bool, error) {
	return s.EvalBool(ctx, expr)
}
