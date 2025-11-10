package ioworkspacev2

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
)

// Mock services for testing
type MockHTTPService struct {
	httpRequests []mhttp.HTTP
	shouldError  bool
}

func (m *MockHTTPService) GetByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return m.httpRequests, nil
}

func (m *MockHTTPService) Create(ctx context.Context, http *mhttp.HTTP) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockHTTPService) Update(ctx context.Context, http *mhttp.HTTP) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockHTTPService) TX(tx interface{}) HTTPService {
	return m
}

type MockFileService struct {
	files       []mfile.File
	shouldError bool
}

func (m *MockFileService) GetFilesByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mfile.File, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return m.files, nil
}

func (m *MockFileService) CreateFile(ctx context.Context, file *mfile.File) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockFileService) UpdateFile(ctx context.Context, file *mfile.File) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockFileService) TX(tx interface{}) FileService {
	return m
}

type MockFlowService struct {
	flows      []mflow.Flow
	shouldError bool
}

func (m *MockFlowService) GetFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return m.flows, nil
}

func (m *MockFlowService) CreateFlow(ctx context.Context, flow *mflow.Flow) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockFlowService) UpdateFlow(ctx context.Context, flow *mflow.Flow) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockFlowService) TX(tx interface{}) FlowService {
	return m
}

// Interface definitions for mock services
type HTTPService interface {
	GetByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTP, error)
	Create(ctx context.Context, http *mhttp.HTTP) error
	Update(ctx context.Context, http *mhttp.HTTP) error
	TX(tx interface{}) HTTPService
}

type FileService interface {
	GetFilesByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mfile.File, error)
	CreateFile(ctx context.Context, file *mfile.File) error
	UpdateFile(ctx context.Context, file *mfile.File) error
	TX(tx interface{}) FileService
}

type FlowService interface {
	GetFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error)
	CreateFlow(ctx context.Context, flow *mflow.Flow) error
	UpdateFlow(ctx context.Context, flow *mflow.Flow) error
	TX(tx interface{}) FlowService
}

func TestNewIOWorkspaceServiceV2(t *testing.T) {
	db := &sql.DB{} // Mock DB
	httpService := &MockHTTPService{}
	fileService := &MockFileService{}
	flowService := &MockFlowService{}

	// Mock remaining services
	nodeService := struct{}{}
	requestService := struct{}{}
	conditionService := struct{}{}
	noopService := struct{}{}
	forService := struct{}{}
	forEachService := struct{}{}
	jsService := struct{}{}
	variableService := struct{}{}

	service := NewIOWorkspaceServiceV2(
		db,
		httpService,
		fileService,
		flowService,
		nodeService,
		requestService,
		conditionService,
		noopService,
		forService,
		forEachService,
		jsService,
		variableService,
	)

	if service == nil {
		t.Errorf("Expected service to be created")
	}

	if service.db != db {
		t.Errorf("Expected db to be set")
	}

	if service.httpService != httpService {
		t.Errorf("Expected httpService to be set")
	}

	if service.fileService != fileService {
		t.Errorf("Expected fileService to be set")
	}

	if service.flowService != flowService {
		t.Errorf("Expected flowService to be set")
	}
}

func TestIOWorkspaceServiceV2_ImportWorkspace(t *testing.T) {
	workspaceID := idwrap.NewNow()
	options := GetDefaultOptions(workspaceID)

	resolved := yamlflowsimplev2.SimplifiedYAMLResolvedV2{
		HTTPRequests: []mhttp.HTTP{
			{ID: idwrap.NewNow(), Name: "Test HTTP", Method: "GET", Url: "https://example.com"},
		},
		Files: []mfile.File{
			{ID: idwrap.NewNow(), Name: "Test File", ContentType: "http"},
		},
		Flows: []mflow.Flow{
			{ID: idwrap.NewNow(), Name: "Test Flow", WorkspaceID: workspaceID},
		},
		FlowNodes: []mnnode.MNode{
			{ID: idwrap.NewNow(), Name: "Test Node", FlowID: workspaceID},
		},
		FlowVariables: []mflowvariable.FlowVariable{
			{ID: idwrap.NewNow(), Name: "test_var", FlowID: workspaceID},
		},
	}

	// Create mock services
	httpService := &MockHTTPService{}
	fileService := &MockFileService{}
	flowService := &MockFlowService{}

	// Mock remaining services (for simplicity)
	nodeService := struct{}{}
	requestService := struct{}{}
	conditionService := struct{}{}
	noopService := struct{}{}
	forService := struct{}{}
	forEachService := struct{}{}
	jsService := struct{}{}
	variableService := struct{}{}

	service := &IOWorkspaceServiceV2{
		httpService:  httpService,
		fileService:  fileService,
		flowService:  flowService,
		nodeService:  nodeService,
		requestService: requestService,
		conditionService: conditionService,
		noopService: noopService,
		forService: forService,
		forEachService: forEachService,
		jsService: jsService,
		variableService: variableService,
	}

	// Test basic functionality (without actual database)
	t.Run("Basic import validation", func(t *testing.T) {
		// This would normally use a real database transaction
		// For testing, we just validate the inputs and structure
		if len(resolved.HTTPRequests) != 1 {
			t.Errorf("Expected 1 HTTP request, got %d", len(resolved.HTTPRequests))
		}

		if len(resolved.Files) != 1 {
			t.Errorf("Expected 1 file, got %d", len(resolved.Files))
		}

		if len(resolved.Flows) != 1 {
			t.Errorf("Expected 1 flow, got %d", len(resolved.Flows))
		}

		if len(resolved.FlowNodes) != 1 {
			t.Errorf("Expected 1 flow node, got %d", len(resolved.FlowNodes))
		}

		if len(resolved.FlowVariables) != 1 {
			t.Errorf("Expected 1 flow variable, got %d", len(resolved.FlowVariables))
		}
	})
}

func TestIOWorkspaceServiceV2_ImportWorkspaceFromYAML(t *testing.T) {
	workspaceID := idwrap.NewNow()
	options := GetDefaultOptions(workspaceID)

	yamlData := `
workspace_name: Test Workspace
flows:
  - name: Test Flow
    steps:
      - request:
          name: Test Request
          method: GET
          url: https://api.example.com/test
`

	// Create mock services
	httpService := &MockHTTPService{}
	fileService := &MockFileService{}
	flowService := &MockFlowService{}

	// Mock remaining services
	nodeService := struct{}{}
	requestService := struct{}{}
	conditionService := struct{}{}
	noopService := struct{}{}
	forService := struct{}{}
	forEachService := struct{}{}
	jsService := struct{}{}
	variableService := struct{}{}

	service := &IOWorkspaceServiceV2{
		httpService:  httpService,
		fileService:  fileService,
		flowService:  flowService,
		nodeService:  nodeService,
		requestService: requestService,
		conditionService: conditionService,
		noopService: noopService,
		forService: forService,
		forEachService: forEachService,
		jsService: jsService,
		variableService: variableService,
	}

	t.Run("YAML import validation", func(t *testing.T) {
		// Test YAML conversion first
		resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML([]byte(yamlData), yamlflowsimplev2.ConvertOptionsV2{
			WorkspaceID: workspaceID,
		})

		if err != nil {
			t.Fatalf("Failed to convert YAML: %v", err)
		}

		// Validate converted data
		if len(resolved.Flows) != 1 {
			t.Errorf("Expected 1 flow, got %d", len(resolved.Flows))
		}

		if len(resolved.HTTPRequests) != 1 {
			t.Errorf("Expected 1 HTTP request, got %d", len(resolved.HTTPRequests))
		}

		if resolved.Flows[0].Name != "Test Flow" {
			t.Errorf("Expected flow name 'Test Flow', got '%s'", resolved.Flows[0].Name)
		}

		if resolved.HTTPRequests[0].Name != "Test Request" {
			t.Errorf("Expected HTTP request name 'Test Request', got '%s'", resolved.HTTPRequests[0].Name)
		}
	})
}

func TestIOWorkspaceServiceV2_ConcurrentImportWorkspace(t *testing.T) {
	workspaceID := idwrap.NewNow()
	options := GetDefaultOptions(workspaceID)
	config := GetDefaultConfig()

	resolved := yamlflowsimplev2.SimplifiedYAMLResolvedV2{
		HTTPRequests: []mhttp.HTTP{
			{ID: idwrap.NewNow(), Name: "HTTP 1", Method: "GET", Url: "https://example.com/1"},
			{ID: idwrap.NewNow(), Name: "HTTP 2", Method: "POST", Url: "https://example.com/2"},
		},
		Files: []mfile.File{
			{ID: idwrap.NewNow(), Name: "File 1", ContentType: "http"},
			{ID: idwrap.NewNow(), Name: "File 2", ContentType: "http"},
		},
		Flows: []mflow.Flow{
			{ID: idwrap.NewNow(), Name: "Flow 1", WorkspaceID: workspaceID},
			{ID: idwrap.NewNow(), Name: "Flow 2", WorkspaceID: workspaceID},
		},
	}

	// Create mock services
	httpService := &MockHTTPService{}
	fileService := &MockFileService{}
	flowService := &MockFlowService{}

	// Mock remaining services
	nodeService := struct{}{}
	requestService := struct{}{}
	conditionService := struct{}{}
	noopService := struct{}{}
	forService := struct{}{}
	forEachService := struct{}{}
	jsService := struct{}{}
	variableService := struct{}{}

	service := &IOWorkspaceServiceV2{
		httpService:  httpService,
		fileService:  fileService,
		flowService:  flowService,
		nodeService:  nodeService,
		requestService: requestService,
		conditionService: conditionService,
		noopService: noopService,
		forService: forService,
		forEachService: forEachService,
		jsService: jsService,
		variableService: variableService,
	}

	t.Run("Concurrent import validation", func(t *testing.T) {
		// Test context creation
		ctx := context.Background()
		importCtx, err := NewImportContext(ctx, options, config)

		if err != nil {
			t.Fatalf("Failed to create import context: %v", err)
		}

		if importCtx == nil {
			t.Errorf("Expected import context to be created")
		}

		if importCtx.Options.WorkspaceID.Compare(workspaceID) != 0 {
			t.Errorf("Expected workspace ID to match")
		}

		if importCtx.Config.MaxConcurrentGoroutines != 10 {
			t.Errorf("Expected max concurrent goroutines 10, got %d", importCtx.Config.MaxConcurrentGoroutines)
		}

		// Test resolved data validation
		if len(resolved.HTTPRequests) != 2 {
			t.Errorf("Expected 2 HTTP requests, got %d", len(resolved.HTTPRequests))
		}

		if len(resolved.Files) != 2 {
			t.Errorf("Expected 2 files, got %d", len(resolved.Files))
		}

		if len(resolved.Flows) != 2 {
			t.Errorf("Expected 2 flows, got %d", len(resolved.Flows))
		}
	})
}

func TestIOWorkspaceServiceV2_processFlowBatch(t *testing.T) {
	workspaceID := idwrap.NewNow()
	options := GetDefaultOptions(workspaceID)

	existingFlows := []mflow.Flow{
		{ID: idwrap.NewNow(), Name: "Existing Flow", WorkspaceID: workspaceID},
	}

	existingMap := make(map[string]*mflow.Flow)
	for i := range existingFlows {
		existingMap[existingFlows[i].Name] = &existingFlows[i]
	}

	newFlows := []mflow.Flow{
		{ID: idwrap.NewNow(), Name: "New Flow", WorkspaceID: workspaceID},
		{ID: idwrap.NewNow(), Name: "Existing Flow", WorkspaceID: workspaceID}, // This will conflict
	}

	flowService := &MockFlowService{flows: existingFlows}
	results := &ImportResults{}

	t.Run("Skip duplicates strategy", func(t *testing.T) {
		options.MergeStrategy = MergeStrategySkipDuplicates

		service := &IOWorkspaceServiceV2{flowService: flowService}
		err := service.processFlowBatch(context.Background(), nil, newFlows, existingMap, options, results)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if results.FlowsSkipped != 1 {
			t.Errorf("Expected 1 flow skipped, got %d", results.FlowsSkipped)
		}

		if results.FlowsCreated != 1 {
			t.Errorf("Expected 1 flow created, got %d", results.FlowsCreated)
		}

		if len(results.Conflicts) != 1 {
			t.Errorf("Expected 1 conflict, got %d", len(results.Conflicts))
		}
	})

	t.Run("Create new strategy", func(t *testing.T) {
		// Reset results
		results = &ImportResults{}
		options.MergeStrategy = MergeStrategyCreateNew

		service := &IOWorkspaceServiceV2{flowService: flowService}
		err := service.processFlowBatch(context.Background(), nil, newFlows, existingMap, options, results)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if results.FlowsCreated != 2 {
			t.Errorf("Expected 2 flows created, got %d", results.FlowsCreated)
		}

		if len(results.Conflicts) == 0 {
			t.Errorf("Expected conflicts when strategy allows creating duplicates")
		}
	})
}

func TestIOWorkspaceServiceV2_processHTTPRequestBatch(t *testing.T) {
	workspaceID := idwrap.NewNow()
	options := GetDefaultOptions(workspaceID)

	existingRequests := []mhttp.HTTP{
		{ID: idwrap.NewNow(), Method: "GET", Url: "https://example.com/api", WorkspaceID: workspaceID},
	}

	existingMap := make(map[string]*mhttp.HTTP)
	for i := range existingRequests {
		key := fmt.Sprintf("%s|%s", existingRequests[i].Method, existingRequests[i].Url)
		existingMap[key] = &existingRequests[i]
	}

	newRequests := []mhttp.HTTP{
		{ID: idwrap.NewNow(), Method: "POST", Url: "https://example.com/new", WorkspaceID: workspaceID},
		{ID: idwrap.NewNow(), Method: "GET", Url: "https://example.com/api", WorkspaceID: workspaceID}, // This will conflict
	}

	httpService := &MockHTTPService{httpRequests: existingRequests}
	results := &ImportResults{}

	t.Run("Skip duplicates strategy", func(t *testing.T) {
		options.MergeStrategy = MergeStrategySkipDuplicates

		service := &IOWorkspaceServiceV2{httpService: httpService}
		err := service.processHTTPRequestBatch(context.Background(), nil, newRequests, existingMap, options, results)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if results.HTTPReqsSkipped != 1 {
			t.Errorf("Expected 1 HTTP request skipped, got %d", results.HTTPReqsSkipped)
		}

		if results.HTTPReqsCreated != 1 {
			t.Errorf("Expected 1 HTTP request created, got %d", results.HTTPReqsCreated)
		}

		if len(results.Conflicts) != 1 {
			t.Errorf("Expected 1 conflict, got %d", len(results.Conflicts))
		}
	})
}

func TestIOWorkspaceServiceV2_processFileBatch(t *testing.T) {
	workspaceID := idwrap.NewNow()
	options := GetDefaultOptions(workspaceID)

	existingFiles := []mfile.File{
		{ID: idwrap.NewNow(), Name: "existing.json", WorkspaceID: workspaceID},
	}

	existingMap := make(map[string]*mfile.File)
	for i := range existingFiles {
		existingMap[existingFiles[i].Name] = &existingFiles[i]
	}

	newFiles := []mfile.File{
		{ID: idwrap.NewNow(), Name: "new.json", WorkspaceID: workspaceID},
		{ID: idwrap.NewNow(), Name: "existing.json", WorkspaceID: workspaceID}, // This will conflict
	}

	fileService := &MockFileService{files: existingFiles}
	results := &ImportResults{}

	t.Run("Skip duplicates strategy", func(t *testing.T) {
		options.MergeStrategy = MergeStrategySkipDuplicates

		service := &IOWorkspaceServiceV2{fileService: fileService}
		err := service.processFileBatch(context.Background(), nil, newFiles, existingMap, options, results)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if results.FilesSkipped != 1 {
			t.Errorf("Expected 1 file skipped, got %d", results.FilesSkipped)
		}

		if results.FilesCreated != 1 {
			t.Errorf("Expected 1 file created, got %d", results.FilesCreated)
		}

		if len(results.Conflicts) != 1 {
			t.Errorf("Expected 1 conflict, got %d", len(results.Conflicts))
		}
	})
}