package yamlflowsimple

import (
	"testing"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/ioworkspace"
)

const roundTripYAML = `workspace_name: Round Trip Workspace
requests:
  - name: get-user
    method: GET
    url: https://example.dev/users
    assertions:
      - expression: response.status == 200
        enabled: true
flows:
  - name: sample
    steps:
      - request:
          name: Fetch User
          use_request: get-user
`

func TestYamlFlowRoundTripWithAssertions(t *testing.T) {
	workspace, err := ImportYamlFlowYAML([]byte(roundTripYAML))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if !workspaceHasAssertion(workspace, "response.status == 200") {
		t.Fatalf("imported workspace missing expected assertion")
	}

	exported, err := ExportYamlFlowYAML(workspace)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	var exportedDoc map[string]any
	if err := yaml.Unmarshal(exported, &exportedDoc); err != nil {
		t.Fatalf("failed to unmarshal exported yaml: %v", err)
	}

	assertions := collectAllRequestAssertions(exportedDoc)
	if len(assertions) == 0 {
		t.Fatalf("exported yaml missing assertions:\n%s", string(exported))
	}

	reimported, err := ImportYamlFlowYAML(exported)
	if err != nil {
		t.Fatalf("re-import failed: %v", err)
	}

	if !workspaceHasAssertion(reimported, "response.status == 200") {
		t.Fatalf("re-imported workspace missing expected assertion")
	}
}

func workspaceHasAssertion(workspace *ioworkspace.WorkspaceData, expression string) bool {
	for _, a := range workspace.ExampleAsserts {
		if a.Condition.Comparisons.Expression == expression {
			return true
		}
	}
	return false
}

func collectAllRequestAssertions(doc map[string]any) []map[string]any {
	requestsAny, ok := doc["requests"].([]any)
	if !ok {
		return nil
	}

	var result []map[string]any

	for _, reqAny := range requestsAny {
		req, ok := reqAny.(map[string]any)
		if !ok {
			continue
		}
		if assertions, ok := req["assertions"].([]any); ok {
			for _, assertionAny := range assertions {
				if assertionMap, ok := assertionAny.(map[string]any); ok {
					result = append(result, assertionMap)
				}
			}
		}
	}
	return result
}
