package response

import (
	"context"
	"strings"
	"testing"
	"time"

	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/varsystem"
)

func makeRequestResponse() request.RequestResponse {
	resp := httpclient.Response{
		StatusCode: 200,
		Body:       []byte(`{"foo":"bar"}`),
		Headers: []mexamplerespheader.ExampleRespHeader{
			{HeaderKey: "Content-Type", Value: "application/json"},
		},
	}

	return request.RequestResponse{
		HttpResp: resp,
		LapTime:  10 * time.Millisecond,
	}
}

func makeAssertions(count int) []massert.Assert {
	asserts := make([]massert.Assert, 0, count)
	for i := 0; i < count; i++ {
		asserts = append(asserts, massert.Assert{
			ID:        idwrap.NewNow(),
			Enable:    true,
			Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "{{ response.status }} == 200"}},
		})
	}
	return asserts
}

func TestResponseCreateEvaluatesAssertions(t *testing.T) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	example := mexampleresp.ExampleResp{ID: idwrap.NewNow(), ExampleID: idwrap.NewNow()}
	assertions := makeAssertions(1)
	varMap := varsystem.NewVarMapFromAnyMap(map[string]any{})

	out, err := ResponseCreate(ctx, reqResp, example, nil, assertions, varMap, nil)
	if err != nil {
		t.Fatalf("ResponseCreate returned error: %v", err)
	}
	if len(out.AssertCouples) != 1 {
		t.Fatalf("expected 1 assertion result, got %d", len(out.AssertCouples))
	}
	if !out.AssertCouples[0].AssertRes.Result {
		t.Fatalf("expected assertion to pass")
	}
}

func BenchmarkResponseCreateAssertions(b *testing.B) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	example := mexampleresp.ExampleResp{ID: idwrap.NewNow(), ExampleID: idwrap.NewNow()}
	assertions := makeAssertions(100)
	varMap := varsystem.NewVarMapFromAnyMap(map[string]any{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ResponseCreate(ctx, reqResp, example, nil, assertions, varMap, nil); err != nil {
			b.Fatalf("ResponseCreate error: %v", err)
		}
	}
}

func TestResponseCreateEvaluatesLoopVariables(t *testing.T) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	example := mexampleresp.ExampleResp{ID: idwrap.NewNow(), ExampleID: idwrap.NewNow()}
	flowVars := map[string]any{
		"for_1": map[string]any{
			"index":           3,
			"totalIterations": 5,
		},
	}
	assertions := []massert.Assert{{
		ID:     idwrap.NewNow(),
		Enable: true,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{
			Expression: "for_1.index < 5",
		}},
	}}
	varMap := varsystem.NewVarMapFromAnyMap(flowVars)

	out, err := ResponseCreate(ctx, reqResp, example, nil, assertions, varMap, flowVars)
	if err != nil {
		t.Fatalf("ResponseCreate returned error: %v", err)
	}
	if len(out.AssertCouples) != 1 {
		t.Fatalf("expected 1 assertion result, got %d", len(out.AssertCouples))
	}
	if !out.AssertCouples[0].AssertRes.Result {
		t.Fatalf("expected assertion to use loop index")
	}
	if len(out.CreateHeaders) != len(reqResp.HttpResp.Headers) {
		t.Fatalf("expected header diff to remain unchanged")
	}
}

func TestResponseCreateUnknownVariableProvidesHint(t *testing.T) {
	ctx := context.Background()
	reqResp := makeRequestResponse()
	example := mexampleresp.ExampleResp{ID: idwrap.NewNow(), ExampleID: idwrap.NewNow()}
	flowVars := map[string]any{"for_1": map[string]any{"index": 2}}
	assertions := []massert.Assert{{
		ID:     idwrap.NewNow(),
		Enable: true,
		Condition: mcondition.Condition{Comparisons: mcondition.Comparison{
			Expression: "missing_var > 0",
		}},
	}}
	varMap := varsystem.NewVarMapFromAnyMap(flowVars)

	_, err := ResponseCreate(ctx, reqResp, example, nil, assertions, varMap, flowVars)
	if err == nil {
		t.Fatalf("expected error for missing variable")
	}
	if !strings.Contains(err.Error(), "available variables") {
		t.Fatalf("expected error message to mention available variables, got %v", err)
	}
}
