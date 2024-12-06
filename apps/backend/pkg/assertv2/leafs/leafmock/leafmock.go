package leafmock

import (
	"context"
	"errors"
	"the-dev-tools/backend/pkg/assertv2"
)

type LeafMock struct {
	doFunc *func() (interface{}, error)
	leafs  map[string]assertv2.AssertLeaf
}

func NewLeafMock(doFunc *func() (interface{}, error), leafs map[string]assertv2.AssertLeaf) *LeafMock {
	return &LeafMock{
		doFunc: doFunc,
		leafs:  leafs,
	}
}

// TODO: add tests
func (l *LeafMock) Get(ctx context.Context, k string) (assertv2.AssertLeafResponse, error) {
	if l.doFunc != nil {
		(*l.doFunc)()
	}

	if l.leafs != nil {
		if leaf, ok := l.leafs[k]; ok {
			return leaf.Get(ctx, k)
		}
	}

	return assertv2.AssertLeafResponse{}, errors.New("key not found")
}
