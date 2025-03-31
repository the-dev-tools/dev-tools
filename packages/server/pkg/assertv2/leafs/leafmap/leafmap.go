package leafmap

import (
	"context"
	"errors"
	"the-dev-tools/server/pkg/assertv2"
)

type LeafMap struct {
	Leafs map[string]interface{}
}

// TODO: add tests
func (l *LeafMap) SelectGVal(ctx context.Context, k string) (interface{}, error) {
	leaf, ok := l.Leafs[k]
	if !ok {
		return assertv2.AssertLeafResponse{}, errors.New("key not found")
	}
	return leaf, nil
}

func ConvertMapToLeafMap(a map[string]interface{}) *LeafMap {
	for k, v := range a {
		castedMap, ok := v.(map[string]interface{})
		if ok {
			a[k] = ConvertMapToLeafMap(castedMap)
		}
	}
	return &LeafMap{Leafs: a}
}
