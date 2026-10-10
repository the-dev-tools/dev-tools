package aicheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
)

func TestTTFT(t *testing.T) {
	spec := mustParse(t, "            max_ttft_ms: 800\n")
	streamed := `{"response":{"status":200,"body":"data: x","ttft_ms":120,"text":"Hello"}}`
	r := Deterministic(spec, "Ask", run(t, 900, streamed))[0]
	if r.Kind != KindTTFT || !r.Passed || *r.Value != 120 || *r.Limit != 800 || r.Reason != "120 ms ≤ 800 ms" {
		t.Fatalf("%+v", r)
	}
	slow := `{"response":{"status":200,"body":"data: x","ttft_ms":1250}}`
	if r := Deterministic(spec, "Ask", run(t, 1300, slow))[0]; r.Passed || r.Reason != "1.2 s > 800 ms" {
		t.Fatalf("%+v", r)
	}
	if r := Deterministic(spec, "Ask", run(t, 10, goodAsk))[0]; r.Passed || r.Reason != "no ttft_ms: the response was not streamed, or no token arrived" {
		t.Fatalf("not streamed: %+v", r)
	}
}

func TestTokensFallBackToStreamUsage(t *testing.T) {
	spec := mustParse(t, "            max_output_tokens: 100\n")
	streamed := `{"response":{"status":200,"body":"data: ...","usage":{"input_tokens":9,"output_tokens":42}}}`
	if r := Deterministic(spec, "Ask", run(t, 1, streamed))[0]; !r.Passed || r.Reason != "output 42 ≤ 100" {
		t.Fatalf("%+v", r)
	}
	// A usage path the step names has no fallback.
	spec = mustParse(t, "            usage: response.body.usage\n            max_output_tokens: 100\n")
	if r := Deterministic(spec, "Ask", run(t, 1, streamed))[0]; r.Passed || r.Reason != "response.body.usage not found" {
		t.Fatalf("%+v", r)
	}
}

func TestEngineNumberShapes(t *testing.T) {
	// The engine decodes bodies with json.Number; usage counts and statuses still read.
	ms := 5.0
	r := Run{Steps: map[string]StepRun{"Ask": {Ran: true, DurationMS: &ms, Data: map[string]any{
		"response": map[string]any{"status": 200, "body": map[string]any{
			"usage": map[string]any{"output_tokens": json.Number("61")}}},
	}}}}
	spec := mustParse(t, "            max_output_tokens: 60\n")
	if res := Deterministic(spec, "Ask", r)[0]; res.Passed || res.Reason != "output 61 > 60" {
		t.Fatalf("%+v", res)
	}
	shaped, ok := JSONShape(map[string]any{"n": json.Number("3"), "s": []string{"a"}})
	if !ok || shaped["n"] != 3.0 || shaped["s"].([]any)[0] != "a" {
		t.Fatalf("%#v", shaped)
	}
}

func TestCompileSpecWithoutDirDefersSchemaFile(t *testing.T) {
	spec, err := CompileSpec(mexpect.Expect{Schema: "schemas/a.json"}, "")
	if err != nil || spec.Schema != nil {
		t.Fatalf("spec=%+v err=%v", spec, err)
	}
	if _, err := CompileSpec(mexpect.Expect{Schema: "schemas/a.json"}, t.TempDir()); err == nil {
		t.Fatal("a missing schema file must fail once the directory is known")
	}
	// Compiling never changes the model's inline schema.
	inline := map[string]any{"type": "object", "properties": map[string]any{"n": map[string]any{"maximum": 3}}}
	if _, err := CompileSpec(mexpect.Expect{Schema: inline}, ""); err != nil {
		t.Fatal(err)
	}
	if _, isInt := inline["properties"].(map[string]any)["n"].(map[string]any)["maximum"].(int); !isInt {
		t.Fatal("the model's schema was normalized in place")
	}
}

func TestKeyEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# judge\nexport FROM_DOTENV=\"quoted value\"\nPLAIN=abc # note\nBAD LINE\nAICHECK_TEST_OVERRIDE=file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AICHECK_TEST_OVERRIDE", "process")
	env := KeyEnvironment(path)
	if env["FROM_DOTENV"] != "quoted value" || env["PLAIN"] != "abc" || env["AICHECK_TEST_OVERRIDE"] != "process" {
		t.Fatalf("%v", map[string]string{"FROM_DOTENV": env["FROM_DOTENV"], "PLAIN": env["PLAIN"], "O": env["AICHECK_TEST_OVERRIDE"]})
	}
	if _, ok := env["BAD LINE"]; ok {
		t.Fatal("kept a malformed line")
	}
	if KeyEnvironment(filepath.Join(dir, "missing"))["AICHECK_TEST_OVERRIDE"] != "process" {
		t.Fatal("a missing .env must not hide the process environment")
	}
}
