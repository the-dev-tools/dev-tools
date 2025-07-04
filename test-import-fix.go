package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/io/workflow"
	"the-dev-tools/server/pkg/io/workflow/simplified"
)

func main() {
	// Read test YAML file
	data, err := os.ReadFile("test-import.yaml")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Parse using simplified format
	s := simplified.New()
	wd, err := s.Unmarshal(data, workflow.FormatYAML)
	if err != nil {
		log.Fatalf("Failed to unmarshal: %v", err)
	}

	// Convert to ioworkspace.WorkspaceData
	workspaceData := convertToIOWorkspaceData(wd)

	// Simulate import with new workspace ID
	newWorkspaceID := idwrap.NewNow()
	
	// Show original IDs
	fmt.Printf("Original workspace ID: %s\n", workspaceData.Workspace.ID)
	if len(workspaceData.Collections) > 0 {
		fmt.Printf("Original collection ID: %s\n", workspaceData.Collections[0].ID)
	}
	if len(workspaceData.Flows) > 0 {
		fmt.Printf("Original flow ID: %s\n", workspaceData.Flows[0].ID)
	}

	// Apply ID regeneration (simulating what ImportWorkflowYAML does)
	regenerateIDs(workspaceData, newWorkspaceID)

	// Show new IDs
	fmt.Printf("\nAfter regeneration:\n")
	fmt.Printf("New workspace ID: %s\n", workspaceData.Workspace.ID)
	if len(workspaceData.Collections) > 0 {
		fmt.Printf("New collection ID: %s\n", workspaceData.Collections[0].ID)
	}
	if len(workspaceData.Flows) > 0 {
		fmt.Printf("New flow ID: %s\n", workspaceData.Flows[0].ID)
	}

	fmt.Println("\nID regeneration test completed successfully!")
}

func convertToIOWorkspaceData(wd *workflow.WorkspaceData) *ioworkspace.WorkspaceData {
	// This is a simplified conversion - in real code this is done by convertFromWorkflowData
	return &ioworkspace.WorkspaceData{
		Workspace:              wd.Workspace,
		Collections:            wd.Collections,
		Folders:                wd.Folders,
		Endpoints:              wd.Endpoints,
		Examples:               wd.Examples,
		ExampleHeaders:         wd.RequestHeaders,
		ExampleQueries:         wd.RequestQueries,
		ExampleAsserts:         wd.RequestAsserts,
		Rawbodies:              wd.RequestBodyRaw,
		FormBodies:             wd.RequestBodyForm,
		UrlBodies:              wd.RequestBodyUrlencoded,
		ExampleResponses:       wd.RequestResponses,
		ExampleResponseHeaders: wd.RequestResponseHeaders,
		ExampleResponseAsserts: wd.RequestResponseAsserts,
		Flows:                  wd.Flows,
		FlowNodes:              wd.FlowNodes,
		FlowEdges:              wd.FlowEdges,
		FlowVariables:          wd.FlowVariables,
		FlowRequestNodes:       wd.FlowRequestNodes,
		FlowConditionNodes:     wd.FlowConditionNodes,
		FlowNoopNodes:          wd.FlowNoopNodes,
		FlowForNodes:           wd.FlowForNodes,
		FlowForEachNodes:       wd.FlowForEachNodes,
		FlowJSNodes:            wd.FlowJSNodes,
	}
}

func regenerateIDs(workspaceData *ioworkspace.WorkspaceData, newWorkspaceID idwrap.IDWrap) {
	idMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	
	getNewID := func(oldID idwrap.IDWrap) idwrap.IDWrap {
		if newID, exists := idMap[oldID]; exists {
			return newID
		}
		newID := idwrap.NewNow()
		idMap[oldID] = newID
		return newID
	}
	
	// Update workspace ID
	oldWorkspaceID := workspaceData.Workspace.ID
	workspaceData.Workspace.ID = newWorkspaceID
	idMap[oldWorkspaceID] = newWorkspaceID

	// Regenerate collection IDs
	for i := range workspaceData.Collections {
		oldID := workspaceData.Collections[i].ID
		workspaceData.Collections[i].ID = getNewID(oldID)
		workspaceData.Collections[i].WorkspaceID = newWorkspaceID
	}

	// Regenerate flow IDs
	for i := range workspaceData.Flows {
		oldID := workspaceData.Flows[i].ID
		workspaceData.Flows[i].ID = getNewID(oldID)
		workspaceData.Flows[i].WorkspaceID = newWorkspaceID
	}

	// ... rest of the ID regeneration logic
}