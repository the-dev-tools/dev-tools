package yamlflowsimplev2

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mworkspace"
)

// ConvertSimplifiedYAML converts simplified YAML to modern HTTP and flow models
func ConvertSimplifiedYAML(data []byte, opts ConvertOptionsV2) (*ioworkspace.WorkspaceBundle, error) {
	// Validate options
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Parse YAML to get structured data
	yamlFormat, err := parseYAMLData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate YAML structure
	if err := yamlFormat.Validate(); err != nil {
		return nil, fmt.Errorf("invalid YAML structure: %w", err)
	}

	// Validate references via utility
	if err := ValidateYAMLStructure(yamlFormat); err != nil {
		return nil, fmt.Errorf("invalid YAML semantics: %w", err)
	}

	// Initialize resolved data structure with workspace metadata
	result := &ioworkspace.WorkspaceBundle{
		Workspace: mworkspace.Workspace{
			ID:   opts.WorkspaceID,
			Name: yamlFormat.WorkspaceName,
		},
	}

	// Prepare request templates map from both Sources
	requestTemplates := make(map[string]YamlRequestDefV2)
	for k, v := range yamlFormat.RequestTemplates {
		requestTemplates[k] = v
	}
	for _, req := range yamlFormat.Requests {
		if req.Name != "" {
			requestTemplates[req.Name] = req
		}
	}

	// Process flows and generate HTTP requests
	for _, flowEntry := range yamlFormat.Flows {
		flowData, err := processFlow(flowEntry, yamlFormat.Run, requestTemplates, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to process flow '%s': %w", flowEntry.Name, err)
		}

		// Merge flow data into result
		mergeFlowData(result, flowData, opts)
	}

	// Ensure all flows have proper structure (start nodes, edges, positioning)
	if err := result.EnsureFlowStructure(); err != nil {
		return nil, fmt.Errorf("failed to ensure flow structure: %w", err)
	}

	return result, nil
}

// parseYAMLData parses YAML data into structured format
func parseYAMLData(data []byte) (*YamlFlowFormatV2, error) {
	var yamlFormat YamlFlowFormatV2
	if err := yaml.Unmarshal(data, &yamlFormat); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	return &yamlFormat, nil
}
