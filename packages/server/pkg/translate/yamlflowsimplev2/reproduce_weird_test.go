package yamlflowsimplev2

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func TestConvertSimplifiedYAML_FlatBody(t *testing.T) {
	workspaceID := idwrap.NewNow()

	// Testing if body fields at the top level of 'body' are correctly parsed as JSON
	yamlData := `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - request:
          name: API Test
          url: https://api.example.com/test
          body:
            title: "Test Post"
            userId: 1
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	require.NoError(t, err)
	require.Len(t, result.HTTPRequests, 1)

	// Verification: We expect a JSON body containing the fields
	require.NotEmpty(t, result.HTTPBodyRaw, "Body should not be empty")
	bodyStr := string(result.HTTPBodyRaw[0].RawData)
	require.Contains(t, bodyStr, "title")
	require.Contains(t, bodyStr, "Test Post")
	require.Contains(t, bodyStr, "userId")
}

func TestConvertSimplifiedYAML_NumericHeaders(t *testing.T) {
	workspaceID := idwrap.NewNow()

	// Testing if numeric values in headers (common in YAML) are handled correctly
	yamlData := `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - request:
          name: API Test
          url: https://api.example.com/test
          headers:
            X-Count: 123
            X-Active: true
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)

	require.NoError(t, err, "Numeric/Boolean headers should not cause error")

	foundCount := false
	foundActive := false
	for _, h := range result.HTTPHeaders {
		if h.Key == "X-Count" {
			require.Equal(t, "123", h.Value)
			foundCount = true
		}
		if h.Key == "X-Active" {
			require.Equal(t, "true", h.Value)
			foundActive = true
		}
	}
	require.True(t, foundCount, "X-Count header not found")
	require.True(t, foundActive, "X-Active header not found")
}
