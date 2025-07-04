package ioworkspace

// UnmarshalWorkflowYAMLSimple provides a simplified interface for unmarshaling workflow YAML
// This is a placeholder for future integration with the workflowformat package
func UnmarshalWorkflowYAMLSimple(data []byte) (*WorkspaceData, error) {
	// For now, use the existing implementation
	return UnmarshalWorkflowYAML(data)
}

// MarshalWorkflowYAMLSimple provides a simplified interface for marshaling workflow YAML
// This is a placeholder for future integration with the workflowformat package
func MarshalWorkflowYAMLSimple(workspaceData *WorkspaceData) ([]byte, error) {
	// For now, use the existing implementation
	return MarshalWorkflowYAML(workspaceData)
}