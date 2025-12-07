package yamlflowsimplev2

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mvar"
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

	// Process Environments
	// Map to track env ID by name for workspace linking
	envNameMap := make(map[string]idwrap.IDWrap)

	for _, yamlEnv := range yamlFormat.Environments {
		envID := idwrap.NewNow()
		env := menv.Env{
			ID:          envID,
			WorkspaceID: opts.WorkspaceID,
			Name:        yamlEnv.Name,
			Description: yamlEnv.Description,
		}
		result.Environments = append(result.Environments, env)
		envNameMap[env.Name] = envID

		// Variables
		// Since map iteration order is random, we sort keys to ensure deterministic order
		var keys []string
		for k := range yamlEnv.Variables {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			val := yamlEnv.Variables[k]
			variable := mvar.Var{
				ID:      idwrap.NewNow(),
				EnvID:   envID,
				VarKey:  k,
				Value:   val,
				Enabled: true,
				Order:   float64(i + 1),
			}
			result.EnvironmentVars = append(result.EnvironmentVars, variable)
		}
	}

	// Link Workspace Environments
	if yamlFormat.ActiveEnvironment != "" {
		if id, ok := envNameMap[yamlFormat.ActiveEnvironment]; ok {
			result.Workspace.ActiveEnv = id
		}
	} else if len(result.Environments) > 0 {
		// Auto-select environment when active_environment is not specified:
		// 1. First, look for an environment named "default"
		// 2. If not found, use the first environment in the list
		if defaultID, ok := envNameMap["default"]; ok {
			result.Workspace.ActiveEnv = defaultID
		} else {
			// Use the first environment
			result.Workspace.ActiveEnv = result.Environments[0].ID
		}
	}

	if yamlFormat.GlobalEnvironment != "" {
		if id, ok := envNameMap[yamlFormat.GlobalEnvironment]; ok {
			result.Workspace.GlobalEnv = id
		}
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
