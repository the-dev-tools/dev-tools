package aicheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const exampleA = `
workspace_name: Acme
judge:
  provider: anthropic
  api_key_env: MY_JUDGE_KEY
quality:
  fail_below: 85%
flows:
  - name: AssistantAnswersBilling
    iterations: 3
    steps:
      - request:
          name: Login
          method: POST
          url: '{{ BASE_URL }}/api/login'
      - request:
          name: AskAssistant
          depends_on: Login
          method: POST
          url: '{{ BASE_URL }}/api/assistant'
          assertions:
            - response.status == 200
          expect:
            schema:
              type: object
              required: [answer, citations]
              properties:
                answer: { type: string, minLength: 20 }
                citations: { type: array, minItems: 1 }
            max_latency_ms: 4000
            usage: response.body.usage
            max_output_tokens: 400
            judge:
              criteria: Answers how to cancel, using only facts from the cited docs.
              steps:
                - Check the answer says where cancellation happens (Settings > Billing).
              input: '{{ AskAssistant.request.body.question }}'
              output: '{{ response.body.answer }}'
              min_score: 4
  - name: Other
    judge: { provider: openai, base_url: 'http://localhost:9/v1/' }
    quality: { fail_below: 0.5 }
    steps:
      - request:
          name: Plain
          url: /x
`

func TestParseExampleA(t *testing.T) {
	f, err := Parse([]byte(exampleA), ".")
	if err != nil {
		t.Fatal(err)
	}
	if !f.HasChecks() {
		t.Fatal("HasChecks = false")
	}
	fl := f.Flow("AssistantAnswersBilling")
	if fl == nil || fl.Iterations != 3 || len(fl.Steps) != 1 || fl.Steps[0] != "AskAssistant" {
		t.Fatalf("flow = %+v", fl)
	}
	if fl.Judge.Provider != "anthropic" || fl.Judge.Model != "claude-haiku-4-5-20251001" || fl.Judge.BaseURL != "https://api.anthropic.com" ||
		fl.Judge.APIKeyEnv != "MY_JUDGE_KEY" || fl.Judge.Samples != 3 {
		t.Fatalf("judge = %+v", fl.Judge)
	}
	if fl.Quality.FailBelow == nil || *fl.Quality.FailBelow != 0.85 {
		t.Fatalf("quality = %+v", fl.Quality)
	}
	s := fl.Specs["AskAssistant"]
	if s.Schema == nil || *s.MaxLatencyMS != 4000 || *s.MaxOutputTokens != 400 || s.SchemaAt != "response.body" || s.Judge.MinScore != 4 || len(s.Judge.Steps) != 1 {
		t.Fatalf("spec = %+v", s)
	}
	if f.Flow("Other") != nil {
		t.Fatal("a flow without expect: has no checks")
	}
	other := f.Flows["Other"]
	if other.Judge.Provider != "openai" || other.Judge.Model != "gpt-4.1-mini" || other.Judge.BaseURL != "http://localhost:9/v1" ||
		other.Judge.APIKeyEnv != "MY_JUDGE_KEY" || *other.Quality.FailBelow != 0.5 {
		t.Fatalf("other = %+v %+v", other.Judge, other.Quality)
	}
}

func TestParseNoChecks(t *testing.T) {
	f, err := Parse([]byte("flows:\n  - name: A\n    steps:\n      - request: { name: X, url: /x }\n"), ".")
	if err != nil || f.HasChecks() {
		t.Fatalf("f=%+v err=%v", f, err)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"unknown key":     "expect: { max_latency: 5 }",
		"min_score range": "expect: { judge: { criteria: c, min_score: 7 } }",
		"no criteria":     "expect: { judge: { min_score: 4 } }",
		"negative budget": "expect: { max_output_tokens: -1 }",
		"empty":           "expect: { usage: response.body.usage }",
		"bad schema":      "expect: { schema: { type: 12 } }",
		"missing file":    "expect: { schema: nope.json }",
		"schema_at alone": "expect: { schema_at: response.body.x, max_latency_ms: 4 }",
		"judge typo":      "expect: { judge: { criteria: c, steps: [a], minscore: 4 } }",
	}
	for name, exp := range cases {
		t.Run(name, func(t *testing.T) {
			doc := "flows:\n  - name: F\n    steps:\n      - request:\n          name: S\n          url: /x\n          " + exp + "\n"
			_, err := Parse([]byte(doc), t.TempDir())
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), "flow F, step S: expect:") {
				t.Fatalf("error does not name the step: %v", err)
			}
		})
	}
	for name, doc := range map[string]string{
		"rate":        "quality: { fail_below: 150% }\nflows: []\n",
		"provider":    "judge: { provider: gemini }\nflows: []\n",
		"iterations":  "flows:\n  - name: F\n    iterations: 0\n    steps: []\n",
		"non-request": "flows:\n  - name: F\n    steps:\n      - js: { name: J, code: x, expect: { max_latency_ms: 4 } }\n",
	} {
		if _, err := Parse([]byte(doc), "."); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestParseSchemaFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "schemas"), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "schemas", "a.json"), []byte(`{"type":"object","required":["answer"]}`), 0o666); err != nil {
		t.Fatal(err)
	}
	doc := "flows:\n  - name: F\n    steps:\n      - request:\n          name: S\n          url: /x\n          expect: { schema: schemas/a.json, schema_at: response.body.data }\n"
	f, err := Parse([]byte(doc), dir)
	if err != nil {
		t.Fatal(err)
	}
	s := f.Flow("F").Specs["S"]
	if s.Schema == nil || s.SchemaAt != "response.body.data" {
		t.Fatalf("spec = %+v", s)
	}
}

func TestParseRate(t *testing.T) {
	for in, want := range map[string]float64{"90%": 0.9, "0.9": 0.9, "100%": 1, "0": 0, " 85 %": 0.85} {
		got, err := ParseRate(in)
		if err != nil || got != want {
			t.Errorf("ParseRate(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"90", "-1%", "abc", ""} {
		if _, err := ParseRate(in); err == nil {
			t.Errorf("ParseRate(%q): want an error", in)
		}
	}
}
