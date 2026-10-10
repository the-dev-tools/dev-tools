package rimportv2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/import/v1"
)

// exportWorkspaceYAML exports the fixture's workspace the way the desktop app's YAML export does.
func exportWorkspaceYAML(t *testing.T, fixture *integrationTestFixture) string {
	t.Helper()
	svc := ioworkspace.New(fixture.base.Queries, fixture.logger)
	bundle, err := svc.Export(fixture.ctx, ioworkspace.ExportOptions{WorkspaceID: fixture.workspaceID, IncludeHTTP: true, IncludeFlows: true})
	require.NoError(t, err)
	out, err := yamlflowsimplev2.MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)
	return string(out)
}

const desktopAIChecksYAML = `
workspace_name: Acme
judge:
  provider: openai
  base_url: http://localhost:11434/v1
quality:
  fail_below: 90%
flows:
  - name: Assistant
    steps:
      - request:
          name: Chat
          method: POST
          url: https://app.test/api/chat
          stream: anthropic
          stream_timeout_ms: 15000
          expect:
            max_ttft_ms: 800
            judge:
              criteria: Names where to cancel.
              output: '{{ response.text }}'
              min_score: 4
`

// A workspace imported from YAML in the desktop app exports its stream: and expect: again.
func TestDesktopYAMLImportKeepsStreamAndExpect(t *testing.T) {
	fixture := newIntegrationTestFixture(t)
	_, err := fixture.rpc.Import(fixture.ctx, connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "AI checks",
		Data:        []byte(desktopAIChecksYAML),
	}))
	require.NoError(t, err)

	text := exportWorkspaceYAML(t, fixture)
	for _, want := range []string{"stream: anthropic", "stream_timeout_ms: 15000", "max_ttft_ms: 800",
		"criteria: Names where to cancel.", "output: '{{ response.text }}'", "min_score: 4",
		"provider: openai", "base_url: http://localhost:11434/v1", "fail_below: 90%"} {
		require.Contains(t, text, want)
	}
	_, err = yamlflowsimplev2.ConvertSimplifiedYAML([]byte(text), yamlflowsimplev2.GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err, text)
}

// A HAR with streaming entries, imported in the desktop app, exports stream: <preset>.
func TestDesktopHARImportExportsStreamPresets(t *testing.T) {
	fixture := newIntegrationTestFixture(t)
	harData, err := os.ReadFile(filepath.Join("..", "..", "..", "pkg", "translate", "harv2", "testdata", "streams.har"))
	require.NoError(t, err)
	_, err = fixture.rpc.Import(fixture.ctx, connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "Streams",
		Data:        harData,
		DomainData:  []*apiv1.ImportDomainData{{Enabled: true, Domain: "app.test", Variable: "BASE_URL"}},
	}))
	require.NoError(t, err)

	text := exportWorkspaceYAML(t, fixture)
	for preset, count := range map[string]int{"openai": 1, "anthropic": 1, "vercel-ai": 2, "sse": 3} {
		require.Equal(t, count, strings.Count(text, "stream: "+preset+"\n"), "stream: %s\n%s", preset, text)
	}
	_, err = yamlflowsimplev2.ConvertSimplifiedYAML([]byte(text), yamlflowsimplev2.GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err, text)
}
