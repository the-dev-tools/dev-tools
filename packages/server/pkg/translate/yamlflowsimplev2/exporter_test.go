package yamlflowsimplev2

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
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
		FlowNodes: []mnnode.MNode{
			{ID: startNodeID, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: nodeAID, FlowID: flowID, Name: "A", NodeKind: mnnode.NODE_KIND_JS},
			{ID: nodeBID, FlowID: flowID, Name: "B", NodeKind: mnnode.NODE_KIND_JS},
		},
		FlowNoopNodes: []mnnoop.NoopNode{
			{FlowNodeID: startNodeID, Type: mnnoop.NODE_NO_OP_KIND_START},
		},
		FlowJSNodes: []mnjs.MNJS{
			{FlowNodeID: nodeAID, Code: []byte("console.log('A')")},
			{FlowNodeID: nodeBID, Code: []byte("console.log('B')")},
		},
		FlowEdges: []edge.Edge{
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
	// Expected: Start -> A, Start -> B, Start -> C

	yamlStr := `
workspace_name: Parallel Import
run:
  - flow: Flow
flows:
  - name: Flow
    steps:
      - noop:
          name: Start
          type: start
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
	var rStart, rA, rB, rC idwrap.IDWrap
	for _, n := range bundle.FlowNodes {
		switch n.Name {
		case "Start":
			rStart = n.ID
		case "A":
			rA = n.ID
		case "B":
			rB = n.ID
		case "C":
			rC = n.ID
		}
	}

	require.NotEqual(t, idwrap.IDWrap{}, rStart, "Start node not found")

	// Check Edges
	// We expect 3 edges: Start->A, Start->B, Start->C
	// No A->B or B->C edges.

	edgeMap := make(map[idwrap.IDWrap][]idwrap.IDWrap) // Source -> [Target]
	for _, e := range bundle.FlowEdges {
		edgeMap[e.SourceID] = append(edgeMap[e.SourceID], e.TargetID)
	}

	require.Contains(t, edgeMap[rStart], rA, "Start should link to A")
	require.Contains(t, edgeMap[rStart], rB, "Start should link to B")
	require.Contains(t, edgeMap[rStart], rC, "Start should link to C")

	require.NotContains(t, edgeMap[rA], rB, "A should NOT link to B (implicit serial was implied before)")
	require.NotContains(t, edgeMap[rB], rC, "B should NOT link to C")
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
		FlowNodes: []mnnode.MNode{
			{ID: nStart, FlowID: flowID, Name: "Start", NodeKind: mnnode.NODE_KIND_NO_OP},
			{ID: nA, FlowID: flowID, Name: "A", NodeKind: mnnode.NODE_KIND_JS},
			{ID: nB, FlowID: flowID, Name: "B", NodeKind: mnnode.NODE_KIND_JS},
		},
		FlowNoopNodes: []mnnoop.NoopNode{{FlowNodeID: nStart, Type: mnnoop.NODE_NO_OP_KIND_START}},
		FlowJSNodes: []mnjs.MNJS{
			{FlowNodeID: nA, Code: []byte("log('A')")},
			{FlowNodeID: nB, Code: []byte("log('B')")},
		},
		FlowEdges: []edge.Edge{
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
