package yamlflowsimplev2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
)

// Real flow files from repos (recorder output and a hand-written demo flow with cleanup:).
var textMergeFiles = map[string]string{
	"admin-catalog-crud.yamlflow.yaml":    "GetTags",
	"items-crud.yamlflow.yaml":            "GetItem",
	"login-products-orders.yamlflow.yaml": "GetProducts",
}

func readTextMergeFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "textmerge", name))
	require.NoError(t, err)
	return data
}

func convertForMerge(t *testing.T, data []byte) *ioworkspace.WorkspaceBundle {
	t.Helper()
	bundle, err := ConvertSimplifiedYAML(data, ConvertOptionsV2{WorkspaceID: idwrap.NewNow(), GenerateFiles: true})
	require.NoError(t, err)
	return bundle
}

func marshalForMerge(t *testing.T, bundle *ioworkspace.WorkspaceBundle) []byte {
	t.Helper()
	out, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)
	return out
}

// requireSameFlows checks that merged, imported again, has the flows of current.
func requireSameFlows(t *testing.T, merged, current []byte) {
	t.Helper()
	again := marshalForMerge(t, convertForMerge(t, merged))
	changed, err := FlowFileChanged(current, again)
	require.NoError(t, err)
	require.False(t, changed, "merged file imports to different flows:\n%s", merged)
}

func changedLines(a, b string) []int {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	if len(al) != len(bl) {
		return nil
	}
	var out []int
	for i := range al {
		if al[i] != bl[i] {
			out = append(out, i)
		}
	}
	return out
}

func TestMergeFlowFileUnchangedKeepsSource(t *testing.T) {
	for name := range textMergeFiles {
		t.Run(name, func(t *testing.T) {
			src := readTextMergeFile(t, name)
			clean := marshalForMerge(t, convertForMerge(t, src))

			merged, err := MergeFlowFile(src, clean, clean)
			require.NoError(t, err)
			require.Equal(t, string(src), string(merged))
		})
	}
}

func TestMergeFlowFileEditChangesOnlyThatLine(t *testing.T) {
	for name, request := range textMergeFiles {
		t.Run(name, func(t *testing.T) {
			// A comment inside the flow, besides the file's own header comment.
			src := []byte(strings.Replace(string(readTextMergeFile(t, name)),
				"      - request:\n", "      # first request\n      - request:\n", 1))
			bundle := convertForMerge(t, src)
			clean := marshalForMerge(t, bundle)

			found := false
			for i := range bundle.HTTPRequests {
				if bundle.HTTPRequests[i].Name == request {
					bundle.HTTPRequests[i].Url += "?edited=1"
					found = true
				}
			}
			require.True(t, found)
			current := marshalForMerge(t, bundle)

			changed, err := FlowFileChanged(clean, current)
			require.NoError(t, err)
			require.True(t, changed)

			merged, err := MergeFlowFile(src, clean, current)
			require.NoError(t, err)

			diff := changedLines(string(src), string(merged))
			require.Len(t, diff, 1, "merged:\n%s", merged)
			line := strings.Split(string(merged), "\n")[diff[0]]
			require.Contains(t, line, "url:")
			require.Contains(t, line, "?edited=1")
			require.Contains(t, string(merged), "# first request")
			requireSameFlows(t, merged, current)
		})
	}
}

func TestMergeFlowFileKeepsFileOnlySections(t *testing.T) {
	src := readTextMergeFile(t, "items-crud.yamlflow.yaml")
	bundle := convertForMerge(t, src)
	// The database keeps neither cleanup: nor load:, so its exports have neither.
	bundle.FlowCleanups = nil
	bundle.LoadScenarios = nil
	clean := marshalForMerge(t, bundle)

	for i := range bundle.HTTPRequests {
		if bundle.HTTPRequests[i].Name == "PostItems" {
			bundle.HTTPRequests[i].Method = "PUT"
		}
	}
	current := marshalForMerge(t, bundle)

	merged, err := MergeFlowFile(src, clean, current)
	require.NoError(t, err)
	require.Contains(t, string(merged), "# Demo flow for the Stresseur demo target")
	require.Contains(t, string(merged), "    cleanup:\n      - request:\n          name: DeleteItem")
	require.Len(t, changedLines(string(src), string(merged)), 1)
	require.Contains(t, string(merged), "method: PUT")
}

func TestMergeFlowFileAddsAndRemovesSteps(t *testing.T) {
	src := string(readTextMergeFile(t, "items-crud.yamlflow.yaml"))
	clean := marshalForMerge(t, convertForMerge(t, []byte(src)))

	getItem := `      - request:
          name: GetItem
          depends_on: PostItems
          method: GET
          url: '{{ #env:STRESSEUR_BASE_URL }}/api/items/{{ PostItems.response.body.id }}'
          headers:
            Authorization: 'Bearer {{ PostLogin.response.body.token }}'
          assertions:
            - response.status == 200
`
	require.Contains(t, src, getItem)

	t.Run("removed", func(t *testing.T) {
		edited := strings.Replace(src, getItem, "", 1)
		current := marshalForMerge(t, convertForMerge(t, []byte(edited)))

		merged, err := MergeFlowFile([]byte(src), clean, current)
		require.NoError(t, err)
		require.Equal(t, edited, string(merged))
	})

	t.Run("added", func(t *testing.T) {
		listItems := `      - request:
          name: ListItems
          depends_on: GetItem
          method: GET
          url: '{{ #env:STRESSEUR_BASE_URL }}/api/items'
`
		edited := strings.Replace(src, getItem, getItem+listItems, 1)
		current := marshalForMerge(t, convertForMerge(t, []byte(edited)))

		merged, err := MergeFlowFile([]byte(src), clean, current)
		require.NoError(t, err)
		// Every source line is still there, in order, and the new step follows GetItem.
		require.True(t, strings.HasPrefix(string(merged), strings.SplitAfter(src, getItem)[0]), "merged:\n%s", merged)
		require.Contains(t, string(merged), "name: ListItems")
		require.True(t, strings.HasSuffix(string(merged), strings.SplitAfter(src, getItem)[1]), "merged:\n%s", merged)
		requireSameFlows(t, merged, current)
	})
}

func TestMergeFlowFileNewFlow(t *testing.T) {
	src := string(readTextMergeFile(t, "admin-catalog-crud.yamlflow.yaml"))
	clean := marshalForMerge(t, convertForMerge(t, []byte(src)))

	edited := strings.Replace(src, "load:\n", `  - name: health
    steps:
      - manual_start:
          name: Start
      - request:
          name: GetHealth
          depends_on: Start
          method: GET
          url: '{{ #env:STRESSEUR_BASE_URL }}/api/health'
load:
`, 1)
	current := marshalForMerge(t, convertForMerge(t, []byte(edited)))

	merged, err := MergeFlowFile([]byte(src), clean, current)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(merged), strings.Split(src, "load:\n")[0]))
	require.True(t, strings.HasSuffix(string(merged), "load:\n"+strings.Split(src, "load:\n")[1]))
	requireSameFlows(t, merged, current)
}

func TestMergeFlowFileKeepsJSONBodyForm(t *testing.T) {
	src := string(readTextMergeFile(t, "admin-catalog-crud.yamlflow.yaml"))
	clean := marshalForMerge(t, convertForMerge(t, []byte(src)))

	edited := strings.Replace(src, "              price: 119\n", "              price: 99\n", 1)
	require.NotEqual(t, src, edited)
	current := marshalForMerge(t, convertForMerge(t, []byte(edited)))

	merged, err := MergeFlowFile([]byte(src), clean, current)
	require.NoError(t, err)
	require.Equal(t, edited, string(merged))
}

func TestFlowFileChangedIgnoresPositions(t *testing.T) {
	bundle := convertForMerge(t, readTextMergeFile(t, "login-products-orders.yamlflow.yaml"))
	clean := marshalForMerge(t, bundle)
	for i := range bundle.FlowNodes {
		bundle.FlowNodes[i].PositionX += 40
		bundle.FlowNodes[i].PositionY += 25
	}
	moved := marshalForMerge(t, bundle)
	require.NotEqual(t, string(clean), string(moved))

	changed, err := FlowFileChanged(clean, moved)
	require.NoError(t, err)
	require.False(t, changed)
}
