package yamlflowsimplev2

import (
	"testing"
	"strings"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

func TestMarshalSimplifiedYAML_RoundTrip(t *testing.T) {
	// 1. Define Source YAML (v2 format)
	sourceYAML := `
workspace_name: Round Trip Test
flows:
  - name: Main Flow
    variables:
      - name: baseURL
        value: https://api.example.com
    steps:
      - request:
          name: Get Users
          method: GET
          url: https://api.example.com/users
          headers:
            - name: Authorization
              value: Bearer token
      - if:
          name: Check Users
          condition: response.status == 200
          then: Create User
          else: Log Error
      - request:
          name: Create User
          method: POST
          url: https://api.example.com/users
          body:
            type: json
            json:
              name: John Doe
              role: admin
          depends_on: 
            - Check Users
      - request:
          name: Log Error
          method: POST
          url: https://logging.example.com/error
          body:
            type: raw
            raw: "Failed to get users"
          depends_on:
            - Check Users
`

	// 2. Import (Convert YAML -> Resolved Data)
	workspaceID := idwrap.NewNow()
	opts := GetDefaultOptions(workspaceID)
	
	importedData, err := ConvertSimplifiedYAML([]byte(sourceYAML), opts)
	if err != nil {
		t.Fatalf("Initial Import failed: %v", err)
	}

	// 3. Export (Resolved Data -> YAML)
	exportedYAML, err := MarshalSimplifiedYAML(importedData)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 4. Import Again (Exported YAML -> Resolved Data 2)
	// We need new IDs for the second import to avoid confusion, or just reuse same logic
	// The IDs will be new generated ones anyway.
	reImportedData, err := ConvertSimplifiedYAML(exportedYAML, opts)
	if err != nil {
		t.Fatalf("Re-Import failed on exported YAML: %v\nYAML Was:\n%s", err, string(exportedYAML))
	}

	// 5. Compare Structures
	// We compare counts and key names since IDs will differ.
	
	// Compare Flow counts
	if len(importedData.Flows) != len(reImportedData.Flows) {
		t.Errorf("Flow count mismatch: %d vs %d", len(importedData.Flows), len(reImportedData.Flows))
	}

	// Compare Node counts
	if len(importedData.FlowNodes) != len(reImportedData.FlowNodes) {
		t.Errorf("Node count mismatch: %d vs %d", len(importedData.FlowNodes), len(reImportedData.FlowNodes))
	}

	// Compare Request Node counts
	if len(importedData.FlowRequestNodes) != len(reImportedData.FlowRequestNodes) {
		t.Errorf("Request Node count mismatch: %d vs %d", len(importedData.FlowRequestNodes), len(reImportedData.FlowRequestNodes))
	}

	// Compare Condition Node counts
	if len(importedData.FlowConditionNodes) != len(reImportedData.FlowConditionNodes) {
		t.Errorf("Condition Node count mismatch: %d vs %d", len(importedData.FlowConditionNodes), len(reImportedData.FlowConditionNodes))
	}

	// Deep dive into a specific request to check body preservation
	findRequest := func(data *ioworkspace.WorkspaceBundle, name string) *mnrequest.MNRequest {
		var nodeID idwrap.IDWrap
		found := false
		for _, n := range data.FlowNodes {
			if n.Name == name {
				nodeID = n.ID
				found = true
				break
			}
		}
		if !found { return nil }
		for _, rn := range data.FlowRequestNodes {
			if rn.FlowNodeID == nodeID {
				return &rn
			}
		}
		return nil
	}

	findHTTP := func(data *ioworkspace.WorkspaceBundle, id idwrap.IDWrap) *mhttp.HTTP {
		for _, h := range data.HTTPRequests {
			if h.ID == id {
				return &h
			}
		}
		return nil
	}

	// Check "Create User" body
	reqNode := findRequest(reImportedData, "Create User")
	if reqNode == nil {
		t.Fatalf("Could not find 'Create User' node in re-imported data")
	}
	
	httpReq := findHTTP(reImportedData, *reqNode.HttpID)
	if httpReq == nil {
		t.Fatalf("Could not find HTTP request for 'Create User'")
	}

	// Check Body Raw
	foundBody := false
	for _, b := range reImportedData.HTTPBodyRaw {
		if b.HttpID == httpReq.ID {
			foundBody = true
			if b.ContentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", b.ContentType)
			}
			// Verify JSON content
			expectedFragment := "John Doe"
			if !strings.Contains(string(b.RawData), expectedFragment) {
				t.Errorf("Body JSON missing content '%s', got: %s", expectedFragment, string(b.RawData))
			}
		}
	}
	if !foundBody {
		t.Errorf("Missing body for 'Create User'")
	}
}
