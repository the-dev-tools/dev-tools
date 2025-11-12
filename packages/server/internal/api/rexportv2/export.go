package rexportv2

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/suser"
)

// Interfaces

// Exporter provides export functionality for different formats
type Exporter interface {
	ExportWorkspaceData(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error)
	ExportToYAML(ctx context.Context, data *WorkspaceExportData, simplified bool) ([]byte, error)
	ExportToCurl(ctx context.Context, data *WorkspaceExportData, exampleIDs []idwrap.IDWrap) (string, error)
}

// Validator provides validation for export operations
type Validator interface {
	ValidateExportRequest(ctx context.Context, req *ExportRequest) error
	ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error
	ValidateExportFilter(ctx context.Context, filter ExportFilter) error
}


// Exporter Implementation

// SimpleExporter implements the Exporter interface using modern services
type SimpleExporter struct {
	httpService *shttp.HTTPService
	flowService *sflow.FlowService
	fileService *sfile.FileService
	storage     Storage
}

// NewExporter creates a new SimpleExporter
func NewExporter(httpService *shttp.HTTPService, flowService *sflow.FlowService, fileService *sfile.FileService) *SimpleExporter {
	return &SimpleExporter{
		httpService: httpService,
		flowService: flowService,
		fileService: fileService,
	}
}

// SetStorage sets the storage dependency (called after storage is created)
func (e *SimpleExporter) SetStorage(storage Storage) {
	e.storage = storage
}

// ExportWorkspaceData retrieves workspace data for export
func (e *SimpleExporter) ExportWorkspaceData(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error) {
	// Get workspace information
	workspace, err := e.storage.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}

	// Get flows
	flows, err := e.storage.GetFlows(ctx, workspaceID, filter.FlowIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get flows: %w", err)
	}

	// Get HTTP requests
	httpRequests, err := e.storage.GetHTTPRequests(ctx, workspaceID, filter.ExampleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTTP requests: %w", err)
	}

	// Get files (empty for now)
	var files []*FileData

	return &WorkspaceExportData{
		Workspace:    workspace,
		Flows:        flows,
		HTTPRequests: httpRequests,
		Files:        files,
	}, nil
}

// ExportToYAML exports data to YAML format
func (e *SimpleExporter) ExportToYAML(ctx context.Context, data *WorkspaceExportData, simplified bool) ([]byte, error) {
	// Create YAML structure
	yamlFormat := map[string]interface{}{
		"requests": make([]map[string]interface{}, 0),
		"flows":    make([]map[string]interface{}, 0),
		"files":    make([]map[string]interface{}, 0),
	}

	// Add workspace data if present
	if data.Workspace != nil {
		workspaceData := map[string]interface{}{
			"name": data.Workspace.Name,
		}
		if !simplified {
			workspaceData["id"] = data.Workspace.ID.String()
		}
		yamlFormat["workspace"] = workspaceData
	}

	// Convert HTTP requests
	for _, httpReq := range data.HTTPRequests {
		request := map[string]interface{}{
			"name":   httpReq.Name,
			"method": httpReq.Method,
			"url":    httpReq.Url,
		}
		if !simplified {
			if httpReq.Headers != nil {
				request["headers"] = httpReq.Headers
			}
			if httpReq.Body != "" {
				request["body"] = httpReq.Body
			}
			if httpReq.QueryParams != nil {
				request["query_params"] = httpReq.QueryParams
			}
		}
		yamlFormat["requests"] = append(yamlFormat["requests"].([]map[string]interface{}), request)
	}

	// Convert flows
	for _, flow := range data.Flows {
		flowData := map[string]interface{}{
			"name": flow.Name,
		}
		if !simplified {
			if flow.Description != "" {
				flowData["description"] = flow.Description
			}
			if flow.Variables != nil {
				flowData["variables"] = flow.Variables
			}
			if len(flow.Steps) > 0 {
				flowData["steps"] = flow.Steps
			}
		}
		yamlFormat["flows"] = append(yamlFormat["flows"].([]map[string]interface{}), flowData)
	}

	// Convert files
	for _, file := range data.Files {
		fileData := map[string]interface{}{
			"name": file.Name,
		}
		if !simplified {
			fileData["id"] = file.ID.String()
			if file.Path != "" {
				fileData["path"] = file.Path
			}
			if len(file.Data) > 0 {
				fileData["size"] = len(file.Data)
			}
		}
		yamlFormat["files"] = append(yamlFormat["files"].([]map[string]interface{}), fileData)
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(yamlFormat)
	if err != nil {
		return nil, fmt.Errorf("YAML export failed: %w", err)
	}

	return yamlData, nil
}

// ExportToCurl exports data to cURL format
func (e *SimpleExporter) ExportToCurl(ctx context.Context, data *WorkspaceExportData, exampleIDs []idwrap.IDWrap) (string, error) {
	if len(data.HTTPRequests) == 0 {
		return "# No HTTP requests to export\n", nil
	}

	var commands []string
	for _, httpReq := range data.HTTPRequests {
		var cmd strings.Builder
		cmd.WriteString(fmt.Sprintf("curl -X %s '%s'", httpReq.Method, httpReq.Url))

		// Add headers if present
		if httpReq.Headers != nil && len(httpReq.Headers) > 0 {
			for key, values := range httpReq.Headers {
				for _, value := range values {
					cmd.WriteString(fmt.Sprintf(" -H '%s: %s'", key, value))
				}
			}
		}

		// Add body if present
		if httpReq.Body != "" {
			cmd.WriteString(fmt.Sprintf(" -d '%s'", strings.ReplaceAll(httpReq.Body, "'", "'\"'\"'")))
		}

		cmd.WriteString(fmt.Sprintf(" # %s", httpReq.Name))
		commands = append(commands, cmd.String())
	}

	return strings.Join(commands, "\n\n"), nil
}

// Validator Implementation

// SimpleValidator implements basic validation
type SimpleValidator struct {
	userService *suser.UserService
}

// NewValidator creates a new simple validator
func NewValidator(userService *suser.UserService) *SimpleValidator {
	return &SimpleValidator{
		userService: userService,
	}
}

// ValidateExportRequest validates an export request
func (v *SimpleValidator) ValidateExportRequest(ctx context.Context, req *ExportRequest) error {
	if req == nil {
		return NewValidationError("request", "request cannot be nil")
	}

	// Validate workspace ID
	if req.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return NewValidationError("workspaceId", "workspace ID cannot be empty")
	}

	// Validate format
	if req.Format != ExportFormat_YAML && req.Format != ExportFormat_CURL {
		return NewValidationError("format", fmt.Sprintf("unsupported format: %v", req.Format))
	}

	// Validate that we have some data to export
	if len(req.FlowIDs) == 0 && len(req.ExampleIDs) == 0 {
		return NewValidationError("request", "at least one flow ID or example ID must be provided")
	}

	// Validate flow IDs
	for i, flowID := range req.FlowIDs {
		if flowID.Compare(idwrap.IDWrap{}) == 0 {
			return NewValidationError("flowIds", fmt.Sprintf("flow ID at index %d cannot be empty", i))
		}
	}

	// Validate example IDs
	for i, exampleID := range req.ExampleIDs {
		if exampleID.Compare(idwrap.IDWrap{}) == 0 {
			return NewValidationError("exampleIds", fmt.Sprintf("example ID at index %d cannot be empty", i))
		}
	}

	return nil
}

// ValidateWorkspaceAccess validates that the user has access to the workspace
func (v *SimpleValidator) ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	// For now, we'll implement basic validation
	// In a real implementation, this would check user permissions

	if workspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return NewValidationError("workspaceId", "workspace ID cannot be empty")
	}

	// TODO: Implement actual workspace access validation using user service
	// For now, we'll assume access is granted if we can parse the ID
	return nil
}

// ValidateExportFilter validates an export filter
func (v *SimpleValidator) ValidateExportFilter(ctx context.Context, filter ExportFilter) error {
	// Validate flow IDs
	for i, flowID := range filter.FlowIDs {
		if flowID.Compare(idwrap.IDWrap{}) == 0 {
			return NewValidationError("filter.flowIds", fmt.Sprintf("flow ID at index %d cannot be empty", i))
		}
	}

	// Validate example IDs
	for i, exampleID := range filter.ExampleIDs {
		if exampleID.Compare(idwrap.IDWrap{}) == 0 {
			return NewValidationError("filter.exampleIds", fmt.Sprintf("example ID at index %d cannot be empty", i))
		}
	}

	// Validate format
	if filter.Format != ExportFormat_YAML && filter.Format != ExportFormat_CURL {
		return NewValidationError("filter.format", fmt.Sprintf("unsupported format: %v", filter.Format))
	}

	return nil
}

// Service Implementation

// Service handles the business logic for export operations
type Service struct {
	exporter  Exporter
	validator Validator
	storage   Storage
	logger    *slog.Logger
}

// NewService creates a new export service
func NewService(exporter Exporter, validator Validator, storage Storage) *Service {
	// Set storage dependency on exporter if it's a SimpleExporter
	if simpleExporter, ok := exporter.(*SimpleExporter); ok {
		simpleExporter.SetStorage(storage)
	}

	return &Service{
		exporter:  exporter,
		validator: validator,
		storage:   storage,
		logger:    slog.Default(), // Can be enhanced with dependency injection if needed
	}
}

// Export performs the main export operation
func (s *Service) Export(ctx context.Context, req *ExportRequest) (*ExportResponse, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.logger.Info("Starting export operation",
		"workspace_id", req.WorkspaceID,
		"format", req.Format,
		"simplified", req.Simplified,
		"flow_ids_count", len(req.FlowIDs),
		"example_ids_count", len(req.ExampleIDs))

	// Validate the export request
	if err := s.validator.ValidateExportRequest(ctx, req); err != nil {
		return nil, err
	}

	// Validate workspace access
	if err := s.validator.ValidateWorkspaceAccess(ctx, req.WorkspaceID); err != nil {
		return nil, err
	}

	// Create export filter
	filter := ExportFilter{
		FlowIDs:    req.FlowIDs,
		ExampleIDs: req.ExampleIDs,
		Format:     req.Format,
		Simplified: req.Simplified,
	}

	// Validate export filter
	if err := s.validator.ValidateExportFilter(ctx, filter); err != nil {
		return nil, err
	}

	// Export workspace data
	exportData, err := s.exporter.ExportWorkspaceData(ctx, req.WorkspaceID, filter)
	if err != nil {
		return nil, fmt.Errorf("workspace data export failed: %w", err)
	}

	s.logger.Info("Workspace data export completed",
		"workspace_id", req.WorkspaceID,
		"flows_count", len(exportData.Flows),
		"http_requests_count", len(exportData.HTTPRequests),
		"files_count", len(exportData.Files))

	// Export to the requested format
	var data []byte
	var name string

	switch req.Format {
	case ExportFormat_YAML:
		data, err = s.exporter.ExportToYAML(ctx, exportData, req.Simplified)
		if err != nil {
			return nil, fmt.Errorf("YAML export failed: %w", err)
		}

		// Construct export name
		if exportData.Workspace != nil && exportData.Workspace.Name != "" {
			if req.Simplified {
				name = exportData.Workspace.Name + "_simplified.yaml"
			} else {
				name = exportData.Workspace.Name + ".yaml"
			}
		} else {
			if req.Simplified {
				name = "export_simplified.yaml"
			} else {
				name = "export.yaml"
			}
		}

	case ExportFormat_CURL:
		curlData, err := s.exporter.ExportToCurl(ctx, exportData, req.ExampleIDs)
		if err != nil {
			return nil, fmt.Errorf("cURL export failed: %w", err)
		}
		data = []byte(curlData)

		// Construct export name
		if exportData.Workspace != nil && exportData.Workspace.Name != "" {
			name = exportData.Workspace.Name + "_curl.sh"
		} else {
			name = "export_curl.sh"
		}

	default:
		return nil, NewValidationError("format", fmt.Sprintf("unsupported export format: %v", req.Format))
	}

	return &ExportResponse{
		Name: name,
		Data: data,
	}, nil
}

// ExportCurl performs cURL export operation
func (s *Service) ExportCurl(ctx context.Context, req *ExportCurlRequest) (*ExportCurlResponse, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.logger.Info("Starting cURL export operation",
		"workspace_id", req.WorkspaceID,
		"example_ids_count", len(req.ExampleIDs))

	// Create an export request for cURL format
	exportReq := &ExportRequest{
		WorkspaceID: req.WorkspaceID,
		ExampleIDs: req.ExampleIDs,
		Format:     ExportFormat_CURL,
		Simplified: false,
	}

	// Validate the export request
	if err := s.validator.ValidateExportRequest(ctx, exportReq); err != nil {
		return nil, err
	}

	// Validate workspace access
	if err := s.validator.ValidateWorkspaceAccess(ctx, req.WorkspaceID); err != nil {
		return nil, err
	}

	// Create export filter
	filter := ExportFilter{
		ExampleIDs: req.ExampleIDs,
		Format:     ExportFormat_CURL,
		Simplified: false,
	}

	// Validate export filter
	if err := s.validator.ValidateExportFilter(ctx, filter); err != nil {
		return nil, err
	}

	// Export workspace data
	exportData, err := s.exporter.ExportWorkspaceData(ctx, req.WorkspaceID, filter)
	if err != nil {
		return nil, fmt.Errorf("workspace data export failed: %w", err)
	}

	s.logger.Info("Workspace data export completed for cURL",
		"workspace_id", req.WorkspaceID,
		"http_requests_count", len(exportData.HTTPRequests))

	// Export to cURL format
	curlData, err := s.exporter.ExportToCurl(ctx, exportData, req.ExampleIDs)
	if err != nil {
		return nil, fmt.Errorf("cURL export failed: %w", err)
	}

	return &ExportCurlResponse{
		Data: curlData,
	}, nil
}