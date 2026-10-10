package ioworkspace_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	yamlflowsimplev2 "github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
)

const roundTripYAML = `
workspace_name: Acme
judge:
  provider: anthropic
  api_key_env: MY_JUDGE_KEY
quality:
  fail_below: 85%
flows:
  - name: Assistant
    iterations: 3
    judge: { model: claude-sonnet-4-5 }
    steps:
      - request:
          name: AskAssistant
          method: POST
          url: https://app.test/api/assistant
          stream: openai
          stream_timeout_ms: 20000
          expect:
            schema:
              type: object
              required: [answer]
              properties:
                answer: { type: string, minLength: 20 }
            max_latency_ms: 4000
            max_ttft_ms: 800
            judge:
              criteria: Answers how to cancel.
              steps: [Check it names Settings > Billing.]
              output: '{{ response.text }}'
              min_score: 4
      - request:
          name: Plain
          depends_on: AskAssistant
          method: GET
          url: https://app.test/api/plain
  - name: Other
    steps:
      - request:
          name: Ping
          method: GET
          url: https://app.test/ping
`

// A workspace imported from YAML (new IDs, as the desktop app imports) exports the same
// stream:, expect:, judge:, quality: and iterations: again.
func TestAIChecksSurviveImportAndExport(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	queries := gen.New(db)
	wsID := idwrap.NewNow()
	require.NoError(t, queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{ID: wsID, Name: "W"}))

	bundle, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(roundTripYAML), yamlflowsimplev2.GetDefaultOptions(wsID))
	require.NoError(t, err)

	svc := ioworkspace.New(queries, nil)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = svc.Import(ctx, tx, bundle, ioworkspace.ImportOptions{WorkspaceID: wsID, ImportHTTP: true, ImportFlows: true})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	exported, err := svc.Export(ctx, ioworkspace.ExportOptions{WorkspaceID: wsID, IncludeHTTP: true, IncludeFlows: true})
	require.NoError(t, err)
	out, err := yamlflowsimplev2.MarshalSimplifiedYAML(exported)
	require.NoError(t, err)
	text := string(out)
	for _, want := range []string{"stream: openai", "stream_timeout_ms: 20000", "max_ttft_ms: 800", "max_latency_ms: 4000",
		"criteria: Answers how to cancel.", "minLength: 20", "iterations: 3",
		// The file's judge: and quality: are folded into each flow.
		"model: claude-sonnet-4-5", "api_key_env: MY_JUDGE_KEY", "fail_below: 85%"} {
		require.Contains(t, text, want)
	}
	require.Equal(t, 1, strings.Count(text, "stream: "), text)
	require.Equal(t, 2, strings.Count(text, "api_key_env: MY_JUDGE_KEY"), "both flows keep the file's judge:\n"+text)

	// The exported file imports again with the same checks.
	again, err := yamlflowsimplev2.ConvertSimplifiedYAML(out, yamlflowsimplev2.GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err, text)
	require.Len(t, again.StepExpects(), 1)
	require.Len(t, again.AllHTTPStreams(), 1)
}
