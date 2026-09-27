package yamlflowsimplev2

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// apiRecorderFixture is written by the API Recorder Chrome extension's own
// exporter (apps/api-recorder-extension/test/yamlflow.test.ts). Importing it
// here with the real parser keeps the extension's YAML export and the format
// Studio and the CLI read from drifting apart.
var apiRecorderFixture = filepath.Join(
	"..", "..", "..", "..", "..",
	"apps", "api-recorder-extension", "test", "fixtures", "recording.yamlflow.yaml",
)

func TestAPIRecorderExtensionExport(t *testing.T) {
	data, err := os.ReadFile(apiRecorderFixture)
	require.NoError(t, err, "extension fixture missing; run `pnpm nx run api-recorder-extension:test`")

	bundle, err := ConvertSimplifiedYAML(data, GetDefaultOptions(idwrap.NewNow()))
	require.NoError(t, err, "extension export must import cleanly")

	require.Equal(t, "API Recorder", bundle.Workspace.Name)
	require.Len(t, bundle.Flows, 1)
	require.Equal(t, "Recorded flow", bundle.Flows[0].Name)

	type request struct{ method, url string }
	requests := map[string]request{}
	httpNames := map[idwrap.IDWrap]string{}
	for _, r := range bundle.HTTPRequests {
		requests[r.Name] = request{r.Method, r.Url}
		httpNames[r.ID] = r.Name
	}
	require.Equal(t, map[string]request{
		"get_v1_users":    {"GET", "https://api.example.com/v1/users"},
		"post_v1_users":   {"POST", "https://api.example.com/v1/users"},
		"get_v1_users_42": {"GET", "https://api.example.com/v1/users/42"},
		"get_v1_search":   {"GET", "https://api.example.com/v1/search"},
		"get_v1_users_2":  {"GET", "https://api.example.com/v1/users"},
	}, requests)

	// Headers, query params, body and assertions land on the right requests
	headers := map[string]map[string]string{}
	for _, h := range bundle.HTTPHeaders {
		name := httpNames[h.HttpID]
		if headers[name] == nil {
			headers[name] = map[string]string{}
		}
		headers[name][h.Key] = h.Value
	}
	require.Equal(t, map[string]string{
		"Accept":        "application/json",
		"Authorization": "Bearer demo-token",
		"Content-Type":  "application/json",
	}, headers["post_v1_users"])

	var searchTags []string
	for _, p := range bundle.HTTPSearchParams {
		if httpNames[p.HttpID] == "get_v1_search" {
			require.Equal(t, "tag", p.Key)
			searchTags = append(searchTags, p.Value)
		}
	}
	sort.Strings(searchTags)
	require.Equal(t, []string{"a", "b"}, searchTags)

	var body string
	for _, b := range bundle.HTTPBodyRaw {
		if httpNames[b.HttpID] == "post_v1_users" {
			raw, err := compress.Decompress(b.RawData, compress.CompressType(b.CompressionType))
			require.NoError(t, err)
			body = string(raw)
		}
	}
	require.JSONEq(t, `{"name":"Ada Lovelace","email":"ada@example.com"}`, body)

	asserts := map[string]string{}
	for _, a := range bundle.HTTPAsserts {
		asserts[httpNames[a.HttpID]] = a.Value
	}
	require.Equal(t, "response.status == 201", asserts["post_v1_users"])
	require.NotContains(t, asserts, "get_v1_users_42", "non-2xx recordings get no status assertion")

	// Start + five request nodes chained in recording order
	require.Len(t, bundle.FlowNodes, 6)
	require.Len(t, bundle.FlowRequestNodes, 5)
	require.Len(t, bundle.FlowEdges, 5)

	// And it survives a Studio/CLI re-export
	_, err = MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)
}
