package yamlflowsimplev2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
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
      - manual_start:
          name: Start
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

	// Compare Flow Variables counts
	require.Equal(t, len(importedData.FlowVariables), len(reImportedData.FlowVariables), "Flow Variables count mismatch")

	// Deep dive into a specific request to check body preservation
	findRequest := func(data *ioworkspace.WorkspaceBundle, name string) *mflow.NodeRequest {
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
			// Verify JSON content is preserved
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
		FlowNodes: []mflow.Node{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Custom Start", // Renamed from "Start"
				NodeKind: mflow.NODE_KIND_MANUAL_START,
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
	// Check that it is exported as a manual_start
	require.Contains(t, yamlStr, "manual_start:")

	// Re-import to check round-trip compatibility
	opts := GetDefaultOptions(workspaceID)
	reImportedData, err := ConvertSimplifiedYAML(yamlBytes, opts)
	require.NoError(t, err)

	// Verify we only have ONE node (the start node)
	require.Equal(t, 1, len(reImportedData.FlowNodes))
	// Verify the name is preserved
	require.Equal(t, "Custom Start", reImportedData.FlowNodes[0].Name)
	// Verify it is a start node
	require.Equal(t, mflow.NODE_KIND_MANUAL_START, reImportedData.FlowNodes[0].NodeKind)
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
		FlowNodes: []mflow.Node{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mflow.NODE_KIND_MANUAL_START,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "Get User",
				NodeKind: mflow.NODE_KIND_REQUEST,
			},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{
				FlowNodeID:  requestNodeID,
				HttpID:      &baseHttpID,
				DeltaHttpID: &deltaHttpID, // This is the key - points to delta
			},
		},
		FlowEdges: []mflow.Edge{
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

func TestMarshalSimplifiedYAML_WithDeltaRawBody(t *testing.T) {
	// This test verifies that when a request node has a DeltaHttpID with DeltaRawData,
	// the exporter uses ONLY the delta body (full overwrite) and preserves template variables.

	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()

	// Base HTTP request with original body
	baseHttpID := idwrap.NewNow()
	baseHttp := mhttp.HTTP{
		ID:          baseHttpID,
		WorkspaceID: workspaceID,
		Name:        "Create Product",
		Method:      "POST",
		Url:         "https://api.example.com/products",
	}

	// Base body - original static content
	baseBodyRaw := mhttp.HTTPBodyRaw{
		ID:      idwrap.NewNow(),
		HttpID:  baseHttpID,
		RawData: []byte(`{"name":"original","description":"static"}`),
	}

	// Delta HTTP request (with template syntax in body)
	deltaHttpID := idwrap.NewNow()
	deltaHttp := mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  workspaceID,
		Name:         "Create Product Delta",
		Method:       "POST",
		Url:          "https://api.example.com/products",
		IsDelta:      true,
		ParentHttpID: &baseHttpID,
	}

	// Delta body with template variables - this should fully overwrite base body
	deltaBodyContent := `{"category_id":"{{ request_5.response.body.id }}","description":"a","name":"macbook pro","options":[{"key":"b","value":"1"},{"key":"d","value":"2"}],"price":123,"tags":["{{ request_7.response.body.id }}"]}`
	deltaBodyRaw := mhttp.HTTPBodyRaw{
		ID:              idwrap.NewNow(),
		HttpID:          deltaHttpID,
		RawData:         nil, // Not used - delta uses DeltaRawData
		DeltaRawData:    []byte(deltaBodyContent),
		ParentBodyRawID: &baseBodyRaw.ID,
		IsDelta:         true,
	}

	// Build the workspace bundle
	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Delta Body Test Workspace",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowNodes: []mflow.Node{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mflow.NODE_KIND_MANUAL_START,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "Create Product",
				NodeKind: mflow.NODE_KIND_REQUEST,
			},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{
				FlowNodeID:  requestNodeID,
				HttpID:      &baseHttpID,
				DeltaHttpID: &deltaHttpID, // Points to delta
			},
		},
		FlowEdges: []mflow.Edge{
			{
				ID:       idwrap.NewNow(),
				FlowID:   flowID,
				SourceID: startNodeID,
				TargetID: requestNodeID,
			},
		},
		HTTPRequests: []mhttp.HTTP{baseHttp, deltaHttp},
		HTTPBodyRaw:  []mhttp.HTTPBodyRaw{baseBodyRaw, deltaBodyRaw},
	}

	// Export to YAML
	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)

	yamlStr := string(yamlBytes)
	t.Logf("Exported YAML:\n%s", yamlStr)

	// Verify the delta body content is in the output (full overwrite)
	require.Contains(t, yamlStr, "{{ request_5.response.body.id }}", "Delta template variable should be preserved")
	require.Contains(t, yamlStr, "{{ request_7.response.body.id }}", "Delta template variable should be preserved")
	require.Contains(t, yamlStr, "macbook pro", "Delta body content should be in output")

	// Verify the original base body content is NOT in the output (it's fully overwritten)
	require.NotContains(t, yamlStr, `"name":"original"`, "Base body should NOT be in output - delta fully overwrites")
	require.NotContains(t, yamlStr, `"description":"static"`, "Base body should NOT be in output - delta fully overwrites")
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
		FlowNodes: []mflow.Node{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mflow.NODE_KIND_MANUAL_START,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "Test Request",
				NodeKind: mflow.NODE_KIND_REQUEST,
			},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{
				FlowNodeID:  requestNodeID,
				HttpID:      &baseHttpID,
				DeltaHttpID: &deltaHttpID,
			},
		},
		FlowEdges: []mflow.Edge{
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

	// The X-Debug header should be in the output but disabled
	require.Contains(t, yamlStr, "X-Debug")
	require.Contains(t, yamlStr, "enabled: false")
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
		FlowNodes: []mflow.Node{
			{
				ID:       startNodeID,
				FlowID:   flowID,
				Name:     "Start",
				NodeKind: mflow.NODE_KIND_MANUAL_START,
			},
			{
				ID:       requestNodeID,
				FlowID:   flowID,
				Name:     "Test Request",
				NodeKind: mflow.NODE_KIND_REQUEST,
			},
		},
		FlowRequestNodes: []mflow.NodeRequest{
			{
				FlowNodeID:  requestNodeID,
				HttpID:      &baseHttpID,
				DeltaHttpID: &deltaHttpID,
			},
		},
		FlowEdges: []mflow.Edge{
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

func TestParallelStartDependency(t *testing.T) {
	// Verify that multiple nodes can depend on Start, and it is preserved in export.
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	nodeAID := idwrap.NewNow()
	nodeBID := idwrap.NewNow()

	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Parallel Start",
		},
		Flows: []mflow.Flow{
			{ID: flowID, WorkspaceID: workspaceID, Name: "Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: nodeAID, FlowID: flowID, Name: "A", NodeKind: mflow.NODE_KIND_JS},
			{ID: nodeBID, FlowID: flowID, Name: "B", NodeKind: mflow.NODE_KIND_JS},
		},
		FlowJSNodes: []mflow.NodeJS{
			{FlowNodeID: nodeAID, Code: []byte("console.log('A')")},
			{FlowNodeID: nodeBID, Code: []byte("console.log('B')")},
		},
		FlowEdges: []mflow.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: nodeAID},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: nodeBID},
		},
	}

	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)

	yamlStr := string(yamlBytes)
	t.Logf("YAML:\n%s", yamlStr)

	// One of them will be first, so implicit.
	// The other MUST have explicit depends_on: [Start]
	// BUT because of the bug, the second one might miss it.

	// Check if Start is mentioned in depends_on
	// "depends_on:\n        - Start"
	// Note: depends_on might be inline or block.

	// Actually, better to Re-Import and check edges.
	opts := GetDefaultOptions(workspaceID)
	reImported, err := ConvertSimplifiedYAML(yamlBytes, opts)
	require.NoError(t, err)

	// Check Edges in ReImported
	// We expect:
	// Start -> A
	// Start -> B
	// Total 2 edges.

	// If bug exists, likely:
	// Start -> A (implicit)
	// A -> B (implicit because explicit dep on Start was lost)

	// Find nodes by name
	var rStart, rA, rB idwrap.IDWrap
	for _, n := range reImported.FlowNodes {
		switch n.Name {
		case "Start":
			rStart = n.ID
		case "A":
			rA = n.ID
		case "B":
			rB = n.ID
		}
	}

	edgeCount := 0
	aSource := idwrap.IDWrap{}
	bSource := idwrap.IDWrap{}

	for _, e := range reImported.FlowEdges {
		if e.TargetID == rA {
			aSource = e.SourceID
			edgeCount++
		}
		if e.TargetID == rB {
			bSource = e.SourceID
			edgeCount++
		}
	}

	require.Equal(t, rStart, aSource, "A should depend on Start")
	require.Equal(t, rStart, bSource, "B should depend on Start")
}

func TestParallelByDefault_Import(t *testing.T) {
	// Import YAML with 3 steps, no depends_on.
	// With an explicit Start node, disconnected nodes should remain disconnected.
	// Expected: No edges from Start to A, B, or C (they have no depends_on)
	// Only the Start node should exist in the connected graph.

	yamlStr := `
workspace_name: Parallel Import
run:
  - flow: Flow
flows:
  - name: Flow
    steps:
      - manual_start:
          name: Start
      - js:
          name: A
          code: log('A')
      - js:
          name: B
          code: log('B')
      - js:
          name: C
          code: log('C')
`

	opts := GetDefaultOptions(idwrap.NewNow())
	bundle, err := ConvertSimplifiedYAML([]byte(yamlStr), opts)
	require.NoError(t, err)

	// Find nodes
	var rStart idwrap.IDWrap
	for _, n := range bundle.FlowNodes {
		if n.Name == "Start" {
			rStart = n.ID
		}
	}

	require.NotEqual(t, idwrap.IDWrap{}, rStart, "Start node not found")

	// Check Edges
	// With the new behavior, nodes without depends_on should NOT be connected to Start.
	// They remain disconnected and will not execute.
	// This is intentional to allow "draft" or "disabled" nodes in a flow.

	edgeMap := make(map[idwrap.IDWrap][]idwrap.IDWrap) // Source -> [Target]
	for _, e := range bundle.FlowEdges {
		edgeMap[e.SourceID] = append(edgeMap[e.SourceID], e.TargetID)
	}

	// Start should have no outgoing edges (A, B, C have no depends_on and are disconnected)
	require.Empty(t, edgeMap[rStart], "Start should have no outgoing edges - nodes without depends_on should remain disconnected")
}

func TestExplicitSerial_Export(t *testing.T) {
	// Create Flow: Start -> A -> B
	// Expected Export:
	// - Start
	// - A (no depends_on, as it depends on Start)
	// - B (depends_on: [A])

	wsID := idwrap.NewNow()
	flowID := idwrap.NewNow()

	nStart := idwrap.NewNow()
	nA := idwrap.NewNow()
	nB := idwrap.NewNow()

	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{ID: wsID, Name: "Serial Export"},
		Flows:     []mflow.Flow{{ID: flowID, WorkspaceID: wsID, Name: "Flow"}},
		FlowNodes: []mflow.Node{
			{ID: nStart, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: nA, FlowID: flowID, Name: "A", NodeKind: mflow.NODE_KIND_JS},
			{ID: nB, FlowID: flowID, Name: "B", NodeKind: mflow.NODE_KIND_JS},
		},
		FlowJSNodes: []mflow.NodeJS{
			{FlowNodeID: nA, Code: []byte("log('A')")},
			{FlowNodeID: nB, Code: []byte("log('B')")},
		},
		FlowEdges: []mflow.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nStart, TargetID: nA},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: nA, TargetID: nB},
		},
	}

	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)
	yamlStr := string(yamlBytes)

	t.Logf("YAML:\n%s", yamlStr)

	// Check A:
	// A depends on Start. Start deps are HIDDEN.
	// So A should NOT have "depends_on:"
	// Finding "name: A" and checking subsequent lines is fuzzy, but let's try strict substring check.
	// If A had depends_on: Start, fail.

	// Actually, just check that B depends on A explicitly.
	require.Contains(t, yamlStr, "depends_on")
	// Since it's a single item, it might be inline "depends_on: A"
	require.True(t, strings.Contains(yamlStr, "depends_on: A") || strings.Contains(yamlStr, "- A"), "Should depend on A")

	// Ensure B definition has depends_on A
	// A simple string check: "depends_on:\n                    - A"
	// Spacing varies, but let's assume standard indentation or use regex if needed.
	// For now, simple containment is a good smoke test.

	// Verify A DOES depend on Start explicitly
	require.True(t, strings.Contains(yamlStr, "depends_on: Start") || strings.Contains(yamlStr, "- Start"), "Should contain explicit Start dependency")
}

// TestMarshalSimplifiedYAML_AllNodeTypes_RoundTrip tests that all node types
// (JS, For, ForEach, Condition) are properly preserved during export/import cycles.
// This is a comprehensive test to catch bugs where node implementations are dropped.
func TestMarshalSimplifiedYAML_AllNodeTypes_RoundTrip(t *testing.T) {
	// YAML with all supported node types
	sourceYAML := `
workspace_name: All Node Types Test
flows:
  - name: Complete Flow
    variables:
      - name: counter
        value: "0"
      - name: items
        value: "[1, 2, 3]"
      - name: apiKey
        value: "secret123"
    steps:
      - manual_start:
          name: Start
      - js:
          name: Init Script
          code: |
            // Multi-line JavaScript code
            const config = { debug: true };
            console.log("Initializing...");
            return { initialized: true, timestamp: Date.now() };
          depends_on: Start
      - if:
          name: Check Init
          condition: "{{Init Script.response.initialized}} == true"
          depends_on: Init Script
      - for:
          name: Retry Loop
          iter_count: "3"
          depends_on: Check Init
      - for_each:
          name: Process Items
          items: "{{items}}"
          depends_on: Retry Loop
      - js:
          name: Final Script
          code: |
            // Final processing
            return { done: true, count: context.iteration };
          depends_on: Process Items
`

	// 1. Import original YAML
	workspaceID := idwrap.NewNow()
	opts := GetDefaultOptions(workspaceID)

	importedData, err := ConvertSimplifiedYAML([]byte(sourceYAML), opts)
	require.NoError(t, err)

	// Verify initial import has all node types
	require.Len(t, importedData.Flows, 1, "Should have 1 flow")
	require.Len(t, importedData.FlowVariables, 3, "Should have 3 flow variables")

	// Count node types from initial import
	initialJSCount := len(importedData.FlowJSNodes)
	initialIfCount := len(importedData.FlowConditionNodes)
	initialForCount := len(importedData.FlowForNodes)
	initialForEachCount := len(importedData.FlowForEachNodes)

	require.Equal(t, 2, initialJSCount, "Should have 2 JS nodes")
	require.Equal(t, 1, initialIfCount, "Should have 1 condition node")
	require.Equal(t, 1, initialForCount, "Should have 1 for node")
	require.Equal(t, 1, initialForEachCount, "Should have 1 foreach node")

	// 2. Export to YAML
	exportedYAML, err := MarshalSimplifiedYAML(importedData)
	require.NoError(t, err)

	t.Logf("Exported YAML:\n%s", string(exportedYAML))

	// 3. Re-import the exported YAML
	reImportedData, err := ConvertSimplifiedYAML(exportedYAML, opts)
	require.NoError(t, err, "Re-import should succeed")

	// 4. Verify all node implementation counts match
	require.Equal(t, initialJSCount, len(reImportedData.FlowJSNodes),
		"JS node count should match after round-trip")
	require.Equal(t, initialIfCount, len(reImportedData.FlowConditionNodes),
		"Condition node count should match after round-trip")
	require.Equal(t, initialForCount, len(reImportedData.FlowForNodes),
		"For node count should match after round-trip")
	require.Equal(t, initialForEachCount, len(reImportedData.FlowForEachNodes),
		"ForEach node count should match after round-trip")
	require.Equal(t, len(importedData.FlowVariables), len(reImportedData.FlowVariables),
		"Flow variables count should match after round-trip")

	// 5. Verify content preservation - find nodes by name and check content

	// Helper to find node by name
	findNodeByName := func(data *ioworkspace.WorkspaceBundle, name string) *mflow.Node {
		for i := range data.FlowNodes {
			if data.FlowNodes[i].Name == name {
				return &data.FlowNodes[i]
			}
		}
		return nil
	}

	// Helper to find JS node by flow node ID
	findJSNode := func(data *ioworkspace.WorkspaceBundle, nodeID idwrap.IDWrap) *mflow.NodeJS {
		for i := range data.FlowJSNodes {
			if data.FlowJSNodes[i].FlowNodeID == nodeID {
				return &data.FlowJSNodes[i]
			}
		}
		return nil
	}

	// Helper to find condition node by flow node ID
	findConditionNode := func(data *ioworkspace.WorkspaceBundle, nodeID idwrap.IDWrap) *mflow.NodeIf {
		for i := range data.FlowConditionNodes {
			if data.FlowConditionNodes[i].FlowNodeID == nodeID {
				return &data.FlowConditionNodes[i]
			}
		}
		return nil
	}

	// Helper to find for node by flow node ID
	findForNode := func(data *ioworkspace.WorkspaceBundle, nodeID idwrap.IDWrap) *mflow.NodeFor {
		for i := range data.FlowForNodes {
			if data.FlowForNodes[i].FlowNodeID == nodeID {
				return &data.FlowForNodes[i]
			}
		}
		return nil
	}

	// Helper to find foreach node by flow node ID
	findForEachNode := func(data *ioworkspace.WorkspaceBundle, nodeID idwrap.IDWrap) *mflow.NodeForEach {
		for i := range data.FlowForEachNodes {
			if data.FlowForEachNodes[i].FlowNodeID == nodeID {
				return &data.FlowForEachNodes[i]
			}
		}
		return nil
	}

	// Verify "Init Script" JS node content
	initScriptNode := findNodeByName(reImportedData, "Init Script")
	require.NotNil(t, initScriptNode, "Should find 'Init Script' node")
	initScriptJS := findJSNode(reImportedData, initScriptNode.ID)
	require.NotNil(t, initScriptJS, "Should find JS implementation for 'Init Script'")
	require.Contains(t, string(initScriptJS.Code), "Initializing", "JS code should contain expected content")
	require.Contains(t, string(initScriptJS.Code), "Date.now()", "JS code should preserve function calls")

	// Verify "Final Script" JS node content
	finalScriptNode := findNodeByName(reImportedData, "Final Script")
	require.NotNil(t, finalScriptNode, "Should find 'Final Script' node")
	finalScriptJS := findJSNode(reImportedData, finalScriptNode.ID)
	require.NotNil(t, finalScriptJS, "Should find JS implementation for 'Final Script'")
	require.Contains(t, string(finalScriptJS.Code), "done: true", "JS code should contain expected content")

	// Verify "Check Init" condition node content
	checkInitNode := findNodeByName(reImportedData, "Check Init")
	require.NotNil(t, checkInitNode, "Should find 'Check Init' node")
	checkInitIf := findConditionNode(reImportedData, checkInitNode.ID)
	require.NotNil(t, checkInitIf, "Should find condition implementation for 'Check Init'")
	require.NotEmpty(t, checkInitIf.Condition, "Condition should not be empty")

	// Verify "Retry Loop" for node content
	retryLoopNode := findNodeByName(reImportedData, "Retry Loop")
	require.NotNil(t, retryLoopNode, "Should find 'Retry Loop' node")
	retryLoopFor := findForNode(reImportedData, retryLoopNode.ID)
	require.NotNil(t, retryLoopFor, "Should find for implementation for 'Retry Loop'")
	require.NotEmpty(t, retryLoopFor.IterCount, "IterCount should not be empty")

	// Verify "Process Items" foreach node content
	processItemsNode := findNodeByName(reImportedData, "Process Items")
	require.NotNil(t, processItemsNode, "Should find 'Process Items' node")
	processItemsForEach := findForEachNode(reImportedData, processItemsNode.ID)
	require.NotNil(t, processItemsForEach, "Should find foreach implementation for 'Process Items'")
	require.Contains(t, processItemsForEach.IterExpression, "items", "ForEach should reference items variable")

	// Verify flow variables
	varNames := make(map[string]string)
	for _, v := range reImportedData.FlowVariables {
		varNames[v.Name] = v.Value
	}
	require.Equal(t, "0", varNames["counter"], "counter variable should be preserved")
	require.Equal(t, "[1, 2, 3]", varNames["items"], "items variable should be preserved")
	require.Equal(t, "secret123", varNames["apiKey"], "apiKey variable should be preserved")

	t.Log("All node types round-trip test passed")
}

// TestJSNodeCodePreservation specifically tests that JS code with special characters
// and multi-line content is preserved exactly through export/import.
func TestJSNodeCodePreservation(t *testing.T) {
	// Test various JS code patterns that might break during serialization
	testCases := []struct {
		name     string
		code     string
		contains []string // strings that must be present after round-trip
	}{
		{
			name: "Multi-line with comments",
			code: `// Line comment
/* Block comment */
const x = 1;
return x;`,
			contains: []string{"// Line comment", "/* Block comment */", "const x = 1"},
		},
		{
			name: "Special characters",
			code: `const msg = "Hello \"world\"";
const path = 'C:\\Users\\test';
const template = ` + "`${name}`" + `;
return msg;`,
			contains: []string{`Hello`, `world`, "return msg"},
		},
		{
			name: "Unicode and emoji",
			code: `const greeting = "こんにちは";
const emoji = "🚀";
return { greeting, emoji };`,
			contains: []string{"こんにちは", "🚀", "greeting"},
		},
		{
			name: "Complex logic",
			code: `async function process(data) {
    const result = await fetch(data.url);
    if (result.ok) {
        return result.json();
    }
    throw new Error('Failed');
}
return process(input);`,
			contains: []string{"async function", "await fetch", "throw new Error"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wsID := idwrap.NewNow()
			flowID := idwrap.NewNow()
			startNodeID := idwrap.NewNow()
			jsNodeID := idwrap.NewNow()

			bundle := &ioworkspace.WorkspaceBundle{
				Workspace: mworkspace.Workspace{ID: wsID, Name: "JS Preservation Test"},
				Flows:     []mflow.Flow{{ID: flowID, WorkspaceID: wsID, Name: "Flow"}},
				FlowNodes: []mflow.Node{
					{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
					{ID: jsNodeID, FlowID: flowID, Name: "Script", NodeKind: mflow.NODE_KIND_JS},
				},
				FlowJSNodes: []mflow.NodeJS{
					{FlowNodeID: jsNodeID, Code: []byte(tc.code)},
				},
				FlowEdges: []mflow.Edge{
					{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: jsNodeID},
				},
			}

			// Export
			yamlBytes, err := MarshalSimplifiedYAML(bundle)
			require.NoError(t, err)

			// Re-import
			opts := GetDefaultOptions(wsID)
			reImported, err := ConvertSimplifiedYAML(yamlBytes, opts)
			require.NoError(t, err)

			// Find the JS node
			require.Len(t, reImported.FlowJSNodes, 1, "Should have 1 JS node")
			reImportedCode := string(reImported.FlowJSNodes[0].Code)

			// Verify all expected content is preserved
			for _, expected := range tc.contains {
				require.Contains(t, reImportedCode, expected,
					"Code should contain '%s' after round-trip", expected)
			}
		})
	}
}

func TestMarshalSimplifiedYAML_WithAINodes(t *testing.T) {
	// Test that AI nodes (AI Agent, AI Provider, AI Memory) are correctly exported
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	memoryNodeID := idwrap.NewNow()
	toolNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	bundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "AI Export Test",
		},
		Flows: []mflow.Flow{
			{ID: flowID, WorkspaceID: workspaceID, Name: "AI Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: aiNodeID, FlowID: flowID, Name: "MyAI", NodeKind: mflow.NODE_KIND_AI},
			{ID: providerNodeID, FlowID: flowID, Name: "GPTProvider", NodeKind: mflow.NODE_KIND_AI_PROVIDER},
			{ID: memoryNodeID, FlowID: flowID, Name: "Memory", NodeKind: mflow.NODE_KIND_AI_MEMORY},
			{ID: toolNodeID, FlowID: flowID, Name: "SearchTool", NodeKind: mflow.NODE_KIND_JS},
		},
		FlowAINodes: []mflow.NodeAI{
			{FlowNodeID: aiNodeID, Prompt: "Analyze this data: {{ input }}", MaxIterations: 5},
		},
		FlowAIProviderNodes: []mflow.NodeAiProvider{
			{
				FlowNodeID:   providerNodeID,
				CredentialID: &credentialID,
				Model:        mflow.AiModelGpt52,
			},
		},
		FlowAIMemoryNodes: []mflow.NodeMemory{
			{FlowNodeID: memoryNodeID, MemoryType: mflow.AiMemoryTypeWindowBuffer, WindowSize: 10},
		},
		FlowJSNodes: []mflow.NodeJS{
			{FlowNodeID: toolNodeID, Code: []byte("return search()")},
		},
		FlowEdges: []mflow.Edge{
			// Start -> AI
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: aiNodeID},
			// AI -> Provider (AI Provider edge)
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: aiNodeID, TargetID: providerNodeID, SourceHandler: mflow.HandleAiProvider},
			// AI -> Memory (AI Memory edge)
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: aiNodeID, TargetID: memoryNodeID, SourceHandler: mflow.HandleAiMemory},
			// AI -> Tool (AI Tools edge)
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: aiNodeID, TargetID: toolNodeID, SourceHandler: mflow.HandleAiTools},
		},
		Credentials: []mcredential.Credential{
			{ID: credentialID, WorkspaceID: workspaceID, Name: "my-openai-key", Kind: mcredential.CREDENTIAL_KIND_OPENAI},
		},
	}

	yamlBytes, err := MarshalSimplifiedYAML(bundle)
	require.NoError(t, err)

	yamlStr := string(yamlBytes)
	t.Logf("Exported YAML:\n%s", yamlStr)

	// Verify AI node is exported with provider, memory, and tools references
	require.Contains(t, yamlStr, "ai:")
	require.Contains(t, yamlStr, "name: MyAI")
	require.Contains(t, yamlStr, "prompt:")
	require.Contains(t, yamlStr, "Analyze this data")
	require.Contains(t, yamlStr, "max_iterations: 5")
	require.Contains(t, yamlStr, "provider: GPTProvider")
	require.Contains(t, yamlStr, "memory: Memory")
	require.Contains(t, yamlStr, "tools:")
	require.Contains(t, yamlStr, "SearchTool")

	// Verify AI Provider is exported
	require.Contains(t, yamlStr, "ai_provider:")
	require.Contains(t, yamlStr, "name: GPTProvider")
	require.Contains(t, yamlStr, "model: gpt-5.2")
	require.Contains(t, yamlStr, "credential: my-openai-key")

	// Verify AI Memory is exported
	require.Contains(t, yamlStr, "ai_memory:")
	require.Contains(t, yamlStr, "name: Memory")
	require.Contains(t, yamlStr, "type: window_buffer")
	require.Contains(t, yamlStr, "window_size: 10")

	// Verify credentials section is generated with real credential name and env placeholder
	require.Contains(t, yamlStr, "credentials:")
	require.Contains(t, yamlStr, "name: my-openai-key")
	require.Contains(t, yamlStr, "type: openai")
	require.Contains(t, yamlStr, "{{ #env:MY_OPENAI_KEY_TOKEN }}")
}

func TestMarshalSimplifiedYAML_AIRoundTrip(t *testing.T) {
	// Test full round-trip: Export -> Import -> Export and verify consistency
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	memoryNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	temp := float32(0.7)
	maxTokens := int32(1024)

	originalBundle := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "AI RoundTrip Test",
		},
		Flows: []mflow.Flow{
			{ID: flowID, WorkspaceID: workspaceID, Name: "AI Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mflow.NODE_KIND_MANUAL_START},
			{ID: aiNodeID, FlowID: flowID, Name: "Agent", NodeKind: mflow.NODE_KIND_AI},
			{ID: providerNodeID, FlowID: flowID, Name: "Provider", NodeKind: mflow.NODE_KIND_AI_PROVIDER},
			{ID: memoryNodeID, FlowID: flowID, Name: "Memory", NodeKind: mflow.NODE_KIND_AI_MEMORY},
		},
		FlowAINodes: []mflow.NodeAI{
			{FlowNodeID: aiNodeID, Prompt: "Hello {{ name }}", MaxIterations: 3},
		},
		FlowAIProviderNodes: []mflow.NodeAiProvider{
			{FlowNodeID: providerNodeID, CredentialID: &credentialID, Model: mflow.AiModelClaudeSonnet45, Temperature: &temp, MaxTokens: &maxTokens},
		},
		FlowAIMemoryNodes: []mflow.NodeMemory{
			{FlowNodeID: memoryNodeID, MemoryType: mflow.AiMemoryTypeWindowBuffer, WindowSize: 5},
		},
		FlowEdges: []mflow.Edge{
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: startNodeID, TargetID: aiNodeID},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: aiNodeID, TargetID: providerNodeID, SourceHandler: mflow.HandleAiProvider},
			{ID: idwrap.NewNow(), FlowID: flowID, SourceID: aiNodeID, TargetID: memoryNodeID, SourceHandler: mflow.HandleAiMemory},
		},
		// Real credential metadata (secrets are never exported)
		Credentials: []mcredential.Credential{
			{ID: credentialID, WorkspaceID: workspaceID, Name: "my-anthropic-key", Kind: mcredential.CREDENTIAL_KIND_ANTHROPIC},
		},
	}

	// Export
	yamlBytes, err := MarshalSimplifiedYAML(originalBundle)
	require.NoError(t, err)
	t.Logf("Exported YAML:\n%s", string(yamlBytes))

	// Import
	opts := GetDefaultOptions(workspaceID)
	reimported, err := ConvertSimplifiedYAML(yamlBytes, opts)
	require.NoError(t, err)

	// Verify AI nodes were reimported
	require.Len(t, reimported.FlowAINodes, 1, "Should have 1 AI node")
	require.Equal(t, "Hello {{ name }}", reimported.FlowAINodes[0].Prompt)
	require.Equal(t, int32(3), reimported.FlowAINodes[0].MaxIterations)

	require.Len(t, reimported.FlowAIProviderNodes, 1, "Should have 1 AI Provider node")
	require.Equal(t, mflow.AiModelClaudeSonnet45, reimported.FlowAIProviderNodes[0].Model)
	require.NotNil(t, reimported.FlowAIProviderNodes[0].Temperature)
	require.InDelta(t, 0.7, *reimported.FlowAIProviderNodes[0].Temperature, 0.01)
	require.NotNil(t, reimported.FlowAIProviderNodes[0].MaxTokens)
	require.Equal(t, int32(1024), *reimported.FlowAIProviderNodes[0].MaxTokens)

	require.Len(t, reimported.FlowAIMemoryNodes, 1, "Should have 1 AI Memory node")
	require.Equal(t, mflow.AiMemoryTypeWindowBuffer, reimported.FlowAIMemoryNodes[0].MemoryType)
	require.Equal(t, int32(5), reimported.FlowAIMemoryNodes[0].WindowSize)

	// Verify edges - should have AI Provider and AI Memory edges
	var hasProviderEdge, hasMemoryEdge bool
	for _, e := range reimported.FlowEdges {
		if e.SourceHandler == mflow.HandleAiProvider {
			hasProviderEdge = true
		}
		if e.SourceHandler == mflow.HandleAiMemory {
			hasMemoryEdge = true
		}
	}
	require.True(t, hasProviderEdge, "Should have AI Provider edge")
	require.True(t, hasMemoryEdge, "Should have AI Memory edge")

	t.Log("Round-trip successful!")
}
