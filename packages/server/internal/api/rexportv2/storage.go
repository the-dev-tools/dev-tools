package rexportv2

import (
	"context"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
)

// Storage provides data access operations using modern services
type Storage interface {
	GetWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) (*WorkspaceInfo, error)
	GetFlows(ctx context.Context, workspaceID idwrap.IDWrap, fileIDs []idwrap.IDWrap) ([]*FlowData, error) // Use file IDs as flow identifiers
	GetHTTPRequests(ctx context.Context, workspaceID idwrap.IDWrap, httpIDs []idwrap.IDWrap) ([]*HTTPData, error)
	GetFiles(ctx context.Context, workspaceID idwrap.IDWrap, fileIDs []idwrap.IDWrap) ([]*FileData, error)
}

// SimpleStorage implements storage using modern services
type SimpleStorage struct {
	workspaceService *sworkspace.WorkspaceService
	httpService      *shttp.HTTPService
	flowService      *sflow.FlowService
	fileService      *sfile.FileService
}

// NewStorage creates a new storage instance with modern services
func NewStorage(
	ws *sworkspace.WorkspaceService,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
) *SimpleStorage {
	return &SimpleStorage{
		workspaceService: ws,
		httpService:      httpService,
		flowService:      flowService,
		fileService:      fileService,
	}
}

// GetWorkspace retrieves workspace information
func (s *SimpleStorage) GetWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) (*WorkspaceInfo, error) {
	// Use modern workspace service to get workspace info
	workspace, err := s.workspaceService.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	return &WorkspaceInfo{
		ID:   workspace.ID,
		Name: workspace.Name,
	}, nil
}

// GetFlows retrieves flow data for the given workspace and file IDs
func (s *SimpleStorage) GetFlows(ctx context.Context, workspaceID idwrap.IDWrap, fileIDs []idwrap.IDWrap) ([]*FlowData, error) {
	// Use modern flow service to get flows
	var flows []*FlowData

	// If specific file IDs are provided, try to get flows associated with those files
	// For now, we'll treat file IDs as flow IDs since the new spec uses file IDs
	if len(fileIDs) > 0 {
		for _, fileID := range fileIDs {
			// Try to get flow by file ID - this may need adjustment based on actual data model
			flow, err := s.flowService.GetFlow(ctx, fileID)
			if err != nil {
				// Log error but continue with other flows
				continue
			}

			flowData := &FlowData{
				ID:   flow.ID,
				Name: flow.Name,
			}
			flows = append(flows, flowData)
		}
	} else {
		// Get all flows for the workspace
		workspaceFlows, err := s.flowService.GetFlowsByWorkspaceID(ctx, workspaceID)
		if err != nil {
			return nil, err
		}

		for _, flow := range workspaceFlows {
			flowData := &FlowData{
				ID:   flow.ID,
				Name: flow.Name,
			}
			flows = append(flows, flowData)
		}
	}

	return flows, nil
}

// GetHTTPRequests retrieves HTTP request data for the given HTTP IDs
func (s *SimpleStorage) GetHTTPRequests(ctx context.Context, workspaceID idwrap.IDWrap, httpIDs []idwrap.IDWrap) ([]*HTTPData, error) {
	// Use modern HTTP service to get HTTP requests
	var httpRequests []*HTTPData

	// If specific HTTP IDs are provided, get only those
	if len(httpIDs) > 0 {
		for _, httpID := range httpIDs {
			httpReq, err := s.httpService.Get(ctx, httpID)
			if err != nil {
				// Log error but continue with other requests
				continue
			}

			httpData := &HTTPData{
				ID:     httpReq.ID,
				Name:   httpReq.Name,
				Method: httpReq.Method,
				Url:    httpReq.Url,
			}
			httpRequests = append(httpRequests, httpData)
		}
	} else {
		// Get all HTTP requests for the workspace
		workspaceRequests, err := s.httpService.GetByWorkspaceID(ctx, workspaceID)
		if err != nil {
			return nil, err
		}

		for _, httpReq := range workspaceRequests {
			httpData := &HTTPData{
				ID:     httpReq.ID,
				Name:   httpReq.Name,
				Method: httpReq.Method,
				Url:    httpReq.Url,
			}
			httpRequests = append(httpRequests, httpData)
		}
	}

	return httpRequests, nil
}

// GetFiles retrieves file data for the given file IDs
func (s *SimpleStorage) GetFiles(ctx context.Context, workspaceID idwrap.IDWrap, fileIDs []idwrap.IDWrap) ([]*FileData, error) {
	// Use modern file service to get files
	var files []*FileData

	// If specific file IDs are provided, get only those
	if len(fileIDs) > 0 {
		for _, fileID := range fileIDs {
			file, err := s.fileService.GetFile(ctx, fileID)
			if err != nil {
				// Log error but continue with other files
				continue
			}

			fileData := &FileData{
				ID:   file.ID,
				Name: file.Name,
			}
			files = append(files, fileData)
		}
	} else {
		// Get all files for the workspace
		workspaceFiles, err := s.fileService.ListFilesByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, err
		}

		for _, file := range workspaceFiles {
			fileData := &FileData{
				ID:   file.ID,
				Name: file.Name,
			}
			files = append(files, fileData)
		}
	}

	return files, nil
}
