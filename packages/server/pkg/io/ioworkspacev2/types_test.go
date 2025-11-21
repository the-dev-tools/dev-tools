package ioworkspacev2

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
)

func TestMergeStrategyString(t *testing.T) {
	tests := []struct {
		strategy  MergeStrategy
		expected  string
	}{
		{MergeStrategyCreateNew, "create_new"},
		{MergeStrategyMergeExisting, "merge_existing"},
		{MergeStrategySkipDuplicates, "skip_duplicates"},
		{MergeStrategyReplaceExisting, "replace_existing"},
		{MergeStrategyUpdateExisting, "update_existing"},
		{MergeStrategy(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.strategy.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestConflictTypeString(t *testing.T) {
	tests := []struct {
		conflictType ConflictType
		expected     string
	}{
		{ConflictTypeHTTP, "http"},
		{ConflictTypeFile, "file"},
		{ConflictTypeFlow, "flow"},
		{ConflictTypeNode, "node"},
		{ConflictTypeEdge, "edge"},
		{ConflictTypeVariable, "variable"},
		{ConflictType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.conflictType.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestConflictResolutionString(t *testing.T) {
	tests := []struct {
		resolution ConflictResolution
		expected   string
	}{
		{ConflictResolutionSkipped, "skipped"},
		{ConflictResolutionMerged, "merged"},
		{ConflictResolutionReplaced, "replaced"},
		{ConflictResolutionUpdated, "updated"},
		{ConflictResolutionCreated, "created"},
		{ConflictResolution(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.resolution.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestImportError(t *testing.T) {
	tests := []struct {
		name       string
		err        ImportError
		expectedMsg string
	}{
		{
			name: "Error with entity name",
			err: ImportError{
				EntityID:   idwrap.NewNow(),
				EntityName: "TestEntity",
				EntityType: "test",
				Err:        fmt.Errorf("something went wrong"),
			},
			expectedMsg: "import error for test 'TestEntity': something went wrong",
		},
		{
			name: "Error without entity name",
			err: ImportError{
				EntityID:   idwrap.NewNow(),
				EntityName: "",
				EntityType: "test",
				Err:        fmt.Errorf("something went wrong"),
			},
			expectedMsg: "import error for test: something went wrong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedMsg, result)
			}
		})
	}
}

func TestImportResultsAddConflict(t *testing.T) {
	results := &ImportResults{}
	conflict := ImportConflict{
		Type:       ConflictTypeHTTP,
		EntityID:   idwrap.NewNow(),
		EntityName: "Test Conflict",
		Resolution: ConflictResolutionSkipped,
	}

	results.AddConflict(conflict)

	if len(results.Conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(results.Conflicts))
	}

	if results.Conflicts[0].EntityName != "Test Conflict" {
		t.Errorf("Expected conflict name 'Test Conflict', got '%s'", results.Conflicts[0].EntityName)
	}
}

func TestImportResultsAddError(t *testing.T) {
	results := &ImportResults{}
	err := ImportError{
		EntityID:   idwrap.NewNow(),
		EntityName: "Test Error",
		EntityType: "test",
		Err:        fmt.Errorf("test error"),
	}

	results.AddError(err)

	if len(results.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(results.Errors))
	}

	if results.Errors[0].EntityName != "Test Error" {
		t.Errorf("Expected error entity name 'Test Error', got '%s'", results.Errors[0].EntityName)
	}
}

func TestImportResultsStatusMethods(t *testing.T) {
	tests := []struct {
		name               string
		setupResults       func() *ImportResults
		hasConflicts       bool
		hasErrors          bool
		isSuccess          bool
		totalProcessed     int
		successRate        float64
	}{
		{
			name: "Empty results",
			setupResults: func() *ImportResults {
				return &ImportResults{}
			},
			hasConflicts:   false,
			hasErrors:      false,
			isSuccess:      true,
			totalProcessed: 0,
			successRate:    0,
		},
		{
			name: "Results with conflicts only",
			setupResults: func() *ImportResults {
				return &ImportResults{
					Conflicts: []ImportConflict{{}},
				}
			},
			hasConflicts:   true,
			hasErrors:      false,
			isSuccess:      true,
			totalProcessed: 0,
			successRate:    0,
		},
		{
			name: "Results with errors only",
			setupResults: func() *ImportResults {
				return &ImportResults{
					Errors: []ImportError{{}},
				}
			},
			hasConflicts:   false,
			hasErrors:      true,
			isSuccess:      false,
			totalProcessed: 0,
			successRate:    0,
		},
		{
			name: "Successful import",
			setupResults: func() *ImportResults {
				return &ImportResults{
					HTTPReqsCreated: 5,
					FilesCreated:    3,
					FlowsCreated:    2,
				}
			},
			hasConflicts:   false,
			hasErrors:      false,
			isSuccess:      true,
			totalProcessed: 10,
			successRate:    100,
		},
		{
			name: "Mixed success and failure",
			setupResults: func() *ImportResults {
				return &ImportResults{
					HTTPReqsCreated: 5,
					HTTPReqsFailed:  2,
					FilesCreated:    3,
					FilesFailed:     1,
				}
			},
			hasConflicts:   false,
			hasErrors:      false,
			isSuccess:      true,
			totalProcessed: 11,
			successRate:    72.72727272727273, // 8/11 * 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := tt.setupResults()

			if results.HasConflicts() != tt.hasConflicts {
				t.Errorf("HasConflicts() = %v, want %v", results.HasConflicts(), tt.hasConflicts)
			}

			if results.HasErrors() != tt.hasErrors {
				t.Errorf("HasErrors() = %v, want %v", results.HasErrors(), tt.hasErrors)
			}

			if results.IsSuccess() != tt.isSuccess {
				t.Errorf("IsSuccess() = %v, want %v", results.IsSuccess(), tt.isSuccess)
			}

			if results.GetTotalProcessed() != tt.totalProcessed {
				t.Errorf("GetTotalProcessed() = %v, want %v", results.GetTotalProcessed(), tt.totalProcessed)
			}

			successRate := results.GetSuccessRate()
			if successRate != tt.successRate {
				t.Errorf("GetSuccessRate() = %v, want %v", successRate, tt.successRate)
			}
		})
	}
}

func TestImportResultsSummary(t *testing.T) {
	results := &ImportResults{
		Duration:         1500,
		HTTPReqsCreated: 5,
		HTTPReqsUpdated: 2,
		HTTPReqsSkipped: 1,
		HTTPReqsFailed:  1,
		FilesCreated:    3,
		FilesUpdated:    1,
		FilesSkipped:    1,
		FilesFailed:     0,
		FlowsCreated:    2,
		FlowsUpdated:    1,
		FlowsSkipped:    0,
		FlowsFailed:     1,
	}

	summary := results.Summary()

	expected := "Import completed in 1500ms. " +
		"HTTP: 5 created, 2 updated, 1 skipped, 1 failed. " +
		"Files: 3 created, 1 updated, 1 skipped, 0 failed. " +
		"Flows: 2 created, 1 updated, 0 skipped, 1 failed. " +
		"Success rate: 77.8%"

	if summary != expected {
		t.Errorf("Summary mismatch.\nExpected: %s\nGot:      %s", expected, summary)
	}
}

func TestImportResultsUpdateEntityCounts(t *testing.T) {
	results := &ImportResults{}
	resolved := &yamlflowsimplev2.SimplifiedYAMLResolvedV2{
		HTTPRequests:     []mhttp.HTTP{{}, {}},
		Files:           []mfile.File{{}},
		Flows:           []mflow.Flow{{}, {}, {}},
		FlowNodes:       []mnnode.MNode{{}, {}, {}, {}},
		FlowVariables:   []mflowvariable.FlowVariable{{}},
	}

	results.UpdateEntityCounts(resolved)

	if results.EntityCounts["http_requests"] != 2 {
		t.Errorf("Expected http_requests count 2, got %d", results.EntityCounts["http_requests"])
	}

	if results.EntityCounts["files"] != 1 {
		t.Errorf("Expected files count 1, got %d", results.EntityCounts["files"])
	}

	if results.EntityCounts["flows"] != 3 {
		t.Errorf("Expected flows count 3, got %d", results.EntityCounts["flows"])
	}

	if results.EntityCounts["flow_nodes"] != 4 {
		t.Errorf("Expected flow_nodes count 4, got %d", results.EntityCounts["flow_nodes"])
	}

	if results.EntityCounts["flow_variables"] != 1 {
		t.Errorf("Expected flow_variables count 1, got %d", results.EntityCounts["flow_variables"])
	}
}

func TestImportResultsGetConflictsByType(t *testing.T) {
	results := &ImportResults{
		Conflicts: []ImportConflict{
			{Type: ConflictTypeHTTP, EntityName: "Conflict1"},
			{Type: ConflictTypeFile, EntityName: "Conflict2"},
			{Type: ConflictTypeHTTP, EntityName: "Conflict3"},
		},
	}

	conflictsByType := results.GetConflictsByType()

	httpConflicts := conflictsByType[ConflictTypeHTTP]
	fileConflicts := conflictsByType[ConflictTypeFile]
	nodeConflicts := conflictsByType[ConflictTypeNode]

	if len(httpConflicts) != 2 {
		t.Errorf("Expected 2 HTTP conflicts, got %d", len(httpConflicts))
	}

	if len(fileConflicts) != 1 {
		t.Errorf("Expected 1 file conflict, got %d", len(fileConflicts))
	}

	if len(nodeConflicts) != 0 {
		t.Errorf("Expected 0 node conflicts, got %d", len(nodeConflicts))
	}
}

func TestImportResultsGetErrorsByType(t *testing.T) {
	results := &ImportResults{
		Errors: []ImportError{
			{EntityType: "http", EntityName: "Error1"},
			{EntityType: "file", EntityName: "Error2"},
			{EntityType: "http", EntityName: "Error3"},
		},
	}

	errorsByType := results.GetErrorsByType()

	httpErrors := errorsByType["http"]
	fileErrors := errorsByType["file"]
	flowErrors := errorsByType["flow"]

	if len(httpErrors) != 2 {
		t.Errorf("Expected 2 HTTP errors, got %d", len(httpErrors))
	}

	if len(fileErrors) != 1 {
		t.Errorf("Expected 1 file error, got %d", len(fileErrors))
	}

	if len(flowErrors) != 0 {
		t.Errorf("Expected 0 flow errors, got %d", len(flowErrors))
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	if !config.ValidateURLs {
		t.Errorf("Expected ValidateURLs to be true")
	}

	if !config.ValidateHTTPMethods {
		t.Errorf("Expected ValidateHTTPMethods to be true")
	}

	if !config.ValidateReferences {
		t.Errorf("Expected ValidateReferences to be true")
	}

	if !config.EnableCompression {
		t.Errorf("Expected EnableCompression to be true")
	}

	if config.CompressionThreshold != 1024 {
		t.Errorf("Expected CompressionThreshold 1024, got %d", config.CompressionThreshold)
	}

	if config.MaxConcurrentGoroutines != 10 {
		t.Errorf("Expected MaxConcurrentGoroutines 10, got %d", config.MaxConcurrentGoroutines)
	}

	if config.AutoResolveConflicts {
		t.Errorf("Expected AutoResolveConflicts to be false")
	}

	if config.DefaultConflictResolution != ConflictResolutionSkipped {
		t.Errorf("Expected DefaultConflictResolution Skipped, got %v", config.DefaultConflictResolution)
	}

	if !config.PreserveMetadata {
		t.Errorf("Expected PreserveMetadata to be true")
	}

	if config.GenerateMetadata {
		t.Errorf("Expected GenerateMetadata to be false")
	}

	if config.EnableDetailedLogging {
		t.Errorf("Expected EnableDetailedLogging to be false")
	}

	if config.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got '%s'", config.LogLevel)
	}

	if config.MetricsEnabled {
		t.Errorf("Expected MetricsEnabled to be false")
	}
}

func TestGetDefaultOptions(t *testing.T) {
	workspaceID := idwrap.NewNow()
	options := GetDefaultOptions(workspaceID)

	if options.WorkspaceID.Compare(workspaceID) != 0 {
		t.Errorf("Expected workspace ID to match")
	}

	if options.FolderID != nil {
		t.Errorf("Expected folder ID to be nil")
	}

	if options.MergeStrategy != MergeStrategyCreateNew {
		t.Errorf("Expected MergeStrategy CreateNew, got %v", options.MergeStrategy)
	}

	if options.SkipDuplicates {
		t.Errorf("Expected SkipDuplicates to be false")
	}

	if !options.PreserveExisting {
		t.Errorf("Expected PreserveExisting to be true")
	}

	if options.BatchSize != 100 {
		t.Errorf("Expected BatchSize 100, got %d", options.BatchSize)
	}
}

func TestWorkspaceImportOptionsValidate(t *testing.T) {
	tests := []struct {
		name      string
		options   WorkspaceImportOptions
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid options",
			options: WorkspaceImportOptions{
				WorkspaceID:   idwrap.NewNow(),
				MergeStrategy: MergeStrategyCreateNew,
				BatchSize:     100,
			},
			expectErr: false,
		},
		{
			name: "Empty workspace ID",
			options: WorkspaceImportOptions{
				WorkspaceID:   idwrap.IDWrap{},
				MergeStrategy: MergeStrategyCreateNew,
				BatchSize:     100,
			},
			expectErr: true,
			errMsg:    "workspace ID is required",
		},
		{
			name: "Negative batch size",
			options: WorkspaceImportOptions{
				WorkspaceID:   idwrap.NewNow(),
				MergeStrategy: MergeStrategyCreateNew,
				BatchSize:     -1,
			},
			expectErr: true,
			errMsg:    "batch size must be positive",
		},
		{
			name: "Batch size too large",
			options: WorkspaceImportOptions{
				WorkspaceID:   idwrap.NewNow(),
				MergeStrategy: MergeStrategyCreateNew,
				BatchSize:     20000,
			},
			expectErr: true,
			errMsg:    "batch size too large",
		},
		{
			name: "Invalid merge strategy",
			options: WorkspaceImportOptions{
				WorkspaceID:   idwrap.NewNow(),
				MergeStrategy: MergeStrategy(999),
				BatchSize:     100,
			},
			expectErr: true,
			errMsg:    "invalid merge strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestWorkspaceImportConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    WorkspaceImportConfig
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid config",
			config: WorkspaceImportConfig{
				CompressionThreshold:    1024,
				MaxConcurrentGoroutines: 10,
				LogLevel:               "info",
				DefaultConflictResolution: ConflictResolutionSkipped,
			},
			expectErr: false,
		},
		{
			name: "Negative compression threshold",
			config: WorkspaceImportConfig{
				CompressionThreshold:    -1,
				MaxConcurrentGoroutines: 10,
				LogLevel:               "info",
				DefaultConflictResolution: ConflictResolutionSkipped,
			},
			expectErr: true,
			errMsg:    "compression threshold cannot be negative",
		},
		{
			name: "Zero max concurrent goroutines",
			config: WorkspaceImportConfig{
				CompressionThreshold:    1024,
				MaxConcurrentGoroutines: 0,
				LogLevel:               "info",
				DefaultConflictResolution: ConflictResolutionSkipped,
			},
			expectErr: true,
			errMsg:    "max concurrent goroutines must be positive",
		},
		{
			name: "Max concurrent goroutines too large",
			config: WorkspaceImportConfig{
				CompressionThreshold:    1024,
				MaxConcurrentGoroutines: 2000,
				LogLevel:               "info",
				DefaultConflictResolution: ConflictResolutionSkipped,
			},
			expectErr: true,
			errMsg:    "max concurrent goroutines too large",
		},
		{
			name: "Invalid log level",
			config: WorkspaceImportConfig{
				CompressionThreshold:    1024,
				MaxConcurrentGoroutines: 10,
				LogLevel:               "invalid",
				DefaultConflictResolution: ConflictResolutionSkipped,
			},
			expectErr: true,
			errMsg:    "invalid log level",
		},
		{
			name: "Invalid conflict resolution",
			config: WorkspaceImportConfig{
				CompressionThreshold:    1024,
				MaxConcurrentGoroutines: 10,
				LogLevel:               "info",
				DefaultConflictResolution: ConflictResolution(999),
			},
			expectErr: true,
			errMsg:    "invalid default conflict resolution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestNewImportCache(t *testing.T) {
	cache := NewImportCache()

	if cache.HTTPRequests == nil {
		t.Errorf("HTTPRequests map should be initialized")
	}

	if cache.Files == nil {
		t.Errorf("Files map should be initialized")
	}

	if cache.Flows == nil {
		t.Errorf("Flows map should be initialized")
	}

	if cache.Nodes == nil {
		t.Errorf("Nodes map should be initialized")
	}

	if cache.ResolvedConflicts == nil {
		t.Errorf("ResolvedConflicts map should be initialized")
	}
}

func TestNewImportContext(t *testing.T) {
	ctx := context.Background()
	options := GetDefaultOptions(idwrap.NewNow())
	config := GetDefaultConfig()

	importCtx, err := NewImportContext(ctx, options, config)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if importCtx.Context == nil {
		t.Errorf("Context should be set")
	}

	if importCtx.StartTime <= 0 {
		t.Errorf("StartTime should be positive")
	}

	if importCtx.CancelFunc == nil {
		t.Errorf("CancelFunc should be set")
	}

	if importCtx.Cache == nil {
		t.Errorf("Cache should be initialized")
	}

	if importCtx.State.CurrentStage != "initializing" {
		t.Errorf("Expected initial stage 'initializing', got '%s'", importCtx.State.CurrentStage)
	}

	if importCtx.State.IsCompleted {
		t.Errorf("Initial state should not be completed")
	}
}

func TestNewImportContextInvalidInputs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		options   WorkspaceImportOptions
		config    WorkspaceImportConfig
		expectErr bool
	}{
		{
			name: "Invalid options",
			options: WorkspaceImportOptions{
				WorkspaceID:   idwrap.IDWrap{}, // Invalid
				MergeStrategy: MergeStrategyCreateNew,
				BatchSize:     100,
			},
			config:    GetDefaultConfig(),
			expectErr: true,
		},
		{
			name: "Invalid config",
			options: GetDefaultOptions(idwrap.NewNow()),
			config: WorkspaceImportConfig{
				MaxConcurrentGoroutines: -1, // Invalid
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewImportContext(ctx, tt.options, tt.config)

			if tt.expectErr && err == nil {
				t.Errorf("Expected error, but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}
		})
	}
}