package parser_test

import (
	"testing"

	"github.com/DevToolsGit/devtools-nodes/pkg/parser"
	"github.com/tidwall/gjson"
)

func TestParser(t *testing.T) {
	var jsonStr string = `{"name":{"first":"Ege","last":"Tuzun"},"age":22}`
	jsonByteArr := []byte(jsonStr)

	res, err := parser.ParseBytes(jsonByteArr, "name.first")
	if err != nil {
		t.Errorf("Error parsing nested value: %s", err)
	}

	if res == nil {
		t.Errorf("Result is nil")
	}

	if !res.Exists() {
		t.Errorf("Result does not exist")
	}

	if res.Type != gjson.String {
		t.Errorf("Result is not a string")
	}

	val := res.String()

	if val != "Ege" {
		t.Errorf("Result is not Ege")
	}
}
