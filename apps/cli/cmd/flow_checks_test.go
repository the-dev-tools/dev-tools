package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/aicheck"
)

// checksApp is the app under test: a JSON assistant endpoint and an OpenAI-style SSE chat.
func checksApp(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/assistant":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"answer":"Open Settings > Billing and press Cancel.","citations":["billing.md"],"usage":{"input_tokens":800,"output_tokens":61}}`))
		case "/api/chat":
			w.Header().Set("Content-Type", "text/event-stream")
			for _, c := range []string{"Settings", " > Billing"} {
				_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", c)
				w.(http.Flusher).Flush()
			}
			_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// fakeJudge is an Anthropic Messages API that scores every output with score.
func fakeJudge(t *testing.T, score int) (string, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		b, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "judge-key" || !strings.Contains(string(b), "app_output_") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		reply := fmt.Sprintf(`{"reason": "scored %d", "score": %d}`, score, score)
		_, _ = fmt.Fprintf(w, `{"content":[{"type":"text","text":%q}],"stop_reason":"end_turn","usage":{"input_tokens":900,"output_tokens":20}}`, reply)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &calls
}

func checksFlow(app, judge, quality string) string {
	return fmt.Sprintf(`
workspace_name: Acme
judge:
  provider: anthropic
  model: judge-model
  base_url: %[2]s
  api_key_env: AICHECK_E2E_JUDGE_KEY
%[3]s
flows:
  - name: Assistant
    steps:
      - request:
          name: AskAssistant
          method: POST
          url: %[1]s/api/assistant
          body:
            question: How do I cancel?
          assertions:
            - response.status == 200
          expect:
            schema:
              type: object
              required: [answer, citations]
            max_latency_ms: 4000
            max_output_tokens: 400
            judge:
              criteria: Answers how to cancel.
              steps: [Check it names Settings > Billing.]
              input: '{{ AskAssistant.request.body.question }}'
              output: '{{ response.body.answer }}'
              min_score: 4
      - request:
          name: Chat
          depends_on: AskAssistant
          method: POST
          url: %[1]s/api/chat
          stream: openai
          assertions:
            - response.text == "Settings > Billing"
          expect:
            max_ttft_ms: 4000
            max_output_tokens: 10
            judge:
              criteria: Names where to cancel.
              output: '{{ response.text }}'
`, app, judge, quality)
}

func runChecksFlow(t *testing.T, doc string) ([]model.FlowRunResult, error) {
	t.Helper()
	dir := t.TempDir()
	flowFile := filepath.Join(dir, "assistant.yaml")
	require.NoError(t, os.WriteFile(flowFile, []byte(doc), 0o600))
	report := filepath.Join(dir, "report.json")

	prevFormats, prevQuiet, prevCache := reportFormats, quietMode, judgeCachePath
	t.Cleanup(func() { reportFormats, quietMode, judgeCachePath = prevFormats, prevQuiet, prevCache })
	reportFormats = []string{"json:" + report}
	quietMode = true
	if consoleMode {
		reportFormats = append(reportFormats, "console")
		quietMode = false
	}
	if judgeCachePath == aicheck.DefaultCachePath {
		judgeCachePath = "off"
	}
	yamlflowRunCmd.SetContext(context.Background())
	err := yamlflowRunCmd.RunE(yamlflowRunCmd, []string{flowFile, "Assistant"})
	return readReport(t, report), err
}

func checksOf(t *testing.T, results []model.FlowRunResult, step string) []aicheck.Result {
	t.Helper()
	for _, n := range results[0].Nodes {
		if n.Name == step {
			return n.Checks
		}
	}
	t.Fatalf("no step %s", step)
	return nil
}

func TestFlowRunEvaluatesAIChecks(t *testing.T) {
	app := checksApp(t)
	judge, calls := fakeJudge(t, 5)
	t.Setenv("AICHECK_E2E_JUDGE_KEY", "judge-key")

	results, err := runChecksFlow(t, checksFlow(app, judge, ""))
	require.NoError(t, err)
	require.Equal(t, "passed", results[0].ChecksStatus)
	require.NotNil(t, results[0].Judge)
	require.Equal(t, 2, results[0].Judge.Calls)

	ask := checksOf(t, results, "AskAssistant")
	require.Len(t, ask, 4)
	kinds := []string{}
	for _, c := range ask {
		kinds = append(kinds, c.Kind)
		require.True(t, c.Passed, "%+v", c)
	}
	require.Equal(t, []string{"schema", "latency", "tokens", "judge"}, kinds)
	require.InDelta(t, 5.0, *ask[3].Score, 0)
	require.Equal(t, "judge-model", *ask[3].JudgeModel)
	require.Equal(t, "output 61 ≤ 400", ask[2].Reason)

	chat := checksOf(t, results, "Chat")
	require.Len(t, chat, 3)
	require.Equal(t, "ttft", chat[0].Kind)
	require.True(t, chat[0].Passed, "%+v", chat[0])
	require.Equal(t, "output 3 ≤ 10", chat[1].Reason) // the stream's usage
	require.True(t, chat[2].Passed)
	require.Equal(t, int32(2), calls.Load())

	// The JSON report carries the documented fields.
	raw, err := json.Marshal(chat[2])
	require.NoError(t, err)
	for _, k := range []string{`"kind"`, `"passed"`, `"score"`, `"reason"`, `"judge_model"`, `"cached"`} {
		require.Contains(t, string(raw), k)
	}

}

// flow run prints the checks under the flow's table, and the no-key notice once.
func TestFlowRunPrintsAIChecks(t *testing.T) {
	app := checksApp(t)
	judge, _ := fakeJudge(t, 5)
	t.Setenv("AICHECK_E2E_JUDGE_KEY", "judge-key")
	stdout, _ := captureOutput(t, func() {
		consoleMode = true
		_, err := runChecksFlow(t, checksFlow(app, judge, ""))
		require.NoError(t, err)
	})
	for _, want := range []string{"Checks\n", "  ✓ AskAssistant schema\n", "  ✓ AskAssistant latency ", "  ✓ AskAssistant tokens output 61 ≤ 400\n",
		"  ✓ AskAssistant judge 5 (min 4): scored 5\n", "  ✓ Chat ttft ", "Judge: 2 calls"} {
		require.Contains(t, stdout, want)
	}

	t.Setenv("AICHECK_E2E_JUDGE_KEY", "")
	stdout, stderr := captureOutput(t, func() {
		consoleMode = true
		_, err := runChecksFlow(t, checksFlow(app, judge, ""))
		require.NoError(t, err)
	})
	require.Contains(t, stdout, "  – AskAssistant judge skipped: no judge API key: set AICHECK_E2E_JUDGE_KEY\n")
	require.Equal(t, 1, strings.Count(stderr, "Judge checks skipped: set AICHECK_E2E_JUDGE_KEY."))
}

// consoleMode makes runChecksFlow add the console reporter.
var consoleMode bool

// captureOutput runs fn with os.Stdout and os.Stderr redirected, returning what it printed.
func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()
	read := func(f *os.File) <-chan string {
		ch := make(chan string, 1)
		go func() {
			b, _ := io.ReadAll(f)
			ch <- string(b)
		}()
		return ch
	}
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	errR, errW, err := os.Pipe()
	require.NoError(t, err)
	prevOut, prevErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	outCh, errCh := read(outR), read(errR)
	defer func() { consoleMode = false }()
	func() {
		defer func() { os.Stdout, os.Stderr = prevOut, prevErr }()
		fn()
	}()
	_ = outW.Close()
	_ = errW.Close()
	return <-outCh, <-errCh
}

func TestFlowRunAIChecksCache(t *testing.T) {
	app := checksApp(t)
	judge, calls := fakeJudge(t, 5)
	t.Setenv("AICHECK_E2E_JUDGE_KEY", "judge-key")
	prev := judgeCachePath
	t.Cleanup(func() { judgeCachePath = prev })
	judgeCachePath = filepath.Join(t.TempDir(), "cache", "judge.json")

	_, _ = runChecksFlow(t, checksFlow(app, judge, ""))
	results, err := runChecksFlow(t, checksFlow(app, judge, ""))
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load(), "the second run must be served from the cache")
	require.True(t, checksOf(t, results, "AskAssistant")[3].Cached)
	require.Equal(t, 2, results[0].Judge.Cached)
}

func TestFlowRunAIChecksFailBelowFailsTheFlow(t *testing.T) {
	app := checksApp(t)
	judge, _ := fakeJudge(t, 2)
	t.Setenv("AICHECK_E2E_JUDGE_KEY", "judge-key")

	// Without quality.fail_below a failed check only reports.
	results, err := runChecksFlow(t, checksFlow(app, judge, ""))
	require.NoError(t, err)
	require.Equal(t, "success", results[0].Status)
	require.Equal(t, "warn", results[0].ChecksStatus)
	require.False(t, checksOf(t, results, "AskAssistant")[3].Passed)

	results, err = runChecksFlow(t, checksFlow(app, judge, "quality: { fail_below: 90% }"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "AI checks failed: AskAssistant judge, Chat judge")
	require.Equal(t, "failed", results[0].Status)
	require.Equal(t, "failed", results[0].ChecksStatus)
}

func TestFlowRunAIChecksWithoutJudgeKey(t *testing.T) {
	app := checksApp(t)
	judge, calls := fakeJudge(t, 5)
	t.Setenv("AICHECK_E2E_JUDGE_KEY", "")

	results, err := runChecksFlow(t, checksFlow(app, judge, "quality: { fail_below: 90% }"))
	require.NoError(t, err, "a skipped judge never fails the flow")
	require.Equal(t, int32(0), calls.Load())
	j := checksOf(t, results, "AskAssistant")[3]
	require.True(t, j.Skipped)
	require.Equal(t, "no judge API key: set AICHECK_E2E_JUDGE_KEY", j.Reason)
	// The deterministic checks still ran.
	require.True(t, checksOf(t, results, "AskAssistant")[0].Passed)
}
