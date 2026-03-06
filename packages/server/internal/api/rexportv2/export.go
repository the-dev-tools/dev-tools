//nolint:revive // exported
package rexportv2

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/swebsocket"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/yamlflowsimplev2"

	"gopkg.in/yaml.v3"
)

// Interfaces

// Exporter provides export functionality for different formats
type Exporter interface {
	ExportWorkspaceData(ctx context.Context, workspaceID idwrap.IDWrap, filter ExportFilter) (*WorkspaceExportData, error)
	ExportToYAML(ctx context.Context, data *WorkspaceExportData, simplified bool, flowIDs []idwrap.IDWrap) ([]byte, error)
	ExportToCurl(ctx context.Context, data *WorkspaceExportData, httpIDs []idwrap.IDWrap) (string, error)
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
	httpService            *shttp.HTTPService
	flowService            *sflow.FlowService
	fileService            *sfile.FileService
	ioWorkspaceService     *ioworkspace.IOWorkspaceService
	graphqlService         *sgraphql.GraphQLService
	graphqlHeaderService   *sgraphql.GraphQLHeaderService
	graphqlAssertService   *sgraphql.GraphQLAssertService
	websocketService       *swebsocket.WebSocketService
	websocketHeaderService *swebsocket.WebSocketHeaderService
	storage                Storage
}

// NewExporter creates a new SimpleExporter
func NewExporter(
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	ioWorkspaceService *ioworkspace.IOWorkspaceService,
	graphqlService *sgraphql.GraphQLService,
	graphqlHeaderService *sgraphql.GraphQLHeaderService,
	graphqlAssertService *sgraphql.GraphQLAssertService,
	websocketService *swebsocket.WebSocketService,
	websocketHeaderService *swebsocket.WebSocketHeaderService,
) *SimpleExporter {
	return &SimpleExporter{
		httpService:            httpService,
		flowService:            flowService,
		fileService:            fileService,
		ioWorkspaceService:     ioWorkspaceService,
		graphqlService:         graphqlService,
		graphqlHeaderService:   graphqlHeaderService,
		graphqlAssertService:   graphqlAssertService,
		websocketService:       websocketService,
		websocketHeaderService: websocketHeaderService,
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

	// Get flows (using file IDs as flow identifiers for now)
	flows, err := e.storage.GetFlows(ctx, workspaceID, filter.FileIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get flows: %w", err)
	}

	// Get HTTP requests
	httpRequests, err := e.storage.GetHTTPRequests(ctx, workspaceID, filter.HTTPIDs)
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

// ExportToYAML exports data to YAML format using ioworkspace and yamlflowsimplev2
func (e *SimpleExporter) ExportToYAML(ctx context.Context, data *WorkspaceExportData, simplified bool, flowIDs []idwrap.IDWrap) ([]byte, error) {
	if data.Workspace == nil {
		return nil, fmt.Errorf("workspace data is required for YAML export")
	}

	if e.ioWorkspaceService == nil {
		return nil, fmt.Errorf("ioWorkspaceService is required for YAML export")
	}

	// Use ioworkspace to export workspace bundle with optional flow filtering
	exportOpts := ioworkspace.ExportOptions{
		WorkspaceID:         data.Workspace.ID,
		IncludeHTTP:         true,
		IncludeFlows:        true,
		IncludeEnvironments: true,
		IncludeFiles:        false,
		FilterByFlowIDs:     flowIDs,
	}

	bundle, err := e.ioWorkspaceService.Export(ctx, exportOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to export workspace bundle: %w", err)
	}

	// Use yamlflowsimplev2 to marshal to YAML
	yamlData, err := yamlflowsimplev2.MarshalSimplifiedYAML(bundle)
	if err != nil {
		return nil, fmt.Errorf("YAML marshalling failed: %w", err)
	}

	return yamlData, nil
}

// ExportToCurl exports data to cURL format
func (e *SimpleExporter) ExportToCurl(ctx context.Context, data *WorkspaceExportData, httpIDs []idwrap.IDWrap) (string, error) {
	if len(data.HTTPRequests) == 0 {
		return "", nil
	}

	// Create a set of IDs for efficient lookup
	httpIDSet := make(map[idwrap.IDWrap]bool)
	for _, id := range httpIDs {
		httpIDSet[id] = true
	}

	var commands []string
	for _, httpReq := range data.HTTPRequests {
		// Skip this request if httpIDs is provided and this request is not in the filter
		if len(httpIDs) > 0 && !httpIDSet[httpReq.ID] {
			continue
		}

		var cmd strings.Builder
		cmd.WriteString(fmt.Sprintf("curl -X %s '%s'", httpReq.Method, httpReq.Url))

		// Add headers if present
		if len(httpReq.Headers) > 0 {
			for key, values := range httpReq.Headers {
				for _, value := range values {
					cmd.WriteString(fmt.Sprintf(" -H \"%s: %s\"", key, value))
				}
			}
		}

		// Add body if present
		if httpReq.Body != "" {
			cmd.WriteString(fmt.Sprintf(" --data-raw '%s'", strings.ReplaceAll(httpReq.Body, "'", "'\"'\"'")))
		}

		cmd.WriteString(fmt.Sprintf(" # %s", httpReq.Name))
		commands = append(commands, cmd.String())
	}

	if len(commands) == 0 {
		return "", nil
	}

	return strings.Join(commands, "\n\n"), nil
}

// ExportGraphQLToCurl exports GraphQL requests as cURL commands (POST with JSON body)
func (e *SimpleExporter) ExportGraphQLToCurl(ctx context.Context, graphqlIDs []idwrap.IDWrap) (string, error) {
	if len(graphqlIDs) == 0 {
		return "", nil
	}

	var commands []string
	for _, gqlID := range graphqlIDs {
		gql, err := e.graphqlService.Get(ctx, gqlID)
		if err != nil {
			continue
		}

		headers, err := e.graphqlHeaderService.GetByGraphQLID(ctx, gqlID)
		if err != nil {
			headers = nil
		}

		var cmd strings.Builder
		cmd.WriteString(fmt.Sprintf("curl -X POST '%s'", gql.Url))
		cmd.WriteString(" -H \"Content-Type: application/json\"")

		for _, h := range headers {
			if h.Enabled {
				cmd.WriteString(fmt.Sprintf(" -H \"%s: %s\"", h.Key, h.Value))
			}
		}

		// Build JSON body with query and variables
		body := buildGraphQLJSONBody(gql.Query, gql.Variables)
		cmd.WriteString(fmt.Sprintf(" --data-raw '%s'", strings.ReplaceAll(body, "'", "'\"'\"'")))
		cmd.WriteString(fmt.Sprintf(" # %s", gql.Name))
		commands = append(commands, cmd.String())
	}

	if len(commands) == 0 {
		return "", nil
	}
	return strings.Join(commands, "\n\n"), nil
}

// buildGraphQLJSONBody builds a JSON string with query and optional variables
func buildGraphQLJSONBody(query, variables string) string {
	// Escape the query string for JSON
	queryJSON, _ := json.Marshal(query)

	if variables == "" || variables == "{}" {
		return fmt.Sprintf(`{"query":%s}`, string(queryJSON))
	}

	// Variables is already a JSON string, use it directly
	return fmt.Sprintf(`{"query":%s,"variables":%s}`, string(queryJSON), variables)
}

// ExportGraphQLToYAML exports GraphQL requests as a focused YAML
func (e *SimpleExporter) ExportGraphQLToYAML(ctx context.Context, graphqlIDs []idwrap.IDWrap) ([]byte, error) {
	var gqlDefs []yamlflowsimplev2.YamlGraphQLDefV2

	for _, gqlID := range graphqlIDs {
		gql, err := e.graphqlService.Get(ctx, gqlID)
		if err != nil {
			continue
		}

		headers, err := e.graphqlHeaderService.GetByGraphQLID(ctx, gqlID)
		if err != nil {
			headers = nil
		}

		asserts, err := e.graphqlAssertService.GetByGraphQLID(ctx, gqlID)
		if err != nil {
			asserts = nil
		}

		gqlDef := yamlflowsimplev2.YamlGraphQLDefV2{
			Name:       gql.Name,
			URL:        gql.Url,
			Query:      gql.Query,
			Variables:  gql.Variables,
			Headers:    buildGraphQLHeaderMapOrSliceExport(headers),
			Assertions: buildGraphQLAssertionsExport(asserts),
		}
		gqlDefs = append(gqlDefs, gqlDef)
	}

	yamlFormat := yamlflowsimplev2.YamlFlowFormatV2{
		WorkspaceName:   "export",
		GraphQLRequests: gqlDefs,
	}

	return yaml.Marshal(yamlFormat)
}

// ExportWebSocketToYAML exports WebSocket items as a focused YAML
func (e *SimpleExporter) ExportWebSocketToYAML(ctx context.Context, websocketIDs []idwrap.IDWrap) ([]byte, error) {
	// Build a minimal YAML with websocket info
	type wsYAMLItem struct {
		Name    string            `yaml:"name"`
		URL     string            `yaml:"url"`
		Headers map[string]string `yaml:"headers,omitempty"`
	}

	type wsYAMLFormat struct {
		WorkspaceName string       `yaml:"workspace_name"`
		WebSockets    []wsYAMLItem `yaml:"websockets"`
	}

	var items []wsYAMLItem
	for _, wsID := range websocketIDs {
		ws, err := e.websocketService.Get(ctx, wsID)
		if err != nil {
			continue
		}

		headers, err := e.websocketHeaderService.GetByWebSocketID(ctx, wsID)
		if err != nil {
			headers = nil
		}

		headerMap := make(map[string]string)
		for _, h := range headers {
			if h.Enabled {
				headerMap[h.Key] = h.Value
			}
		}

		item := wsYAMLItem{
			Name: ws.Name,
			URL:  ws.Url,
		}
		if len(headerMap) > 0 {
			item.Headers = headerMap
		}
		items = append(items, item)
	}

	wsFormat := wsYAMLFormat{
		WorkspaceName: "export",
		WebSockets:    items,
	}

	return yaml.Marshal(wsFormat)
}

// tryPerItemYAMLExport checks if fileIDs refer to GraphQL or WebSocket items
// and exports them as focused YAML. Returns (data, name, handled).
func (e *SimpleExporter) tryPerItemYAMLExport(ctx context.Context, fileIDs []idwrap.IDWrap) ([]byte, string, bool) {
	if e.fileService == nil {
		return nil, "", false
	}

	// Check the first fileID's content type
	file, err := e.fileService.GetFile(ctx, fileIDs[0])
	if err != nil {
		return nil, "", false
	}

	switch file.ContentType {
	case mfile.ContentTypeGraphQL:
		data, err := e.ExportGraphQLToYAML(ctx, fileIDs)
		if err != nil {
			return nil, "", false
		}
		return data, "graphql_export.yaml", true

	case mfile.ContentTypeWebSocket:
		data, err := e.ExportWebSocketToYAML(ctx, fileIDs)
		if err != nil {
			return nil, "", false
		}
		return data, "websocket_export.yaml", true

	case mfile.ContentTypeHTTP, mfile.ContentTypeHTTPDelta:
		// Use FilterByHTTPIDs for per-item HTTP export
		exportOpts := ioworkspace.ExportOptions{
			WorkspaceID:     file.WorkspaceID,
			IncludeHTTP:     true,
			FilterByHTTPIDs: fileIDs,
		}
		bundle, err := e.ioWorkspaceService.Export(ctx, exportOpts)
		if err != nil {
			return nil, "", false
		}
		yamlData, err := yamlflowsimplev2.MarshalSimplifiedYAML(bundle)
		if err != nil {
			return nil, "", false
		}
		return yamlData, "http_export.yaml", true

	default:
		return nil, "", false
	}
}

func buildGraphQLHeaderMapOrSliceExport(headers []mgraphql.GraphQLHeader) yamlflowsimplev2.HeaderMapOrSlice {
	if len(headers) == 0 {
		return nil
	}
	var result []yamlflowsimplev2.YamlNameValuePairV2
	for _, h := range headers {
		result = append(result, yamlflowsimplev2.YamlNameValuePairV2{
			Name:        h.Key,
			Value:       h.Value,
			Enabled:     h.Enabled,
			Description: h.Description,
		})
	}
	return yamlflowsimplev2.HeaderMapOrSlice(result)
}

func buildGraphQLAssertionsExport(asserts []mgraphql.GraphQLAssert) yamlflowsimplev2.AssertionsOrSlice {
	if len(asserts) == 0 {
		return nil
	}
	var result []yamlflowsimplev2.YamlAssertionV2
	for _, a := range asserts {
		result = append(result, yamlflowsimplev2.YamlAssertionV2{Expression: a.Value, Enabled: a.Enabled})
	}
	return yamlflowsimplev2.AssertionsOrSlice(result)
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

	// Validate file IDs
	for i, fileID := range req.FileIDs {
		if fileID.Compare(idwrap.IDWrap{}) == 0 {
			return NewValidationError("fileIds", fmt.Sprintf("file ID at index %d cannot be empty", i))
		}
	}

	return nil
}

// ValidateWorkspaceAccess validates that the user has access to the workspace
func (v *SimpleValidator) ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	if workspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return NewValidationError("workspaceId", "workspace ID cannot be empty")
	}

	// Check user permissions using rworkspace helper
	hasAccess, err := mwauth.CheckOwnerWorkspace(ctx, *v.userService, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to check workspace access: %w", err)
	}

	if !hasAccess {
		// Return NotFound to prevent ID enumeration/leaking existence
		return ErrWorkspaceNotFound
	}

	return nil
}

// ValidateExportFilter validates an export filter
func (v *SimpleValidator) ValidateExportFilter(ctx context.Context, filter ExportFilter) error {
	// Validate file IDs
	for i, fileID := range filter.FileIDs {
		if fileID.Compare(idwrap.IDWrap{}) == 0 {
			return NewValidationError("filter.fileIds", fmt.Sprintf("file ID at index %d cannot be empty", i))
		}
	}

	// Validate HTTP IDs
	for i, httpID := range filter.HTTPIDs {
		if httpID.Compare(idwrap.IDWrap{}) == 0 {
			return NewValidationError("filter.httpIds", fmt.Sprintf("HTTP ID at index %d cannot be empty", i))
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
		"file_ids_count", len(req.FileIDs))

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
		FileIDs:    req.FileIDs,
		HTTPIDs:    []idwrap.IDWrap{}, // Empty for regular export
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
		// Try per-item YAML export for GraphQL and WebSocket items
		if simpleExporter, ok := s.exporter.(*SimpleExporter); ok && len(req.FileIDs) > 0 {
			itemData, itemName, handled := simpleExporter.tryPerItemYAMLExport(ctx, req.FileIDs)
			if handled {
				return &ExportResponse{
					Name: itemName,
					Data: itemData,
				}, nil
			}
		}

		data, err = s.exporter.ExportToYAML(ctx, exportData, req.Simplified, req.FileIDs)
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
		// For cURL format, we need HTTP requests but regular ExportRequest only has file IDs
		// This is a limitation of the new spec - we may need to revisit this approach
		curlData, err := s.exporter.ExportToCurl(ctx, exportData, []idwrap.IDWrap{})
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
		"http_ids_count", len(req.HTTPIDs))

	// Create an export request for cURL format
	exportReq := &ExportRequest{
		WorkspaceID: req.WorkspaceID,
		FileIDs:     []idwrap.IDWrap{}, // Empty for cURL export
		Format:      ExportFormat_CURL,
		Simplified:  false,
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
		FileIDs:    []idwrap.IDWrap{}, // Empty for cURL export
		HTTPIDs:    req.HTTPIDs,
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
	curlData, err := s.exporter.ExportToCurl(ctx, exportData, req.HTTPIDs)
	if err != nil {
		return nil, fmt.Errorf("cURL export failed: %w", err)
	}

	return &ExportCurlResponse{
		Data: curlData,
	}, nil
}
