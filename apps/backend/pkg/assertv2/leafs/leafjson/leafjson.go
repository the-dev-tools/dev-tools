package leafjson

import "encoding/json"

type LeafJSON struct {
	jsonMap map[string]interface{}
}

// TODO: refactor this to not use json.Marshal
func NewWithStruct(a interface{}) (*LeafJSON, error) {
	jsonBytes, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(jsonBytes, &m)
	return New(m), nil
}

func New(input map[string]interface{}) *LeafJSON {
	return &LeafJSON{
		jsonMap: input,
	}
}
