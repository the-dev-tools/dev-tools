package workflowsimple

import (
	"fmt"
	"strings"
	"the-dev-tools/server/pkg/ioworkspace"
)

// DebugExportData prints diagnostic information about the workspace data
func DebugExportData(workspaceData *ioworkspace.WorkspaceData) string {
	var output strings.Builder
	
	output.WriteString("=== DEBUG EXPORT DATA ===\n\n")
	
	// Show endpoints
	output.WriteString("ENDPOINTS:\n")
	for _, endpoint := range workspaceData.Endpoints {
		output.WriteString(fmt.Sprintf("  ID: %s, Name: %s, Hidden: %v, URL: %s\n", 
			endpoint.ID.String(), endpoint.Name, endpoint.Hidden, endpoint.Url))
		if endpoint.DeltaParentID != nil {
			output.WriteString(fmt.Sprintf("    -> Delta of: %s\n", endpoint.DeltaParentID.String()))
		}
	}
	
	// Show examples
	output.WriteString("\nEXAMPLES:\n")
	for _, example := range workspaceData.Examples {
		output.WriteString(fmt.Sprintf("  ID: %s, Name: %s, EndpointID: %s\n",
			example.ID.String(), example.Name, example.ItemApiID.String()))
	}
	
	// Show request nodes
	output.WriteString("\nREQUEST NODES:\n")
	for _, reqNode := range workspaceData.FlowRequestNodes {
		nodeName := "?"
		for _, node := range workspaceData.FlowNodes {
			if node.ID == reqNode.FlowNodeID {
				nodeName = node.Name
				break
			}
		}
		output.WriteString(fmt.Sprintf("  Node: %s\n", nodeName))
		if reqNode.EndpointID != nil {
			output.WriteString(fmt.Sprintf("    EndpointID: %s\n", reqNode.EndpointID.String()))
		}
		if reqNode.DeltaEndpointID != nil {
			output.WriteString(fmt.Sprintf("    DeltaEndpointID: %s\n", reqNode.DeltaEndpointID.String()))
		}
		if reqNode.ExampleID != nil {
			output.WriteString(fmt.Sprintf("    ExampleID: %s\n", reqNode.ExampleID.String()))
		}
		if reqNode.DeltaExampleID != nil {
			output.WriteString(fmt.Sprintf("    DeltaExampleID: %s\n", reqNode.DeltaExampleID.String()))
		}
	}
	
	// Sample headers
	output.WriteString("\nSAMPLE HEADERS (first 5):\n")
	count := 0
	for _, header := range workspaceData.ExampleHeaders {
		if count >= 5 {
			break
		}
		output.WriteString(fmt.Sprintf("  ExampleID: %s, Key: %s, Value: %s\n",
			header.ExampleID.String(), header.HeaderKey, header.Value))
		if header.DeltaParentID != nil {
			output.WriteString(fmt.Sprintf("    -> Delta of: %s\n", header.DeltaParentID.String()))
		}
		count++
	}
	
	return output.String()
}