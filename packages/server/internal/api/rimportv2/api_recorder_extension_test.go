package rimportv2

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/import/v1"
)

// TestAPIRecorderExtensionYAML_StudioImport sends the API Recorder Chrome
// extension's YAML export through Studio's import RPC (format detection
// included). The fixture is produced by the extension's own exporter, see
// apps/api-recorder-extension/test/yamlflow.test.ts.
func TestAPIRecorderExtensionYAML_StudioImport(t *testing.T) {
	fixture := newIntegrationTestFixture(t)

	yamlData, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "..", "..",
		"apps", "api-recorder-extension", "test", "fixtures", "recording.yamlflow.yaml",
	))
	require.NoError(t, err)

	detection := NewFormatDetector().DetectFormat(yamlData)
	require.Equal(t, FormatYAML, detection.Format, "Studio must detect the export as a YAML flow")

	resp, err := fixture.rpc.Import(fixture.ctx, connect.NewRequest(&apiv1.ImportRequest{
		WorkspaceId: fixture.workspaceID.Bytes(),
		Name:        "api-recorder-flow.yaml",
		Data:        yamlData,
	}))
	require.NoError(t, err)
	require.Equal(t, apiv1.ImportMissingDataKind_IMPORT_MISSING_DATA_KIND_UNSPECIFIED, resp.Msg.MissingData)

	flows, err := fixture.services.FlowService.GetFlowsByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	require.Len(t, flows, 1)
	require.Equal(t, "Recorded flow", flows[0].Name)

	httpReqs, err := fixture.services.HttpService.GetByWorkspaceID(fixture.ctx, fixture.workspaceID)
	require.NoError(t, err)
	var names []string
	for _, r := range httpReqs {
		if !r.IsDelta {
			names = append(names, r.Name)
		}
	}
	sort.Strings(names)
	require.Equal(t, []string{"get_v1_search", "get_v1_users", "get_v1_users_2", "get_v1_users_42", "post_v1_users"}, names)
}
