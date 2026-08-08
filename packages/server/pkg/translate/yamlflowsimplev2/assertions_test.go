package yamlflowsimplev2

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// TestHTTPAssertionsImported locks in the fix for a bug where HTTP request
// assertions were parsed from YAML but never converted into mhttp.HTTPAssert
// records, so they silently vanished on import (they always exported fine,
// since WorkspaceBundle.HTTPAsserts simply stayed empty). GraphQL assertions
// never had this bug (see processGraphQLStructStep in converter_node.go).
func TestHTTPAssertionsImported(t *testing.T) {
	workspaceID := idwrap.NewNow()

	yamlData := `
workspace_name: Assertion Test
flows:
  - name: AssertFlow
    steps:
      - manual_start:
          name: Start
      - request:
          name: CheckStatus
          depends_on: Start
          method: GET
          url: https://api.example.com/status
          assertions:
            - expression: response.status == 200
              enabled: true
            - expression: response.body.ok == true
              enabled: false
`

	opts := GetDefaultOptions(workspaceID)
	result, err := ConvertSimplifiedYAML([]byte(yamlData), opts)
	if err != nil {
		t.Fatalf("failed to convert: %v", err)
	}

	if len(result.HTTPRequests) != 1 {
		t.Fatalf("expected 1 HTTP request, got %d", len(result.HTTPRequests))
	}
	httpID := result.HTTPRequests[0].ID

	if len(result.HTTPAsserts) != 2 {
		t.Fatalf("expected 2 HTTP asserts bound to the request, got %d", len(result.HTTPAsserts))
	}

	byValue := make(map[string]mhttp.HTTPAssert, len(result.HTTPAsserts))
	for _, a := range result.HTTPAsserts {
		if a.HttpID.Compare(httpID) != 0 {
			t.Errorf("assert %q bound to wrong HttpID: got %s, want %s", a.Value, a.HttpID.String(), httpID.String())
		}
		byValue[a.Value] = a
	}

	enabledAssert, ok := byValue["response.status == 200"]
	if !ok {
		t.Fatal("missing assertion 'response.status == 200'")
	}
	if !enabledAssert.Enabled {
		t.Error("expected 'response.status == 200' assertion to be enabled")
	}

	disabledAssert, ok := byValue["response.body.ok == true"]
	if !ok {
		t.Fatal("missing assertion 'response.body.ok == true'")
	}
	if disabledAssert.Enabled {
		t.Error("expected 'response.body.ok == true' assertion to be disabled")
	}
}
