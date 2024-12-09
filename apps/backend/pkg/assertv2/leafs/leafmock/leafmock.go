package leafmock

import (
	"context"
	"errors"
	"the-dev-tools/backend/pkg/assertv2"
)

type LeafMock struct {
	DoFunc *func() (interface{}, error)
	Leafs  map[string]interface{}
}

// TODO: add tests
func (l *LeafMock) SelectGVal(ctx context.Context, k string) (interface{}, error) {
	if l.DoFunc != nil {
		(*l.DoFunc)()
	}

	leaf, ok := l.Leafs[k]
	if !ok {
		return assertv2.AssertLeafResponse{}, errors.New("key not found")
	}
	return leaf, nil
}
