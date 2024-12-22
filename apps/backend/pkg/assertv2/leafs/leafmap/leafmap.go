package leafmap

import (
	"context"
	"errors"
	"the-dev-tools/backend/pkg/assertv2"
)

type LeafMock struct {
	Leafs map[string]interface{}
}

// TODO: add tests
func (l *LeafMock) SelectGVal(ctx context.Context, k string) (interface{}, error) {
	leaf, ok := l.Leafs[k]
	if !ok {
		return assertv2.AssertLeafResponse{}, errors.New("key not found")
	}
	return leaf, nil
}

func ConvertMapToLeafMap(a map[string]interface{}) *LeafMock {
	for k, v := range a {
		castedMap, ok := v.(map[string]interface{})
		if ok {
			a[k] = ConvertMapToLeafMap(castedMap)
		}
	}
	return &LeafMock{Leafs: a}
}
