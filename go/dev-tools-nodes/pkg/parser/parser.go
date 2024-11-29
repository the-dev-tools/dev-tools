package parser

import (
	"errors"

	"github.com/tidwall/gjson"
)

// val will be a pointer to a struct or primitive type
func ParseBytes(json []byte, path string) (*gjson.Result, error) {
	value := gjson.GetBytes(json, path)
	if !value.Exists() {
		return nil, errors.New("value does not exist")
	}
	return &value, nil
}
