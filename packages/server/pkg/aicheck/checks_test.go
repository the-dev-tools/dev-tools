package aicheck

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustParse(t *testing.T, expectYAML string) *Spec {
	t.Helper()
	doc := "flows:\n  - name: F\n    steps:\n      - request:\n          name: Ask\n          url: /x\n          expect:\n" + expectYAML
	f, err := Parse([]byte(doc), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return f.Flow("F").Specs["Ask"]
}

func data(t *testing.T, js string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func run(t *testing.T, ms float64, js string) Run {
	return Run{Steps: map[string]StepRun{
		"Login": {Ran: true, Data: data(t, `{"request":{"body":"{\"email\":\"a@b.c\"}"},"response":{"status":200,"body":{"token":"t"}}}`)},
		"Ask":   {Ran: true, DurationMS: &ms, Data: data(t, js)},
	}}
}

const goodAsk = `{"request":{"body":"{\"question\":\"How do I cancel?\"}"},
 "response":{"status":200,"body":{"answer":"Open Settings > Billing and press Cancel.","citations":["d"],"usage":{"input_tokens":800,"output_tokens":61}}}}`

func TestSchema(t *testing.T) {
	spec := mustParse(t, `            schema:
              type: object
              required: [answer, citations]
              properties:
                answer: { type: string, minLength: 20 }
                citations: { type: array, minItems: 1 }
`)
	if r := Deterministic(spec, "Ask", run(t, 10, goodAsk)); len(r) != 1 || !r[0].Passed || r[0].Kind != KindSchema {
		t.Fatalf("good: %+v", r)
	}
	bad := `{"response":{"status":200,"body":{"answer":"short","citations":[]}}}`
	r := Deterministic(spec, "Ask", run(t, 10, bad))
	if r[0].Passed || !strings.Contains(r[0].Reason, "/answer") && !strings.Contains(r[0].Reason, "/citations") {
		t.Fatalf("bad: %+v", r)
	}
	r = Deterministic(spec, "Ask", run(t, 10, `{"response":{"status":200,"body":"not json"}}`))
	if r[0].Passed || !strings.Contains(r[0].Reason, "body") {
		t.Fatalf("string body: %+v", r)
	}
	r = Deterministic(spec, "Ask", run(t, 10, `{"request":{}}`))
	if r[0].Passed || r[0].Reason != "no response" {
		t.Fatalf("no response: %+v", r)
	}
}

func TestSchemaAt(t *testing.T) {
	spec := mustParse(t, "            schema: { type: integer, maximum: 100 }\n            schema_at: response.body.usage.output_tokens\n")
	if r := Deterministic(spec, "Ask", run(t, 10, goodAsk)); !r[0].Passed {
		t.Fatalf("%+v", r)
	}
	spec = mustParse(t, "            schema: { type: string }\n            schema_at: response.body.nope\n")
	if r := Deterministic(spec, "Ask", run(t, 10, goodAsk)); r[0].Passed || r[0].Reason != "response.body.nope not found" {
		t.Fatalf("%+v", r)
	}
}

func TestLatency(t *testing.T) {
	spec := mustParse(t, "            max_latency_ms: 500\n")
	r := Deterministic(spec, "Ask", run(t, 499.5, goodAsk))[0]
	if !r.Passed || *r.Value != 499.5 || *r.Limit != 500 || r.Reason != "500 ms ≤ 500 ms" {
		t.Fatalf("%+v", r)
	}
	r = Deterministic(spec, "Ask", run(t, 1812, goodAsk))[0]
	if r.Passed || r.Reason != "1.8 s > 500 ms" {
		t.Fatalf("%+v", r)
	}
}

func TestTokens(t *testing.T) {
	spec := mustParse(t, "            max_output_tokens: 100\n            max_total_tokens: 1000\n")
	r := Deterministic(spec, "Ask", run(t, 1, goodAsk))[0]
	if !r.Passed || *r.Value != 61 || *r.Limit != 100 || r.Reason != "output 61 ≤ 100, total 861 ≤ 1000" {
		t.Fatalf("%+v", r)
	}
	spec = mustParse(t, "            max_input_tokens: 500\n")
	r = Deterministic(spec, "Ask", run(t, 1, goodAsk))[0]
	if r.Passed || r.Reason != "input 800 > 500" {
		t.Fatalf("%+v", r)
	}
	// OpenAI names, custom usage path.
	spec = mustParse(t, "            usage: response.body.meta.usage\n            max_output_tokens: 10\n")
	r = Deterministic(spec, "Ask", run(t, 1, `{"response":{"status":200,"body":{"meta":{"usage":{"prompt_tokens":5,"completion_tokens":12}}}}}`))[0]
	if r.Passed || r.Reason != "output 12 > 10" {
		t.Fatalf("%+v", r)
	}
	r = Deterministic(spec, "Ask", run(t, 1, goodAsk))[0]
	if r.Passed || r.Reason != "response.body.meta.usage not found" {
		t.Fatalf("%+v", r)
	}
}

func TestRender(t *testing.T) {
	r := run(t, 1, goodAsk)
	out, missing := Render("Q: {{ Ask.request.body.question }} / {{response.body.citations}} / {{ Login.response.body.token }}", "Ask", r)
	if missing != "" || out != `Q: How do I cancel? / ["d"] / t` {
		t.Fatalf("out=%q missing=%q", out, missing)
	}
	if _, missing := Render("{{ response.body.answer }} {{ response.body.reply }}", "Ask", r); missing != "response.body.reply" {
		t.Fatalf("missing = %q", missing)
	}
	r.Steps["Ask"].Data["response"].(map[string]any)["body"] = map[string]any{"tool_calls": []any{map[string]any{"name": "lookup_order"}}}
	if out, _ := Render("{{ response.body.tool_calls[0].name }}", "Ask", r); out != "lookup_order" {
		t.Fatalf("index: %q", out)
	}
}
