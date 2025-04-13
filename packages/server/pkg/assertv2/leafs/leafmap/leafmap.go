package leafmap

import (
	"context"
	"errors"
	"maps"
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

func ConvertMapToLeafMap(oldMap map[string]any) *LeafMap {
	newMap := make(map[string]any)
	maps.Copy(newMap, oldMap)

	for k, v := range newMap {
		castedMap, ok := v.(map[string]any)
		if ok {
			newMap[k] = ConvertMapToLeafMap(castedMap)
		}
	}
	return &LeafMap{Leafs: newMap}
}
