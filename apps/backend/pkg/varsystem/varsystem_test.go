package varsystem_test

import (
	"fmt"
	"testing"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mvar"
	"the-dev-tools/backend/pkg/varsystem"
)

func TestMergeVars(t *testing.T) {
	// TestMergeVars tests the mergeVars function
	// when the input is a slice of variables with no duplicates
	a := []mvar.Var{}
	const aSize = 10

	for i := 0; i < aSize; i++ {
		a = append(a, mvar.Var{
			ID:     idwrap.NewNow(),
			VarKey: fmt.Sprintf("key_%d", i),
			EnvID:  idwrap.NewNow(),
			Value:  fmt.Sprintf("value_%d", i),
		})
	}

	b := []mvar.Var{}
	const bNonDupe = 10
	const bSize = bNonDupe + aSize

	for i := aSize; i < bSize; i++ {
		b = append(b, mvar.Var{
			ID:     idwrap.NewNow(),
			VarKey: fmt.Sprintf("key_%d", i),
			EnvID:  idwrap.NewNow(),
			Value:  fmt.Sprintf("value_%d", i),
		})
	}

	c := varsystem.MergeVars(a, b)
	const expectedSize = aSize + bNonDupe
	if len(c) != expectedSize {
		t.Errorf("Expected size of %d, got %d", expectedSize, len(c))
	}
}

func TestGetVars(t *testing.T) {
	const key1 = "key1"
	const value1 = "value1"

	vs := varsystem.NewVarMap([]mvar.Var{
		{ID: idwrap.NewNow(), VarKey: key1, EnvID: idwrap.NewNow(), Value: value1},
	})

	t.Run("raw var", func(t *testing.T) {
		raw := fmt.Sprintf("{{%s}}", key1)
		result := varsystem.GetVarKeyFromRaw(raw)
		if result != key1 {
			t.Errorf("Expected %s, got %s", key1, result)
		}
	})

	t.Run("non-raw var", func(t *testing.T) {
		wsVar, ok := vs.Get(key1)
		if !ok {
			t.Errorf("Expected to get var")
		}
		if wsVar.Value != value1 {
			t.Errorf("Expected %s, got %s", value1, wsVar.Value)
		}
	})
}

func TestLongStringReplace(t *testing.T) {
	const total_key = 10
	const total_val = 10
	const key_prefix = "key_"
	const val_prefix = "val_"

	const BaseUrl = "https://www.google.com/search?q="
	var expectedUrl string = BaseUrl
	var testUrl string = BaseUrl
	for i := 0; i < total_key; i++ {
		expectedUrl += fmt.Sprintf("%s%d", val_prefix, i)
	}
	for i := 0; i < total_key; i++ {
		testUrl += fmt.Sprintf("{{%s%d}}", key_prefix, i)
	}

	a := make([]mvar.Var, total_key)
	for i := 0; i < total_key; i++ {
		a[i] = mvar.Var{
			ID:     idwrap.NewNow(),
			VarKey: fmt.Sprintf("%s%d", key_prefix, i),
			EnvID:  idwrap.NewNow(),
			Value:  fmt.Sprintf("%s%d", val_prefix, i),
		}
	}

	vs := varsystem.NewVarMap(a)
	longUrlNew, err := vs.ReplaceVars(testUrl)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	if longUrlNew != expectedUrl {
		t.Errorf("Expected %s , got %s", expectedUrl, longUrlNew)
	}
}

func TestHostStringReplace(t *testing.T) {
	const hostVarKey = "host"
	const hostVarVal = "www.google.com"
	const BaseUrl = "https://{{host}}/search?q="

	expectedUrl := fmt.Sprintf("https://%s/search?q=", hostVarVal)

	a := mvar.Var{
		ID:     idwrap.NewNow(),
		EnvID:  idwrap.NewNow(),
		VarKey: hostVarKey,
		Value:  hostVarVal,
	}
	vs := varsystem.NewVarMap([]mvar.Var{a})
	urlNew, err := vs.ReplaceVars(BaseUrl)
	if err != nil {
		t.Fatalf("Error: %s", err)
	}

	if urlNew != expectedUrl {
		t.Errorf("Expected %s , got %s", expectedUrl, urlNew)
	}
}

func TestNewVarMapFromAnyMap(t *testing.T) {
	input := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": true,
		"key4": map[string]any{
			"nestedKey1": "nestedValue1",
		},
		"key5": []any{1, 2, 3},
	}

	expected := varsystem.VarMap{
		"key1":            mvar.Var{Value: "value1"},
		"key2":            mvar.Var{Value: "42"},
		"key3":            mvar.Var{Value: "true"},
		"key4.nestedKey1": mvar.Var{Value: "nestedValue1"},
		"key5[0]":         mvar.Var{Value: "1"},
		"key5[1]":         mvar.Var{Value: "2"},
		"key5[2]":         mvar.Var{Value: "3"},
	}

	result := varsystem.NewVarMapFromAnyMap(input)
	if result["key1"].Value != input["key1"] {
		fmt.Println(result)
		t.Errorf("Expected %v, got %v", expected["key1"].Value, result["key1"].Value)
	}

	if result["key2"].Value != fmt.Sprint(input["key2"]) {
		t.Errorf("Expected %v, got %v", expected["key2"].Value, result["key2"].Value)
	}

	if result["key3"].Value != fmt.Sprint(input["key3"]) {
		t.Errorf("Expected %v, got %v", expected["key3"].Value, result["key3"].Value)
	}

	if result["key4.nestedKey1"].Value != fmt.Sprint(input["key4"].(map[string]any)["nestedKey1"]) {
		t.Errorf("Expected %v, got %v", expected["key4.nestedKey1"].Value, result["key4.nestedKey1"].Value)
	}

	if result["key5[0]"].Value != fmt.Sprint(input["key5"].([]any)[0]) {
		t.Errorf("Expected %v, got %v", expected["key5[0]"].Value, result["key5[0]"].Value)
	}

	if result["key5[1]"].Value != fmt.Sprint(input["key5"].([]any)[1]) {
		t.Errorf("Expected %v, got %v", expected["key5[1]"].Value, result["key5[1]"].Value)
	}

	if result["key5[2]"].Value != fmt.Sprint(input["key5"].([]any)[2]) {
		t.Errorf("Expected %v, got %v", expected["key5[2]"].Value, result["key5[2]"].Value)
	}
}
