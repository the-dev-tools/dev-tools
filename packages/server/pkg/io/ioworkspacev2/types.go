package ioworkspacev2

import (
	"context"
	"fmt"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
)

// WorkspaceImportOptions defines configuration options for modern workspace import
type WorkspaceImportOptions struct {
	WorkspaceID      idwrap.IDWrap
	FolderID         *idwrap.IDWrap
	MergeStrategy    MergeStrategy
	SkipDuplicates   bool
	PreserveExisting bool
	ConflictResolver ConflictResolverFunc
	ProgressCallback ProgressCallbackFunc
	BatchSize        int // Number of entities to process in each batch
}

// MergeStrategy defines how to handle conflicts during import
type MergeStrategy int

const (
	// MergeStrategyCreateNew always creates new entities, ignores conflicts
	MergeStrategyCreateNew MergeStrategy = iota
	// MergeStrategyMergeExisting merges with existing entities
	MergeStrategyMergeExisting
	// MergeStrategySkipDuplicates skips entities that already exist
	MergeStrategySkipDuplicates
	// MergeStrategyReplaceExisting replaces existing entities
	MergeStrategyReplaceExisting
	// MergeStrategyUpdateExisting updates existing entities with new data
	MergeStrategyUpdateExisting
)

// String returns the string representation of the merge strategy
func (ms MergeStrategy) String() string {
	switch ms {
	case MergeStrategyCreateNew:
		return "create_new"
	case MergeStrategyMergeExisting:
		return "merge_existing"
	case MergeStrategySkipDuplicates:
		return "skip_duplicates"
	case MergeStrategyReplaceExisting:
		return "replace_existing"
	case MergeStrategyUpdateExisting:
		return "update_existing"
	default:
		return "unknown"
	}
}

// ImportResults contains the results of a modern workspace import operation
type ImportResults struct {
	WorkspaceID      idwrap.IDWrap
	HTTPReqsCreated  int
	HTTPReqsUpdated  int
	HTTPReqsSkipped  int
	HTTPReqsFailed   int
	FilesCreated     int
	FilesUpdated     int
	FilesSkipped     int
	FilesFailed      int
	FlowsCreated     int
	FlowsUpdated     int
	FlowsSkipped     int
	FlowsFailed      int
	NodesCreated     int
	NodesUpdated     int
	NodesSkipped     int
	NodesFailed      int
	EdgesCreated     int
	EdgesUpdated     int
	EdgesSkipped     int
	EdgesFailed      int
	VariablesCreated int
	VariablesUpdated int
	VariablesSkipped int
	VariablesFailed  int
	Conflicts        []ImportConflict
	Errors           []ImportError
	Duration         int64 // Import duration in milliseconds
	EntityCounts     map[string]int
}

// ImportConflict represents a conflict that occurred during import
type ImportConflict struct {
	Type       ConflictType
	EntityID   idwrap.IDWrap
	EntityName string
	FieldName  string
	Existing   interface{}
	New        interface{}
	Resolution ConflictResolution
	Message    string
}

// ConflictType defines the type of conflict
type ConflictType int

const (
	ConflictTypeHTTP ConflictType = iota
	ConflictTypeFile
	ConflictTypeFlow
	ConflictTypeNode
	ConflictTypeEdge
	ConflictTypeVariable
)

// String returns the string representation of the conflict type
func (ct ConflictType) String() string {
	switch ct {
	case ConflictTypeHTTP:
		return "http"
	case ConflictTypeFile:
		return "file"
	case ConflictTypeFlow:
		return "flow"
	case ConflictTypeNode:
		return "node"
	case ConflictTypeEdge:
		return "edge"
	case ConflictTypeVariable:
		return "variable"
	default:
		return "unknown"
	}
}

// ConflictResolution defines how a conflict was resolved
type ConflictResolution int

const (
	ConflictResolutionSkipped ConflictResolution = iota
	ConflictResolutionMerged
	ConflictResolutionReplaced
	ConflictResolutionUpdated
	ConflictResolutionCreated
)

// String returns the string representation of the conflict resolution
func (cr ConflictResolution) String() string {
	switch cr {
	case ConflictResolutionSkipped:
		return "skipped"
	case ConflictResolutionMerged:
		return "merged"
	case ConflictResolutionReplaced:
		return "replaced"
	case ConflictResolutionUpdated:
		return "updated"
	case ConflictResolutionCreated:
		return "created"
	default:
		return "unknown"
	}
}

// ImportError represents an error that occurred during import
type ImportError struct {
	EntityID   idwrap.IDWrap
	EntityName string
	EntityType string
	Err        error
	Context    map[string]interface{}
}

// Error returns the error message
func (ie ImportError) Error() string {
	if ie.EntityName != "" {
		return fmt.Sprintf("import error for %s '%s': %v", ie.EntityType, ie.EntityName, ie.Err)
	}
	return fmt.Sprintf("import error for %s: %v", ie.EntityType, ie.Err)
}

// ConflictResolverFunc is a function that resolves conflicts during import
type ConflictResolverFunc func(conflict ImportConflict) (ConflictResolution, error)

// ProgressCallbackFunc is a function that reports progress during import
type ProgressCallbackFunc func(progress ImportProgress)

// ImportProgress reports the current progress of an import operation
type ImportProgress struct {
	Stage          string  // Current stage of import
	Total          int     // Total items to process
	Processed      int     // Items processed so far
	Success        int     // Items successfully processed
	Failed         int     // Items that failed
	Percentage     float64 // Completion percentage
	CurrentItem    string  // Name of current item being processed
	EstimatedTime  int64   // Estimated remaining time in milliseconds
	BytesProcessed int64   // Bytes processed so far
	TotalBytes     int64   // Total bytes to process
}

// WorkspaceImportConfig provides additional configuration for import
type WorkspaceImportConfig struct {
	// Validation settings
	ValidateURLs        bool
	ValidateHTTPMethods bool
	ValidateReferences  bool

	// Performance settings
	EnableCompression       bool
	CompressionThreshold    int // Minimum size in bytes to compress
	MaxConcurrentGoroutines int

	// Conflict handling
	AutoResolveConflicts      bool
	DefaultConflictResolution ConflictResolution

	// Data transformation
	TransformHTTPHeaders func([]string) []string
	TransformQueryParams func(map[string]string) map[string]string
	TransformBody        func([]byte) []byte

	// Metadata handling
	PreserveMetadata bool
	GenerateMetadata bool
	MetadataTemplate map[string]interface{}

	// Logging and monitoring
	EnableDetailedLogging bool
	LogLevel              string
	MetricsEnabled        bool
}

// GetDefaultConfig returns a default configuration for workspace import
func GetDefaultConfig() WorkspaceImportConfig {
	return WorkspaceImportConfig{
		ValidateURLs:              true,
		ValidateHTTPMethods:       true,
		ValidateReferences:        true,
		EnableCompression:         true,
		CompressionThreshold:      1024,
		MaxConcurrentGoroutines:   10,
		AutoResolveConflicts:      false,
		DefaultConflictResolution: ConflictResolutionSkipped,
		PreserveMetadata:          true,
		GenerateMetadata:          false,
		EnableDetailedLogging:     false,
		LogLevel:                  "info",
		MetricsEnabled:            false,
	}
}

// GetDefaultOptions returns default options for workspace import
func GetDefaultOptions(workspaceID idwrap.IDWrap) WorkspaceImportOptions {
	return WorkspaceImportOptions{
		WorkspaceID:      workspaceID,
		FolderID:         nil,
		MergeStrategy:    MergeStrategyCreateNew,
		SkipDuplicates:   false,
		PreserveExisting: true,
		BatchSize:        100,
	}
}

// Validation functions

// Validate validates the workspace import options
func (opts WorkspaceImportOptions) Validate() error {
	if opts.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
		return fmt.Errorf("workspace ID is required")
	}

	if opts.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	if opts.BatchSize > 10000 {
		return fmt.Errorf("batch size too large (max 10000)")
	}

	// Validate merge strategy
	switch opts.MergeStrategy {
	case MergeStrategyCreateNew, MergeStrategyMergeExisting, MergeStrategySkipDuplicates,
		MergeStrategyReplaceExisting, MergeStrategyUpdateExisting:
		// Valid strategies
	default:
		return fmt.Errorf("invalid merge strategy: %v", opts.MergeStrategy)
	}

	return nil
}

// Validate validates the workspace import configuration
func (config WorkspaceImportConfig) Validate() error {
	if config.CompressionThreshold < 0 {
		return fmt.Errorf("compression threshold cannot be negative")
	}

	if config.MaxConcurrentGoroutines <= 0 {
		return fmt.Errorf("max concurrent goroutines must be positive")
	}

	if config.MaxConcurrentGoroutines > 1000 {
		return fmt.Errorf("max concurrent goroutines too large (max 1000)")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[config.LogLevel] {
		return fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	// Validate conflict resolution
	switch config.DefaultConflictResolution {
	case ConflictResolutionSkipped, ConflictResolutionMerged, ConflictResolutionReplaced,
		ConflictResolutionUpdated, ConflictResolutionCreated:
		// Valid resolutions
	default:
		return fmt.Errorf("invalid default conflict resolution: %v", config.DefaultConflictResolution)
	}

	return nil
}

// Helper functions for working with import results

// AddConflict adds a conflict to the import results
func (ir *ImportResults) AddConflict(conflict ImportConflict) {
	ir.Conflicts = append(ir.Conflicts, conflict)
}

// AddError adds an error to the import results
func (ir *ImportResults) AddError(err ImportError) {
	ir.Errors = append(ir.Errors, err)
}

// HasConflicts returns true if there are any conflicts
func (ir *ImportResults) HasConflicts() bool {
	return len(ir.Conflicts) > 0
}

// HasErrors returns true if there are any errors
func (ir *ImportResults) HasErrors() bool {
	return len(ir.Errors) > 0
}

// IsSuccess returns true if the import was successful (no critical errors)
func (ir *ImportResults) IsSuccess() bool {
	return len(ir.Errors) == 0
}

// GetTotalProcessed returns the total number of entities processed
func (ir *ImportResults) GetTotalProcessed() int {
	return ir.HTTPReqsCreated + ir.HTTPReqsUpdated + ir.HTTPReqsSkipped + ir.HTTPReqsFailed +
		ir.FilesCreated + ir.FilesUpdated + ir.FilesSkipped + ir.FilesFailed +
		ir.FlowsCreated + ir.FlowsUpdated + ir.FlowsSkipped + ir.FlowsFailed +
		ir.NodesCreated + ir.NodesUpdated + ir.NodesSkipped + ir.NodesFailed +
		ir.EdgesCreated + ir.EdgesUpdated + ir.EdgesSkipped + ir.EdgesFailed +
		ir.VariablesCreated + ir.VariablesUpdated + ir.VariablesSkipped + ir.VariablesFailed
}

// GetSuccessRate returns the success rate as a percentage
func (ir *ImportResults) GetSuccessRate() float64 {
	total := ir.GetTotalProcessed()
	if total == 0 {
		return 0
	}
	successful := ir.HTTPReqsCreated + ir.HTTPReqsUpdated + ir.FilesCreated + ir.FilesUpdated +
		ir.FlowsCreated + ir.FlowsUpdated + ir.NodesCreated + ir.NodesUpdated +
		ir.EdgesCreated + ir.EdgesUpdated + ir.VariablesCreated + ir.VariablesUpdated
	return float64(successful) / float64(total) * 100
}

// Summary returns a human-readable summary of the import results
func (ir *ImportResults) Summary() string {
	return fmt.Sprintf(
		"Import completed in %dms. "+
			"HTTP: %d created, %d updated, %d skipped, %d failed. "+
			"Files: %d created, %d updated, %d skipped, %d failed. "+
			"Flows: %d created, %d updated, %d skipped, %d failed. "+
			"Success rate: %.1f%%",
		ir.Duration,
		ir.HTTPReqsCreated, ir.HTTPReqsUpdated, ir.HTTPReqsSkipped, ir.HTTPReqsFailed,
		ir.FilesCreated, ir.FilesUpdated, ir.FilesSkipped, ir.FilesFailed,
		ir.FlowsCreated, ir.FlowsUpdated, ir.FlowsSkipped, ir.FlowsFailed,
		ir.GetSuccessRate(),
	)
}

// UpdateEntityCounts updates the entity counts in the results
func (ir *ImportResults) UpdateEntityCounts(resolved *yamlflowsimplev2.SimplifiedYAMLResolvedV2) {
	if ir.EntityCounts == nil {
		ir.EntityCounts = make(map[string]int)
	}

	ir.EntityCounts["http_requests"] = len(resolved.HTTPRequests)
	ir.EntityCounts["files"] = len(resolved.Files)
	ir.EntityCounts["flows"] = len(resolved.Flows)
	ir.EntityCounts["flow_nodes"] = len(resolved.FlowNodes)
	ir.EntityCounts["flow_edges"] = len(resolved.FlowEdges)
	ir.EntityCounts["flow_variables"] = len(resolved.FlowVariables)
	ir.EntityCounts["flow_request_nodes"] = len(resolved.FlowRequestNodes)
	ir.EntityCounts["flow_condition_nodes"] = len(resolved.FlowConditionNodes)
	ir.EntityCounts["flow_noop_nodes"] = len(resolved.FlowNoopNodes)
	ir.EntityCounts["flow_for_nodes"] = len(resolved.FlowForNodes)
	ir.EntityCounts["flow_foreach_nodes"] = len(resolved.FlowForEachNodes)
	ir.EntityCounts["flow_js_nodes"] = len(resolved.FlowJSNodes)
	ir.EntityCounts["headers"] = len(resolved.Headers)
	ir.EntityCounts["search_params"] = len(resolved.SearchParams)
	ir.EntityCounts["body_forms"] = len(resolved.BodyForms)
	ir.EntityCounts["body_urlencoded"] = len(resolved.BodyUrlencoded)
	ir.EntityCounts["body_raw"] = len(resolved.BodyRaw)
}

// GetConflictsByType returns conflicts grouped by type
func (ir *ImportResults) GetConflictsByType() map[ConflictType][]ImportConflict {
	conflictsByType := make(map[ConflictType][]ImportConflict)
	for _, conflict := range ir.Conflicts {
		conflictsByType[conflict.Type] = append(conflictsByType[conflict.Type], conflict)
	}
	return conflictsByType
}

// GetErrorsByType returns errors grouped by entity type
func (ir *ImportResults) GetErrorsByType() map[string][]ImportError {
	errorsByType := make(map[string][]ImportError)
	for _, err := range ir.Errors {
		errorsByType[err.EntityType] = append(errorsByType[err.EntityType], err)
	}
	return errorsByType
}

// WorkspaceImportContext provides context for the import operation
type WorkspaceImportContext struct {
	Context    context.Context
	Options    WorkspaceImportOptions
	Config     WorkspaceImportConfig
	StartTime  int64
	CancelFunc context.CancelFunc
	Metrics    ImportMetrics
	State      ImportState
	Cache      *ImportCache
}

// ImportMetrics tracks metrics during import
type ImportMetrics struct {
	StartTime         int64
	EndTime           int64
	Duration          int64
	EntitiesProcessed int
	BytesProcessed    int64
	MemoryUsed        int64
	GoroutinesUsed    int
	DatabaseQueries   int
	CacheHits         int
	CacheMisses       int
}

// ImportState tracks the current state of the import
type ImportState struct {
	CurrentStage   string
	TotalItems     int
	ProcessedItems int
	CurrentItem    string
	LastEntityID   idwrap.IDWrap
	LastError      error
	IsPaused       bool
	IsCancelled    bool
	IsCompleted    bool
}

// ImportCache provides caching for import operations
type ImportCache struct {
	HTTPRequests      map[string]idwrap.IDWrap // URL+method -> ID mapping
	Files             map[string]idwrap.IDWrap // Name -> ID mapping
	Flows             map[string]idwrap.IDWrap // Name -> ID mapping
	Nodes             map[string]idwrap.IDWrap // Name -> ID mapping
	ResolvedConflicts map[string]bool          // Conflict resolutions
}

// NewImportCache creates a new import cache
func NewImportCache() *ImportCache {
	return &ImportCache{
		HTTPRequests:      make(map[string]idwrap.IDWrap),
		Files:             make(map[string]idwrap.IDWrap),
		Flows:             make(map[string]idwrap.IDWrap),
		Nodes:             make(map[string]idwrap.IDWrap),
		ResolvedConflicts: make(map[string]bool),
	}
}

// NewImportContext creates a new import context
func NewImportContext(ctx context.Context, options WorkspaceImportOptions, config WorkspaceImportConfig) (*WorkspaceImportContext, error) {
	// Validate inputs
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create cancellable context
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	return &WorkspaceImportContext{
		Context:    cancelCtx,
		Options:    options,
		Config:     config,
		StartTime:  getCurrentTimeMillis(),
		CancelFunc: cancelFunc,
		Cache:      NewImportCache(),
		State: ImportState{
			CurrentStage:   "initializing",
			TotalItems:     0,
			ProcessedItems: 0,
			IsCompleted:    false,
		},
	}, nil
}

// getCurrentTimeMillis returns the current time in milliseconds
func getCurrentTimeMillis() int64 {
	return time.Now().UnixMilli()
}
