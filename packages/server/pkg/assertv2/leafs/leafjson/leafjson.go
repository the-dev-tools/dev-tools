package leafjson

import (
	"context"
	"encoding/json"
	"errors"
	"the-dev-tools/server/pkg/assertv2"
)

type LeafJSON struct {
	JsonMap map[string]interface{}
}

// TODO: refactor this to not use json.Marshal
func NewWithStruct(a interface{}) (*LeafJSON, error) {
	jsonBytes, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(jsonBytes, &m)
	if err != nil {
		return nil, err
	}

	return New(m), nil
}

func New(input map[string]interface{}) *LeafJSON {
	return &LeafJSON{
		JsonMap: input,
	}
}

// TODO: add tests
func (l *LeafJSON) SelectGVal(ctx context.Context, k string) (interface{}, error) {
	leaf, ok := l.JsonMap[k]
	if !ok {
		return assertv2.AssertLeafResponse{}, errors.New("key not found")
	}
	return leaf, nil
}
