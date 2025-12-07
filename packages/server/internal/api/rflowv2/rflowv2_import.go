//nolint:revive // exported
package rflowv2

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/translate/tcurlv2"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
)

// ImportYAMLFlow imports a YAML flow definition into the workspace
func (s *FlowServiceV2RPC) ImportYAMLFlow(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*ImportResults, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Import using the v2 workspace import service
	results, err := s.workspaceImportService.ImportWorkspaceFromYAML(ctx, data, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to import YAML flow: %w", err))
	}

	return results, nil
}

// ImportYAMLFlowSimple imports a YAML flow with simple options
func (s *FlowServiceV2RPC) ImportYAMLFlowSimple(
	ctx context.Context,
	data []byte,
	workspaceID idwrap.IDWrap,
) (*ImportResults, error) {
	// This is a simplified version that just delegates to ImportYAMLFlow
	return s.ImportYAMLFlow(ctx, data, workspaceID)
}

// ParseYAMLFlow parses YAML flow data without importing it
func (s *FlowServiceV2RPC) ParseYAMLFlow(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*ioworkspace.WorkspaceBundle, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Create conversion options
	opts := yamlflowsimplev2.GetDefaultOptions(workspaceID)
	opts.IsDelta = false
	opts.GenerateFiles = true

	// Parse the YAML data
	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(data, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse YAML flow: %w", err))
	}

	return resolved, nil
}

// DetectFlowFormat detects the format of flow data (YAML, JSON, curl, etc.)
func (s *FlowServiceV2RPC) DetectFlowFormat(ctx context.Context, data []byte) (string, error) {
	// Try to detect if it's YAML
	dataStr := string(data)
	trimmedData := strings.TrimSpace(dataStr)

	// Check for curl command first (most specific)
	if strings.HasPrefix(trimmedData, "curl ") ||
		strings.Contains(dataStr, "\ncurl ") ||
		strings.Contains(dataStr, " curl ") {
		return "curl", nil
	}

	// Simple YAML detection - check for common YAML patterns
	if strings.Contains(dataStr, "flows:") ||
		strings.Contains(dataStr, "workspace_name:") ||
		strings.Contains(dataStr, "requests:") ||
		strings.Contains(dataStr, "run:") ||
		strings.Contains(dataStr, "- name:") ||
		strings.Contains(dataStr, "steps:") {
		return "yaml", nil
	}

	// Check if it's JSON
	if strings.HasPrefix(trimmedData, "{") ||
		strings.HasPrefix(trimmedData, "[") {
		return "json", nil
	}

	return "unknown", nil
}

// ValidateYAMLFlow validates YAML flow data without importing
func (s *FlowServiceV2RPC) ValidateYAMLFlow(ctx context.Context, data []byte) error {
	// Parse the YAML data to validate structure
	var yamlFormat yamlflowsimplev2.YamlFlowFormatV2
	if err := yaml.Unmarshal(data, &yamlFormat); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid YAML format: %w", err))
	}

	// Validate the YAML structure
	if err := yamlFormat.Validate(); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("YAML validation failed: %w", err))
	}

	return nil
}

// ImportCurlCommand imports a curl command into the workspace
func (s *FlowServiceV2RPC) ImportCurlCommand(ctx context.Context, curlData []byte, workspaceID idwrap.IDWrap) (*ImportResults, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Convert curl command to modern HTTP models
	curlOpts := tcurlv2.ConvertCurlOptions{
		WorkspaceID: workspaceID,
		Filename:    "curl_request",
	}

	resolved, err := tcurlv2.ConvertCurl(string(curlData), curlOpts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse curl command: %w", err))
	}

	// Create a simple YAML structure from the curl command for import
	// This allows us to reuse the existing import infrastructure
	simpleYAML := map[string]interface{}{
		"workspace_name": "Imported from Curl",
		"requests": []map[string]interface{}{
			{
				"name":        resolved.HTTP.Name,
				"method":      resolved.HTTP.Method,
				"url":         resolved.HTTP.Url,
				"description": "Imported from curl command",
			},
		},
		"flows": []map[string]interface{}{
			{
				"name": "Curl Import Flow",
				"steps": []map[string]interface{}{
					{
						"type":    "request",
						"request": resolved.HTTP.Name,
					},
				},
			},
		},
	}

	// Convert to YAML bytes
	yamlData, err := yaml.Marshal(simpleYAML)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert curl to YAML: %w", err))
	}

	// Use the existing YAML import functionality
	return s.ImportYAMLFlow(ctx, yamlData, workspaceID)
}

// ParseCurlCommand parses a curl command without importing it
func (s *FlowServiceV2RPC) ParseCurlCommand(ctx context.Context, curlData []byte, workspaceID idwrap.IDWrap) (*tcurlv2.CurlResolvedV2, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Parse the curl command
	curlOpts := tcurlv2.ConvertCurlOptions{
		WorkspaceID: workspaceID,
		Filename:    "curl_request",
	}

	resolved, err := tcurlv2.ConvertCurl(string(curlData), curlOpts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse curl command: %w", err))
	}

	return resolved, nil
}

// ParseFlowData parses flow data without importing it (supports YAML, JSON, curl)
func (s *FlowServiceV2RPC) ParseFlowData(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (interface{}, error) {
	// Detect format first
	format, err := s.DetectFlowFormat(ctx, data)
	if err != nil {
		return nil, err
	}

	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	switch format {
	case "yaml":
		return s.ParseYAMLFlow(ctx, data, workspaceID)
	case "json":
		// For JSON, try to parse as YAML first (YAML is a superset of JSON)
		return s.ParseYAMLFlow(ctx, data, workspaceID)
	case "curl":
		return s.ParseCurlCommand(ctx, data, workspaceID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported format: %s", format))
	}
}
