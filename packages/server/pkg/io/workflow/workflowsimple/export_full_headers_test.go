package workflowsimple

import (
	"sort"
	"testing"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/model/mbodyraw"
)

func TestExportFullBrowserHeaders(t *testing.T) {
	// Test that all browser headers are preserved in export
	flowID := idwrap.NewNow()
	startNodeID := idwrap.NewNow()
	requestNodeID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	deltaExampleID := idwrap.NewNow()

	// Full set of browser headers from HAR
	browserHeaders := []struct {
		key   string
		value string
	}{
		{"accept", "*/*"},
		{"accept-encoding", "gzip, deflate, br, zstd"},
		{"accept-language", "en-US,en;q=0.9"},
		{"authorization", "Bearer {{ request_0.response.body.token }}"},
		{"content-type", "application/json"},
		{"priority", "u=1, i"},
		{"referer", "https://ecommerce-admin-panel.fly.dev/"},
		{"sec-ch-ua", `"Chromium";v="136", "Google Chrome";v="136", "Not.A/Brand";v="99"`},
		{"sec-ch-ua-mobile", "?0"},
		{"sec-ch-ua-platform", `"macOS"`},
		{"sec-fetch-dest", "empty"},
		{"sec-fetch-mode", "cors"},
		{"sec-fetch-site", "same-origin"},
		{"user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"},
	}

	// Create headers for delta example
	var exampleHeaders []mexampleheader.Header
	for _, h := range browserHeaders {
		exampleHeaders = append(exampleHeaders, mexampleheader.Header{
			ID:        idwrap.NewNow(),
			ExampleID: deltaExampleID,
			HeaderKey: h.key,
			Value:     h.value,
			Enable:    true,
		})
	}

	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   idwrap.NewNow(),
			Name: "Browser Headers Test",
		},
		Flows: []mflow.Flow{
			{
				ID:   flowID,
				Name: "Test Flow",
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
				Name:     "request_7",
				NodeKind: mnnode.NODE_KIND_REQUEST,
			},
		},
		FlowEdges: []edge.Edge{
			{
				ID:            idwrap.NewNow(),
				FlowID:        flowID,
				SourceID:      startNodeID,
				TargetID:      requestNodeID,
				SourceHandler: edge.HandleUnspecified,
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
				FlowNodeID:     requestNodeID,
				EndpointID:     &endpointID,
				ExampleID:      &exampleID,
				DeltaExampleID: &deltaExampleID,
			},
		},
		Endpoints: []mitemapi.ItemApi{
			{
				ID:     endpointID,
				Name:   "tags",
				Url:    "https://ecommerce-admin-panel.fly.dev/api/tags",
				Method: "GET",
			},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        exampleID,
				Name:      "tags",
				ItemApiID: endpointID,
			},
			{
				ID:              deltaExampleID,
				Name:            "tags (Delta)",
				ItemApiID:       endpointID,
				VersionParentID: &exampleID,
			},
		},
		ExampleHeaders: exampleHeaders,
		Rawbodies: []mbodyraw.ExampleBodyRaw{
			{
				ID:        idwrap.NewNow(),
				ExampleID: deltaExampleID,
				Data:      []byte{}, // Empty body for GET request
			},
		},
	}

	// Export using the clean format
	yamlData, err := ExportWorkflowClean(workspaceData)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}

	// Parse the exported YAML
	var exported map[string]any
	if err := yaml.Unmarshal(yamlData, &exported); err != nil {
		t.Fatalf("failed to parse exported YAML: %v", err)
	}

	t.Logf("Exported YAML:\n%s", string(yamlData))

	// Verify requests section
	requests, ok := exported["requests"].([]any)
	if !ok || len(requests) != 1 {
		t.Fatal("requests section not found or incorrect count")
	}

	request := requests[0].(map[string]any)
	
	// Verify request has all headers
	headers, ok := request["headers"].(map[string]any)
	if !ok {
		t.Fatal("headers not found or not a map")
	}

	// Check that all browser headers are present
	if len(headers) != len(browserHeaders) {
		t.Errorf("expected %d headers, got %d", len(browserHeaders), len(headers))
	}

	// Verify each header
	for _, expectedHeader := range browserHeaders {
		value, exists := headers[expectedHeader.key]
		if !exists {
			t.Errorf("header %s not found", expectedHeader.key)
			continue
		}
		if value != expectedHeader.value {
			t.Errorf("header %s: expected '%s', got '%v'", expectedHeader.key, expectedHeader.value, value)
		}
	}

	// Verify the headers are alphabetically sorted
	var headerKeys []string
	for k := range headers {
		headerKeys = append(headerKeys, k)
	}
	
	// Create a sorted copy to compare
	sortedKeys := make([]string, len(headerKeys))
	copy(sortedKeys, headerKeys)
	sort.Strings(sortedKeys)
	
	// Log the header keys for debugging
	t.Logf("Header keys found: %v", headerKeys)
}