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
		raw := fmt.Sprintf("{{env.%s}}", key1)
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
		testUrl += fmt.Sprintf("{{env.%s%d}}", key_prefix, i)
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
	const BaseUrl = "https://{{env.host}}/search?q="

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
