package yamlflowsimplev2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
)

// aiChecksYAML uses the Stresseur AI checks spec's syntax unchanged, plus the engine's stream:.
const aiChecksYAML = `
workspace_name: Acme
judge:
  provider: anthropic
  api_key_env: MY_JUDGE_KEY
quality:
  fail_below: 85%
flows:
  - name: AssistantAnswersBilling
    iterations: 3
    judge: { model: claude-sonnet-4-5, samples: 1 }
    quality: { fail_below: 0.9 }
    steps:
      - request:
          name: Login
          method: POST
          url: https://app.test/api/login
      - request:
          name: AskAssistant
          depends_on: Login
          method: POST
          url: https://app.test/api/assistant
          stream: openai
          stream_timeout_ms: 20000
          assertions:
            - response.status == 200
          expect:
            schema:
              type: object
              required: [answer, citations]
            schema_at: response.body
            max_latency_ms: 4000
            max_ttft_ms: 800
            usage: response.body.usage
            max_output_tokens: 400
            judge:
              criteria: Answers how to cancel, using only facts from the cited docs.
              steps:
                - Check the answer says where cancellation happens (Settings > Billing).
              input: '{{ AskAssistant.request.body.question }}'
              output: '{{ response.body.answer }}'
              min_score: 4
      - graphql:
          name: Plans
          depends_on: AskAssistant
          url: https://app.test/graphql
          query: '{ plans { id } }'
          expect:
            max_latency_ms: 500
    cleanup:
      - request:
          name: Logout
          method: POST
          url: https://app.test/api/logout
          expect:
            max_latency_ms: 300
`

func nodeIDByName(t *testing.T, b *ioworkspace.WorkspaceBundle, name string) idwrap.IDWrap {
	t.Helper()
	for _, n := range b.FlowNodes {
		if n.Name == name {
			return n.ID
		}
	}
	for _, c := range b.FlowCleanups {
		for _, s := range c.Steps {
			if s.Name == name {
				return s.NodeID
			}
		}
	}
	t.Fatalf("no node %s", name)
	return idwrap.IDWrap{}
}

func TestAIChecksImport(t *testing.T) {
	b, err := ConvertSimplifiedYAML([]byte(aiChecksYAML), GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err)
	require.NotNil(t, b.AIChecks)
	require.Equal(t, &mexpect.JudgeConfig{Provider: "anthropic", APIKeyEnv: "MY_JUDGE_KEY"}, b.AIChecks.Judge)
	require.Equal(t, "85%", b.AIChecks.Quality.FailBelow)

	flow := b.AIChecks.Flows[b.Flows[0].ID]
	require.Equal(t, 3, *flow.Iterations)
	require.Equal(t, &mexpect.JudgeConfig{Model: "claude-sonnet-4-5", Samples: 1}, flow.Judge)
	require.Equal(t, "0.9", flow.Quality.FailBelow)

	ask := b.AIChecks.Steps[nodeIDByName(t, b, "AskAssistant")]
	require.InDelta(t, 4000.0, *ask.MaxLatencyMS, 0)
	require.InDelta(t, 800.0, *ask.MaxTTFTMS, 0)
	require.InDelta(t, 400.0, *ask.MaxOutputTokens, 0)
	require.Equal(t, "response.body.usage", ask.Usage)
	require.Equal(t, "Answers how to cancel, using only facts from the cited docs.", ask.Judge.Criteria)
	require.InDelta(t, 4.0, ask.Judge.MinScore, 0)
	require.NotNil(t, ask.Schema)

	require.Contains(t, b.AIChecks.Steps, nodeIDByName(t, b, "Plans"))
	require.NotContains(t, b.AIChecks.Steps, nodeIDByName(t, b, "Login"))
	require.Contains(t, b.StepExpects(), nodeIDByName(t, b, "Logout"))

	require.Equal(t, httpclient.StreamOptions{Preset: "openai", Timeout: 20 * time.Second},
		b.RequestStreams[nodeIDByName(t, b, "AskAssistant")])
}

func TestAIChecksRoundTrip(t *testing.T) {
	b, err := ConvertSimplifiedYAML([]byte(aiChecksYAML), GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err)
	out, err := MarshalSimplifiedYAML(b)
	require.NoError(t, err)
	text := string(out)
	for _, want := range []string{"api_key_env: MY_JUDGE_KEY", "fail_below: 85%", "iterations: 3", "samples: 1",
		"stream: openai", "stream_timeout_ms: 20000", "max_ttft_ms: 800", "min_score: 4",
		"criteria: Answers how to cancel"} {
		require.Contains(t, text, want)
	}

	again, err := ConvertSimplifiedYAML(out, GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err, text)
	require.Equal(t, b.AIChecks.Judge, again.AIChecks.Judge)
	require.Equal(t, b.AIChecks.Quality, again.AIChecks.Quality)
	require.Equal(t, b.AIChecks.Flows[b.Flows[0].ID], again.AIChecks.Flows[again.Flows[0].ID])
	for _, name := range []string{"AskAssistant", "Plans", "Logout"} {
		require.Equal(t, b.StepExpects()[nodeIDByName(t, b, name)], again.StepExpects()[nodeIDByName(t, again, name)], name)
	}
	require.Equal(t, b.RequestStreams[nodeIDByName(t, b, "AskAssistant")], again.RequestStreams[nodeIDByName(t, again, "AskAssistant")])
}

func TestFlowWithoutAIChecksHasNone(t *testing.T) {
	b, err := ConvertSimplifiedYAML([]byte("workspace_name: W\nflows:\n  - name: F\n    steps:\n      - request: { name: A, url: https://x.test }\n"),
		GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err)
	require.True(t, b.AIChecks.IsEmpty())
	require.Empty(t, b.RequestStreams)
	out, err := MarshalSimplifiedYAML(b)
	require.NoError(t, err)
	require.NotContains(t, string(out), "expect:")
	require.NotContains(t, string(out), "judge:")
}

func TestAIChecksValidation(t *testing.T) {
	step := func(extra string) string {
		return "workspace_name: W\nflows:\n  - name: F\n    steps:\n      - request:\n          name: S\n          url: https://x.test\n          " + extra + "\n"
	}
	cases := map[string]struct{ doc, want string }{
		"unknown key":     {step("expect: { max_latency: 5 }"), "flow F, step S: expect: "},
		"min_score range": {step("expect: { judge: { criteria: c, min_score: 7 } }"), "min_score is on the 1–5 scale"},
		"no criteria":     {step("expect: { judge: { min_score: 4 } }"), "criteria is required"},
		"negative budget": {step("expect: { max_output_tokens: -1 }"), "max_output_tokens must be at least 0"},
		"empty":           {step("expect: { usage: response.body.usage }"), "no check"},
		"bad schema":      {step("expect: { schema: { type: 12 } }"), "schema:"},
		"schema_at alone": {step("expect: { schema_at: response.body.x, max_latency_ms: 4 }"), "schema_at needs a schema"},
		"judge typo":      {step("expect: { judge: { criteria: c, steps: [a], minscore: 4 } }"), "minscore"},
		"stream preset":   {step("stream: grpc"), "stream must be openai, anthropic, vercel-ai or sse"},
		"stream timeout":  {step("stream: sse\n          stream_timeout_ms: -5"), "stream_timeout_ms must be positive"},
		"file judge":      {"judge: { provider: gemini }\n" + step(""), "file judge: provider must be anthropic or openai"},
		"file judge key":  {"judge: { provider: openai, apikey: x }\n" + step(""), "apikey"},
		"quality rate":    {"quality: { fail_below: 150% }\n" + step(""), "quality"},
		"flow iterations": {"workspace_name: W\nflows:\n  - name: F\n    iterations: 0\n    steps:\n      - request: { name: S, url: https://x.test }\n", "iterations must be a whole number of at least 1"},
		"non-request":     {"workspace_name: W\nflows:\n  - name: F\n    steps:\n      - js: { name: J, code: x, expect: { max_latency_ms: 4 } }\n", "expect: works on request and graphql steps only"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ConvertSimplifiedYAML([]byte(c.doc), GetDefaultOptions(idwrap.NewNow()))
			require.Error(t, err)
			require.Contains(t, err.Error(), c.want)
		})
	}
}

func TestAIChecksSchemaFileUsesBaseDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "answer.json"), []byte(`{"type":"object"}`), 0o600))
	doc := "workspace_name: W\nflows:\n  - name: F\n    steps:\n      - request:\n          name: S\n          url: https://x.test\n          expect: { schema: answer.json }\n"
	opts := GetDefaultOptions(idwrap.NewNow())
	opts.BaseDir = dir
	_, err := ConvertSimplifiedYAML([]byte(doc), opts)
	require.NoError(t, err)

	opts.BaseDir = t.TempDir()
	_, err = ConvertSimplifiedYAML([]byte(doc), opts)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "could not read answer.json"), err.Error())
}
