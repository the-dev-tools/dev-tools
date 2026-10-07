package expression

import (
	"bytes"
	"encoding/json"
	"testing"
)

// decodeLikeHTTPClient mirrors httpclient.ConvertResponseToVar, which decodes JSON response
// bodies with UseNumber so integers keep their exact text when templated into later requests.
func decodeLikeHTTPClient(t *testing.T, raw string) any {
	t.Helper()
	var v any
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

func TestEvalBoolComparesJSONNumbersAsNumbers(t *testing.T) {
	body := decodeLikeHTTPClient(t, `{"qty": 3, "price": 129.99, "id": 9007199254740993, "items": [{"n": 2}]}`)
	env := NewUnifiedEnv(map[string]any{"response": map[string]any{"status": 200, "body": body}})

	for _, expr := range []string{
		"response.body.qty == 3",
		"response.body.price == 129.99",
		"response.body.price > 100",
		"response.body.qty + 1 == 4",
		"response.body.id == 9007199254740993",
		"response.body.items[0].n >= 2",
		"response.status == 200",
	} {
		ok, err := env.EvalBool(t.Context(), expr)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", expr, err)
			continue
		}
		if !ok {
			t.Errorf("%s: got false, want true", expr)
		}
	}
}

func TestInterpolateKeepsJSONNumberText(t *testing.T) {
	body := decodeLikeHTTPClient(t, `{"id": 9007199254740993, "ratio": 1.0}`)
	env := NewUnifiedEnv(map[string]any{"Login": map[string]any{"response": map[string]any{"body": body}}})

	got, err := env.Interpolate("{{ Login.response.body.id }}/{{ Login.response.body.ratio }}")
	if err != nil {
		t.Fatalf("interpolate: %v", err)
	}
	if want := "9007199254740993/1.0"; got != want {
		t.Errorf("Interpolate = %q, want %q", got, want)
	}
}
