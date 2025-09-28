package yamlflowsimple

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
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

	exported, err := ExportYamlFlowYAML(context.Background(), workspace, nil)
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

func TestBuildRequestDefinitionsDedupsAssertions(t *testing.T) {
	flowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	deltaEndpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	workspace := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{ID: idwrap.NewNow(), Name: "workspace"},
		Flows: []mflow.Flow{
			{ID: flowID, Name: "flow"},
		},
		FlowNodes: []mnnode.MNode{
			{ID: nodeID, FlowID: flowID, Name: "request_1", NodeKind: mnnode.NODE_KIND_REQUEST},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:       nodeID,
				EndpointID:       &endpointID,
				ExampleID:        &exampleID,
				DeltaEndpointID:  &deltaEndpointID,
				DeltaExampleID:   &deltaExampleID,
				HasRequestConfig: true,
			},
		},
		Endpoints: []mitemapi.ItemApi{
			{ID: endpointID, Name: "request_1", Method: "GET", Url: "https://example.dev"},
			{ID: deltaEndpointID, Name: "request_1 (delta)", Method: "GET", Url: "https://example.dev", DeltaParentID: &endpointID},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{ID: exampleID, ItemApiID: endpointID, Name: "request_1"},
			{ID: deltaExampleID, ItemApiID: deltaEndpointID, Name: "request_1 (delta)", VersionParentID: &exampleID},
		},
		ExampleAsserts: []massert.Assert{
			{ID: idwrap.NewNow(), ExampleID: exampleID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}}, Enable: true},
			{ID: idwrap.NewNow(), ExampleID: exampleID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}}, Enable: true},
			{ID: idwrap.NewNow(), ExampleID: deltaExampleID, Condition: mcondition.Condition{Comparisons: mcondition.Comparison{Expression: "response.status == 201"}}, Enable: true},
		},
	}

	requests, err := buildRequestDefinitions(context.Background(), workspace, nil)
	if err != nil {
		t.Fatalf("buildRequestDefinitions failed: %v", err)
	}
	req, ok := requests["request_1"]
	if !ok {
		t.Fatalf("expected request_1 in definitions")
	}

	assertionsAny, ok := req["assertions"].([]map[string]any)
	if !ok {
		t.Fatalf("expected assertions slice in request definition")
	}

	if len(assertionsAny) != 1 {
		t.Fatalf("expected 1 assertion after dedup, got %d", len(assertionsAny))
	}

	if expr, _ := assertionsAny[0]["expression"].(string); expr != "response.status == 201" {
		t.Fatalf("unexpected assertion expression %q", expr)
	}
}
