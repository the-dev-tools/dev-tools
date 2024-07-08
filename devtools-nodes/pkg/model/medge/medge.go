package medge

import (
	"database/sql/driver"
	"encoding/json"
)

type Edges struct {
	InNodes  map[string]string `json:"inNode"`
	OutNodes map[string]string `json:"outNode"`
}

func (edge Edges) Value() (driver.Value, error) {
	byteArr, err := json.Marshal(edge)
	if err != nil {
		return nil, err
	}
	return byteArr, nil
}

func (edge *Edges) Scan(value interface{}) error {
	byteArr, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(byteArr, edge)
}
