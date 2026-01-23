package response

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/request"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

func makeRequestResponse() request.RequestResponse {
	resp := httpclient.Response{
		StatusCode: 200,
		Body:       []byte(`{"foo":"bar"}`),
		Headers: []httpclient.Header{
			{HeaderKey: "Content-Type", Value: "application/json"},
		},
	}

	return request.RequestResponse{
		HttpResp: resp,
		LapTime:  10 * time.Millisecond,
	}
}

func makeAssertions(count int) []mhttp.HTTPAssert {
	asserts := make([]mhttp.HTTPAssert, 0, count)
	for i := 0; i < count; i++ {
		asserts = append(asserts, mhttp.HTTPAssert{
			ID:        idwrap.NewNow(),
			HttpID:    idwrap.NewNow(),
			Value:     "{{ response.status }} == 200",
			Enabled:   true,
			CreatedAt: time.Now().Unix(),
		})
	}
	return asserts
}

func TestResponseCreateEvaluatesAssertions(t *testing.T) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	httpResp := mhttp.HTTPResponse{ID: idwrap.NewNow(), HttpID: idwrap.NewNow()}
	assertions := makeAssertions(1)

	out, err := ResponseCreate(ctx, reqResp, httpResp, nil, assertions, map[string]any{})
	if err != nil {
		t.Fatalf("ResponseCreate returned error: %v", err)
	}
	if len(out.AssertCouples) != 1 {
		t.Fatalf("expected 1 assertion result, got %d", len(out.AssertCouples))
	}
	if !out.AssertCouples[0].AssertRes.Success {
		t.Fatalf("expected assertion to pass")
	}
}

func BenchmarkResponseCreateAssertions(b *testing.B) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	httpResp := mhttp.HTTPResponse{ID: idwrap.NewNow(), HttpID: idwrap.NewNow()}
	assertions := makeAssertions(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ResponseCreate(ctx, reqResp, httpResp, nil, assertions, map[string]any{}); err != nil {
			b.Fatalf("ResponseCreate error: %v", err)
		}
	}
}

func TestResponseCreateEvaluatesLoopVariables(t *testing.T) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	httpResp := mhttp.HTTPResponse{ID: idwrap.NewNow(), HttpID: idwrap.NewNow()}
	flowVars := map[string]any{
		"for_1": map[string]any{
			"index":           3,
			"totalIterations": 5,
		},
	}
	assertions := []mhttp.HTTPAssert{{
		ID:        idwrap.NewNow(),
		HttpID:    idwrap.NewNow(),
		Value:     "for_1.index < 5",
		Enabled:   true,
		CreatedAt: time.Now().Unix(),
	}}

	out, err := ResponseCreate(ctx, reqResp, httpResp, nil, assertions, flowVars)
	if err != nil {
		t.Fatalf("ResponseCreate returned error: %v", err)
	}
	if len(out.AssertCouples) != 1 {
		t.Fatalf("expected 1 assertion result, got %d", len(out.AssertCouples))
	}
	if !out.AssertCouples[0].AssertRes.Success {
		t.Fatalf("expected assertion to use loop index")
	}
	if len(out.CreateHeaders) != len(reqResp.HttpResp.Headers) {
		t.Fatalf("expected header diff to remain unchanged")
	}
}

func TestResponseCreateUnknownVariableProvidesHint(t *testing.T) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	httpResp := mhttp.HTTPResponse{ID: idwrap.NewNow(), HttpID: idwrap.NewNow()}
	flowVars := map[string]any{"for_1": map[string]any{"index": 2}}
	assertions := []mhttp.HTTPAssert{{
		ID:        idwrap.NewNow(),
		HttpID:    idwrap.NewNow(),
		Value:     "missing_var > 0",
		Enabled:   true,
		CreatedAt: time.Now().Unix(),
	}}

	_, err := ResponseCreate(ctx, reqResp, httpResp, nil, assertions, flowVars)
	if err == nil {
		t.Fatalf("expected error for missing variable")
	}
	if !strings.Contains(err.Error(), "available variables") {
		t.Fatalf("expected error message to mention available variables, got %v", err)
	}
}
