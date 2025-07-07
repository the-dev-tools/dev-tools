package workflowsimple

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mworkspace"
)

func TestExportCleanWithOptions_NoMagicVariables(t *testing.T) {
	// Create workspace data with JWT token in header
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()
	
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		// No flow variables defined
		FlowVariables: []mflowvariable.FlowVariable{},
		Endpoints: []mitemapi.ItemApi{
			{
				ID:           endpointID,
				CollectionID: workspaceID,
				Name:        "Test API",
				Method:      "GET",
				Url:         "https://api.example.com/data",
			},
		},
		Examples: []mitemapiexample.ItemApiExample{
			{
				ID:        exampleID,
				ItemApiID: endpointID,
				Name:      "Example",
			},
		},
		ExampleHeaders: []mexampleheader.Header{
			{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				HeaderKey: "Authorization",
				Value:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
				Enable:    true,
			},
		},
	}
	
	// Export with token replacement enabled
	options := ExportOptions{
		ReplaceTokens:        true,
		FilterBrowserHeaders: false,
		TokenVariableName:    "token",
	}
	
	exported, err := ExportWorkflowCleanWithOptions(workspaceData, options)
	require.NoError(t, err)
	
	// Parse the exported YAML
	var result map[string]any
	err = yaml.Unmarshal(exported, &result)
	require.NoError(t, err)
	
	// Check that there are no requests (no flow nodes)
	requests, ok := result["requests"]
	if ok {
		requestsArray, ok := requests.([]any)
		require.True(t, ok)
		require.Len(t, requestsArray, 0) // No requests because no flow nodes
	}
	
	// Check flows
	flows, ok := result["flows"].([]any)
	require.True(t, ok)
	require.Len(t, flows, 1)
	
	flow := flows[0].(map[string]any)
	
	// Verify no variables were magically created
	variables, ok := flow["variables"]
	assert.False(t, ok, "No variables should be created automatically")
	assert.Nil(t, variables)
}

func TestExportCleanWithOptions_PreservesExistingVariables(t *testing.T) {
	// Create workspace data with existing flow variables
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	
	workspaceData := &ioworkspace.WorkspaceData{
		Workspace: mworkspace.Workspace{
			ID:   workspaceID,
			Name: "Test Workspace",
		},
		Flows: []mflow.Flow{
			{
				ID:          flowID,
				WorkspaceID: workspaceID,
				Name:        "Test Flow",
			},
		},
		FlowVariables: []mflowvariable.FlowVariable{
			{
				ID:      idwrap.NewNow(),
				FlowID:  flowID,
				Name:    "base_url",
				Value:   "https://api.example.com",
				Enabled: true,
			},
			{
				ID:      idwrap.NewNow(),
				FlowID:  flowID,
				Name:    "api_key",
				Value:   "secret123",
				Enabled: true,
			},
		},
	}
	
	// Export with default options
	exported, err := ExportWorkflowCleanWithOptions(workspaceData, DefaultExportOptions)
	require.NoError(t, err)
	
	// Parse the exported YAML
	var result map[string]any
	err = yaml.Unmarshal(exported, &result)
	require.NoError(t, err)
	
	// Check flows
	flows, ok := result["flows"].([]any)
	require.True(t, ok)
	require.Len(t, flows, 1)
	
	flow := flows[0].(map[string]any)
	
	// Verify existing variables are preserved
	variables, ok := flow["variables"].([]any)
	require.True(t, ok)
	require.Len(t, variables, 2)
	
	// Check variable values
	var foundBaseUrl, foundApiKey bool
	for _, v := range variables {
		varMap := v.(map[string]any)
		name := varMap["name"].(string)
		value := varMap["value"].(string)
		
		if name == "base_url" {
			assert.Equal(t, "https://api.example.com", value)
			foundBaseUrl = true
		} else if name == "api_key" {
			assert.Equal(t, "secret123", value)
			foundApiKey = true
		}
	}
	
	assert.True(t, foundBaseUrl, "base_url variable should be present")
	assert.True(t, foundApiKey, "api_key variable should be present")
}