package nrequest

import (
	"encoding/json"
	"reflect"
	"testing"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
)

func legacyBuildNodeRequestOutputMap(output NodeRequestOutput) (map[string]any, error) {
	marshaledResp, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}

	respMap := make(map[string]any)
	if err := json.Unmarshal(marshaledResp, &respMap); err != nil {
		return nil, err
	}
	return respMap, nil
}

func sampleOutput() NodeRequestOutput {
	reqVar := request.RequestResponseVar{
		Method:  "GET",
		URL:     "https://example.test/resource",
		Headers: map[string]string{"Authorization": "Bearer token", "X-Test": "true"},
		Queries: map[string]string{"q": "value", "limit": "10"},
		Body:    "{}",
	}

	respVar := httpclient.ResponseVar{
		StatusCode: 200,
		Body: map[string]any{
			"message": "ok",
			"count":   float64(2),
		},
		Headers:  map[string]string{"Content-Type": "application/json"},
		Duration: 123,
	}

	return NodeRequestOutput{Request: reqVar, Response: respVar}
}

func TestBuildNodeRequestOutputMapMatchesLegacy(t *testing.T) {
	output := sampleOutput()

	expected, err := legacyBuildNodeRequestOutputMap(output)
	if err != nil {
		t.Fatalf("legacy builder returned error: %v", err)
	}

	got := buildNodeRequestOutputMap(output)

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("map mismatch\nexpected: %#v\n     got: %#v", expected, got)
	}
}

func BenchmarkLegacyBuildNodeRequestOutputMap(b *testing.B) {
	output := sampleOutput()
	for i := 0; i < b.N; i++ {
		if _, err := legacyBuildNodeRequestOutputMap(output); err != nil {
			b.Fatalf("legacy builder error: %v", err)
		}
	}
}

func BenchmarkNewBuildNodeRequestOutputMap(b *testing.B) {
	output := sampleOutput()
	for i := 0; i < b.N; i++ {
		if result := buildNodeRequestOutputMap(output); len(result) == 0 {
			b.Fatalf("builder returned empty map")
		}
	}
}
