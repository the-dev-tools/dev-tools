package assertv2

import (
	"context"
	"fmt"
	"regexp"

	"github.com/PaesslerAG/gval"
)

type AssertType int8

type AssertTargetType int16

const (
	AssertTypeUndefined AssertType = iota
	AssertTypeEqual
	AssertTypeNotEqual
	AssertTypeContains
)

const (
	AssertTypeEqualStr    = "=="
	AssertTypeNotEqualStr = "!="
	AssertTypeContainsStr = "in"
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

func (s AssertSystem) EvalBool(ctx context.Context, expr string) (bool, error) {
	ln := gval.Full()
	eval, err := ln.NewEvaluable(expr)
	if err != nil {
		return false, err
	}
	return eval.EvalBool(ctx, s.root.Leaf)
}

func ConvertAssertTypeToExpr(assertType AssertType) string {
	switch assertType {
	case AssertTypeEqual:
		return AssertTypeEqualStr
	case AssertTypeNotEqual:
		return AssertTypeNotEqualStr
	case AssertTypeContains:
		return AssertTypeContainsStr
	default:
		return ""
	}
}

func (s AssertSystem) AssertSimple(ctx context.Context, assertType AssertType, path string, value interface{}) (bool, error) {
	// Regex should not contain any special characters
	// only dot (.) is allowed
	// TODO: change the regex to allow only dot (.) and underscore (_)
	re := regexp.MustCompile(`^[a-zA-Z0-9.]+$`)
	if !re.MatchString(path) {
		return false, fmt.Errorf("invalid path: %s", path)
	}

	assertTypeStr := ConvertAssertTypeToExpr(assertType)
	expr := fmt.Sprintf("%s %s %s", path, assertTypeStr, value)
	return s.EvalBool(ctx, expr)
}

func (s AssertSystem) AssertComplex(ctx context.Context, expr string) (bool, error) {
	return s.EvalBool(ctx, expr)
}
