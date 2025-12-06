package yamlflowsimplev2

import (
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
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
      - noop:
          name: Start
          type: start
      - request:
          name: Get Users
          method: GET
          url: https://api.example.com/users
          headers:
            - name: Authorization
              value: Bearer token
          depends_on:
            - Start
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
	require.NoError(t, err)

	// 3. Export (Resolved Data -> YAML)
	exportedYAML, err := MarshalSimplifiedYAML(importedData)
	require.NoError(t, err)

	// 4. Import Again (Exported YAML -> Resolved Data 2)
	// We need new IDs for the second import to avoid confusion, or just reuse same logic
	// The IDs will be new generated ones anyway.
	reImportedData, err := ConvertSimplifiedYAML(exportedYAML, opts)
	require.NoError(t, err, "Re-Import failed on exported YAML:\n%s", string(exportedYAML))

	// 5. Compare Structures
	// We compare counts and key names since IDs will differ.

	// Compare Flow counts
	require.Equal(t, len(importedData.Flows), len(reImportedData.Flows), "Flow count mismatch")

	// Compare Node counts
	require.Equal(t, len(importedData.FlowNodes), len(reImportedData.FlowNodes), "Node count mismatch")

	// Compare Request Node counts
	require.Equal(t, len(importedData.FlowRequestNodes), len(reImportedData.FlowRequestNodes), "Request Node count mismatch")

	// Compare Condition Node counts
	require.Equal(t, len(importedData.FlowConditionNodes), len(reImportedData.FlowConditionNodes), "Condition Node count mismatch")

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
		if !found {
			return nil
		}
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
	require.NotNil(t, reqNode, "Could not find 'Create User' node in re-imported data")

	httpReq := findHTTP(reImportedData, *reqNode.HttpID)
	require.NotNil(t, httpReq, "Could not find HTTP request for 'Create User'")

	// Check Body Raw
	foundBody := false
	for _, b := range reImportedData.HTTPBodyRaw {
		if b.HttpID == httpReq.ID {
			foundBody = true
			require.Equal(t, "application/json", b.ContentType)
			// Verify JSON content
			expectedFragment := "John Doe"
			require.Contains(t, string(b.RawData), expectedFragment)
		}
	}
	require.True(t, foundBody, "Missing body for 'Create User'")
}

func TestMarshalSimplifiedYAML_WithStartNode(t *testing.T) {
	// This test verifies that the start node is exported correctly and
	// that a renamed start node is preserved in the export.

	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()

	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Start Node Test",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Renamed Start Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Custom Start", // Renamed from "Start"
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{
				FlowNodeID: startNodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
		},
	}

	// Export to YAML
	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)

	yamlStr := string(yamlBytes)
	t.Logf("Exported YAML:\n%s", yamlStr)

	// Check that the custom name appears
	require.Contains(t, yamlStr, "name: Custom Start")
	// Check that it is exported as a noop with type start
	require.Contains(t, yamlStr, "type: start")
	require.Contains(t, yamlStr, "noop:")

	// Re-import to check round-trip compatibility
	opts := GetDefaultOptions(workspaceID)
	reImportedData, err := ConvertSimplifiedYAML(yamlBytes, opts)
	require.NoError(t, err)

	// Verify we only have ONE node (the start node)
	require.Equal(t, 1, len(reImportedData.FlowNodes))
	// Verify the name is preserved
	require.Equal(t, "Custom Start", reImportedData.FlowNodes[0].Name)
	// Verify it is a start node
	require.Equal(t, 1, len(reImportedData.FlowNoopNodes))
	require.Equal(t, mnnoop.NODE_NO_OP_KIND_START, reImportedData.FlowNoopNodes[0].Type)
}

func TestMarshalSimplifiedYAML_WithDeltaOverrides(t *testing.T) {
	// This test verifies that when a request node has a DeltaHttpID,
	// the exporter merges the delta values (like template syntax) into the output.

	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	// Base HTTP request (static values)
	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Get User",
		Method:      "GET",
		Url:         "https://api.example.com/users/123",
	}

	// Delta HTTP request (with template syntax)
	deltaHttpID := idwrap.NewNow()
	deltaUrl := "https://api.example.com/users/{{ request_1.response.body.id }}"
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Get User Delta",
		Method:       "GET",
		Url:          "https://api.example.com/users/123", // Base URL
		IsDelta:      true,
		ParentHttpID: &baseHttpID,
		DeltaUrl:     &deltaUrl,
	}

	// Base header (static value)
	baseHeaderID := idwrap.NewNow()
	baseHeader := mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseHttpID,
		Key:     "Authorization",
		Value:   "Bearer static-token",
		Enabled: true,
	}

	// Delta header (with template syntax)
	deltaHeaderValue := "Bearer {{ request_1.response.body.token }}"
	deltaHeader := mhttp.HTTPHeader{
		ID:                 idwrap.NewNow(),
		HttpID:             deltaHttpID,
		Key:                "Authorization",
		Value:              "Bearer static-token", // Base value
		Enabled:            true,
		ParentHttpHeaderID: &baseHeaderID,
		IsDelta:            true,
		DeltaValue:         &deltaHeaderValue,
	}

	// Build the workspace bundle
	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Delta Test Workspace",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "Get User",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{
				FlowNodeID: startNodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:  requestNodeID,
				HttpID:      &baseHttpID,
				DeltaHttpID: &deltaHttpID, // This is the key - points to delta
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:       idwrap.NewNow(),
				FlowID:   flowID,
				SourceID: startNodeID,
				TargetID: requestNodeID,
			},
		},
		HTTPRequests: []mhttp.HTTP{baseHttp, deltaHttp},
		HTTPHeaders:  []mhttp.HTTPHeader{baseHeader, deltaHeader},
	}

	// Export to YAML
	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)

	yamlStr := string(yamlBytes)
	t.Logf("Exported YAML:\n%s", yamlStr)

	// Verify the delta URL template is in the output
	require.Contains(t, yamlStr, "{{ request_1.response.body.id }}")

	// Verify the delta header template is in the output
	require.Contains(t, yamlStr, "{{ request_1.response.body.token }}")

	// Verify the static values are NOT in the output (they should be replaced by delta)
	require.NotContains(t, yamlStr, "static-token")
}

func TestMarshalSimplifiedYAML_WithDeltaDisabledHeader(t *testing.T) {
	// Test that delta can disable a header

	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Test Request",
		Method:      "GET",
		Url:         "https://api.example.com/test",
	}

	deltaHttpID := idwrap.NewNow()
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Test Request Delta",
		Method:       "GET",
		Url:          "https://api.example.com/test",
		IsDelta:      true,
		ParentHttpID: &baseHttpID,
	}

	// Base header that should be disabled by delta
	baseHeaderID := idwrap.NewNow()
	baseHeader := mhttp.HTTPHeader{
		ID:      baseHeaderID,
		HttpID:  baseHttpID,
		Key:     "X-Debug",
		Value:   "true",
		Enabled: true,
	}

	// Delta header that disables the base header
	deltaEnabled := false
	deltaHeader := mhttp.HTTPHeader{
		ID:                 idwrap.NewNow(),
		HttpID:             deltaHttpID,
		Key:                "X-Debug",
		Value:              "true",
		Enabled:            true,
		ParentHttpHeaderID: &baseHeaderID,
		IsDelta:            true,
		DeltaEnabled:       &deltaEnabled,
	}

	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Delta Disable Test",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "Test Request",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{
				FlowNodeID: startNodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:  requestNodeID,
				HttpID:      &baseHttpID,
				DeltaHttpID: &deltaHttpID,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:       idwrap.NewNow(),
				FlowID:   flowID,
				SourceID: startNodeID,
				TargetID: requestNodeID,
			},
		},
		HTTPRequests: []mhttp.HTTP{baseHttp, deltaHttp},
		HTTPHeaders:  []mhttp.HTTPHeader{baseHeader, deltaHeader},
	}

	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)

	yamlStr := string(yamlBytes)
	t.Logf("Exported YAML:\n%s", yamlStr)

	// The X-Debug header should NOT be in the output because delta disabled it
	require.NotContains(t, yamlStr, "X-Debug")
}

func TestMarshalSimplifiedYAML_WithNewDeltaHeader(t *testing.T) {
	// Test that delta can add a new header not present in base

	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Test Request",
		Method:      "GET",
		Url:         "https://api.example.com/test",
	}

	deltaHttpID := idwrap.NewNow()
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Test Request Delta",
		Method:       "GET",
		Url:          "https://api.example.com/test",
		IsDelta:      true,
		ParentHttpID: &baseHttpID,
	}

	// New header added only in delta (no parent)
	newDeltaHeader := mhttp.HTTPHeader{
		ID:      idwrap.NewNow(),
		HttpID:  deltaHttpID,
		Key:     "X-Request-ID",
		Value:   "{{ uuid() }}",
		Enabled: true,
		IsDelta: false, // It's a new header, not a delta override
		// ParentHttpHeaderID is nil - this is a new header
	}

	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Delta New Header Test",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mnnode.MNode{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mnnode.NODE_KIND_NO_OP,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "Test Request",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{
				FlowNodeID: startNodeID,
				Type:       mnnoop.NODE_NO_OP_KIND_START,
			},
		},
		FlowRequestNodes: []mnrequest.MNRequest{
			{
				FlowNodeID:  requestNodeID,
				HttpID:      &baseHttpID,
				DeltaHttpID: &deltaHttpID,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:       idwrap.NewNow(),
				FlowID:   flowID,
				SourceID: startNodeID,
				TargetID: requestNodeID,
			},
		},
		HTTPRequests: []mhttp.HTTP{baseHttp, deltaHttp},
		HTTPHeaders:  []mhttp.HTTPHeader{newDeltaHeader},
	}

	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)

	yamlStr := string(yamlBytes)
	t.Logf("Exported YAML:\n%s", yamlStr)

	// The new header should be in the output
	require.Contains(t, yamlStr, "X-Request-ID")
	require.Contains(t, yamlStr, "{{ uuid() }}")
}
