package assertv2

import (
	"context"
	"errors"

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

type AssertSystem struct {
	root AssertRoot
}

func NewAssertSystem(root AssertRoot) *AssertSystem {
	return &AssertSystem{
		root: root,
	}
}

type AssertRoot struct {
	leafs map[string]AssertLeaf
}

func NewAssertRoot(leafs map[string]AssertLeaf) *AssertRoot {
	return &AssertRoot{
		leafs: make(map[string]AssertLeaf),
	}
}

type AssertLeaf interface {
	Get(ctx context.Context, k string) (AssertLeafResponse, error)
}

type AssertLeafResponse struct {
	Result *interface{}
	leafs  map[string]AssertLeaf
}

func (s AssertRoot) SelectGVal(ctx context.Context, k string) (interface{}, error) {
	leaf, ok := s.leafs[k]
	if !ok {
		return nil, errors.New("key not found")
	}
	if leaf == nil {
		return nil, errors.New("leaf not found")
	}

	result, err := leaf.Get(ctx, k)
	if err != nil {
		return nil, err
	}

	if result.leafs != nil {
		s.leafs = result.leafs
		return s, nil
	}

	return result.Result, nil
}

func (s AssertSystem) EvalBool(ctx context.Context, expr string) (bool, error) {
	ln := gval.Full()
	eval, err := ln.NewEvaluable(expr)
	if err != nil {
		return false, err
	}
	return eval.EvalBool(ctx, s.root)
}
