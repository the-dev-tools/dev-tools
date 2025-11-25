package ioworkspacev2

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
	"time"
)

// IOWorkspaceServiceV2 provides modern workspace import without collections
type IOWorkspaceServiceV2 struct {
	db *sql.DB

	// Modern services
	httpService shttp.HTTPService
	fileService sfile.FileService
	flowService sflow.FlowService

	// Flow node services
	nodeService      snode.NodeService
	requestService   snoderequest.NodeRequestService
	conditionService snodeif.NodeIfService
	noopService      snodenoop.NodeNoopService
	forService       snodefor.NodeForService
	forEachService   snodeforeach.NodeForEachService
	jsService        snodejs.NodeJSService

	// Flow data services
	variableService sflowvariable.FlowVariableService
}

// NewIOWorkspaceServiceV2 creates a new modern workspace import service
func NewIOWorkspaceServiceV2(
	db *sql.DB,
	httpService shttp.HTTPService,
	fileService sfile.FileService,
	flowService sflow.FlowService,
	nodeService snode.NodeService,
	requestService snoderequest.NodeRequestService,
	conditionService snodeif.NodeIfService,
	noopService snodenoop.NodeNoopService,
	forService snodefor.NodeForService,
	forEachService snodeforeach.NodeForEachService,
	jsService snodejs.NodeJSService,
	variableService sflowvariable.FlowVariableService,
) *IOWorkspaceServiceV2 {
	return &IOWorkspaceServiceV2{
		db:               db,
		httpService:      httpService,
		fileService:      fileService,
		flowService:      flowService,
		nodeService:      nodeService,
		requestService:   requestService,
		conditionService: conditionService,
		noopService:      noopService,
		forService:       forService,
		forEachService:   forEachService,
		jsService:        jsService,
		variableService:  variableService,
	}
}

// ImportWorkspace imports simplified YAML data using modern models
func (s *IOWorkspaceServiceV2) ImportWorkspace(
	ctx context.Context,
	resolved yamlflowsimplev2.SimplifiedYAMLResolvedV2,
	options WorkspaceImportOptions,
) (*ImportResults, error) {
	// Validate options
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid import options: %w", err)
	}

	// Create import context
	importCtx, err := NewImportContext(ctx, options, GetDefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create import context: %w", err)
	}
	defer importCtx.CancelFunc()

	// Initialize results
	results := &ImportResults{
		WorkspaceID:  options.WorkspaceID,
		EntityCounts: make(map[string]int),
	}

	// Update entity counts
	results.UpdateEntityCounts(&resolved)

	// Start database transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Report progress
	if options.ProgressCallback != nil {
		options.ProgressCallback(ImportProgress{
			Stage:      "importing",
			Total:      results.GetTotalProcessed(),
			Processed:  0,
			Percentage: 0,
		})
	}

	// Import in the optimal order: flows -> HTTP -> files -> nodes -> edges -> variables
	err = s.importFlows(importCtx.Context, tx, resolved.Flows, options, results)
	if err != nil {
		return nil, fmt.Errorf("failed to import flows: %w", err)
	}

	err = s.importHTTPRequests(importCtx.Context, tx, resolved.HTTPRequests, options, results)
	if err != nil {
		return nil, fmt.Errorf("failed to import HTTP requests: %w", err)
	}

	err = s.importFiles(importCtx.Context, tx, resolved.Files, options, results)
	if err != nil {
		return nil, fmt.Errorf("failed to import files: %w", err)
	}

	err = s.importFlowNodes(importCtx.Context, tx, resolved.FlowNodes, options, results)
	if err != nil {
		return nil, fmt.Errorf("failed to import flow nodes: %w", err)
	}

	err = s.importFlowNodeImplementations(importCtx.Context, tx, resolved, options, results)
	if err != nil {
		return nil, fmt.Errorf("failed to import flow node implementations: %w", err)
	}

	err = s.importFlowVariables(importCtx.Context, tx, resolved.FlowVariables, options, results)
	if err != nil {
		return nil, fmt.Errorf("failed to import flow variables: %w", err)
	}

	// Note: Edges would be imported here if there was an edge service
	// For now, we'll skip edges as they're typically handled by the flow service

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Calculate final duration
	results.Duration = time.Now().UnixMilli() - importCtx.StartTime

	// Report final progress
	if options.ProgressCallback != nil {
		options.ProgressCallback(ImportProgress{
			Stage:      "completed",
			Total:      results.GetTotalProcessed(),
			Processed:  results.GetTotalProcessed(),
			Percentage: 100,
		})
	}

	return results, nil
}

// importFlows imports flow entities with conflict handling
func (s *IOWorkspaceServiceV2) importFlows(
	ctx context.Context,
	tx *sql.Tx,
	flows []mflow.Flow,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	if len(flows) == 0 {
		return nil
	}

	// Get existing flows to detect conflicts
	existingFlows, err := s.flowService.GetFlowsByWorkspaceID(ctx, options.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get existing flows: %w", err)
	}

	// Create map for fast lookup
	existingMap := make(map[string]*mflow.Flow)
	for _, existing := range existingFlows {
		existingMap[existing.Name] = &existing
	}

	// Process flows in batches
	for i := 0; i < len(flows); i += options.BatchSize {
		end := i + options.BatchSize
		if end > len(flows) {
			end = len(flows)
		}

		batch := flows[i:end]
		err := s.processFlowBatch(ctx, tx, batch, existingMap, options, results)
		if err != nil {
			return fmt.Errorf("failed to process flow batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// processFlowBatch processes a batch of flows
func (s *IOWorkspaceServiceV2) processFlowBatch(
	ctx context.Context,
	tx *sql.Tx,
	batch []mflow.Flow,
	existingMap map[string]*mflow.Flow,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	for _, flow := range batch {
		flow.WorkspaceID = options.WorkspaceID
		if options.FolderID != nil {
			// Note: mflow.Flow doesn't have FolderID field, this would be handled differently
		}

		if existing, exists := existingMap[flow.Name]; exists {
			// Handle conflict based on strategy
			switch options.MergeStrategy {
			case MergeStrategySkipDuplicates:
				results.FlowsSkipped++
				conflict := ImportConflict{
					Type:       ConflictTypeFlow,
					EntityID:   existing.ID,
					EntityName: flow.Name,
					FieldName:  "name",
					Existing:   existing,
					New:        &flow,
					Resolution: ConflictResolutionSkipped,
					Message:    fmt.Sprintf("Flow '%s' already exists, skipping", flow.Name),
				}
				results.AddConflict(conflict)
				continue

			case MergeStrategyReplaceExisting:
				flow.ID = existing.ID
				flowTx := s.flowService.TX(tx)
				err := flowTx.UpdateFlow(ctx, flow)
				if err != nil {
					results.FlowsFailed++
					results.AddError(ImportError{
						EntityID:   flow.ID,
						EntityName: flow.Name,
						EntityType: "flow",
						Err:        err,
					})
					continue
				}
				results.FlowsUpdated++
				conflict := ImportConflict{
					Type:       ConflictTypeFlow,
					EntityID:   existing.ID,
					EntityName: flow.Name,
					FieldName:  "name",
					Existing:   existing,
					New:        &flow,
					Resolution: ConflictResolutionReplaced,
					Message:    fmt.Sprintf("Replaced existing flow '%s'", flow.Name),
				}
				results.AddConflict(conflict)

			case MergeStrategyUpdateExisting:
				flow.ID = existing.ID
				// Merge fields from existing flow
				mergedFlow := *existing
				// Note: Flow only has ID, WorkspaceID, VersionParentID, Name, and Duration fields
				// Additional metadata can be added here when the model is extended
				// Add other field merging as needed
				flowTx := s.flowService.TX(tx)
				err := flowTx.UpdateFlow(ctx, mergedFlow)
				if err != nil {
					results.FlowsFailed++
					results.AddError(ImportError{
						EntityID:   flow.ID,
						EntityName: flow.Name,
						EntityType: "flow",
						Err:        err,
					})
					continue
				}
				results.FlowsUpdated++
				conflict := ImportConflict{
					Type:       ConflictTypeFlow,
					EntityID:   existing.ID,
					EntityName: flow.Name,
					FieldName:  "name",
					Existing:   existing,
					New:        &flow,
					Resolution: ConflictResolutionUpdated,
					Message:    fmt.Sprintf("Updated existing flow '%s'", flow.Name),
				}
				results.AddConflict(conflict)

			default:
				// Create new flow (default behavior)
				flowTx := s.flowService.TX(tx)
				err := flowTx.CreateFlow(ctx, flow)
				if err != nil {
					results.FlowsFailed++
					results.AddError(ImportError{
						EntityID:   flow.ID,
						EntityName: flow.Name,
						EntityType: "flow",
						Err:        err,
					})
					continue
				}
				results.FlowsCreated++
			}
		} else {
			// No conflict, create new flow
			flowTx := s.flowService.TX(tx)
			err := flowTx.CreateFlow(ctx, flow)
			if err != nil {
				results.FlowsFailed++
				results.AddError(ImportError{
					EntityID:   flow.ID,
					EntityName: flow.Name,
					EntityType: "flow",
					Err:        err,
				})
				continue
			}
			results.FlowsCreated++
		}
	}

	return nil
}

// importHTTPRequests imports HTTP request entities with conflict handling
func (s *IOWorkspaceServiceV2) importHTTPRequests(
	ctx context.Context,
	tx *sql.Tx,
	requests []mhttp.HTTP,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	if len(requests) == 0 {
		return nil
	}

	// Get existing HTTP requests to detect conflicts
	existingRequests, err := s.httpService.GetByWorkspaceID(ctx, options.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get existing HTTP requests: %w", err)
	}

	// Create map for fast lookup
	existingMap := make(map[string]*mhttp.HTTP)
	for _, existing := range existingRequests {
		key := fmt.Sprintf("%s|%s", existing.Method, existing.Url)
		existingMap[key] = &existing
	}

	// Process HTTP requests in batches
	for i := 0; i < len(requests); i += options.BatchSize {
		end := i + options.BatchSize
		if end > len(requests) {
			end = len(requests)
		}

		batch := requests[i:end]
		err := s.processHTTPRequestBatch(ctx, tx, batch, existingMap, options, results)
		if err != nil {
			return fmt.Errorf("failed to process HTTP request batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// processHTTPRequestBatch processes a batch of HTTP requests
func (s *IOWorkspaceServiceV2) processHTTPRequestBatch(
	ctx context.Context,
	tx *sql.Tx,
	batch []mhttp.HTTP,
	existingMap map[string]*mhttp.HTTP,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	for _, req := range batch {
		req.WorkspaceID = options.WorkspaceID
		if options.FolderID != nil {
			req.FolderID = options.FolderID
		}

		key := fmt.Sprintf("%s|%s", req.Method, req.Url)
		if existing, exists := existingMap[key]; exists {
			// Handle conflict based on strategy
			switch options.MergeStrategy {
			case MergeStrategySkipDuplicates:
				results.HTTPReqsSkipped++
				conflict := ImportConflict{
					Type:       ConflictTypeHTTP,
					EntityID:   existing.ID,
					EntityName: req.Name,
					FieldName:  "method+url",
					Existing:   existing,
					New:        &req,
					Resolution: ConflictResolutionSkipped,
					Message:    fmt.Sprintf("HTTP request %s %s already exists, skipping", req.Method, req.Url),
				}
				results.AddConflict(conflict)
				continue

			case MergeStrategyReplaceExisting:
				req.ID = existing.ID
				err := s.httpService.TX(tx).Update(ctx, &req)
				if err != nil {
					results.HTTPReqsFailed++
					results.AddError(ImportError{
						EntityID:   req.ID,
						EntityName: req.Name,
						EntityType: "http_request",
						Err:        err,
					})
					continue
				}
				results.HTTPReqsUpdated++
				conflict := ImportConflict{
					Type:       ConflictTypeHTTP,
					EntityID:   existing.ID,
					EntityName: req.Name,
					FieldName:  "method+url",
					Existing:   existing,
					New:        &req,
					Resolution: ConflictResolutionReplaced,
					Message:    fmt.Sprintf("Replaced existing HTTP request %s %s", req.Method, req.Url),
				}
				results.AddConflict(conflict)

			default:
				// Create new request (default behavior)
				err := s.httpService.TX(tx).Create(ctx, &req)
				if err != nil {
					results.HTTPReqsFailed++
					results.AddError(ImportError{
						EntityID:   req.ID,
						EntityName: req.Name,
						EntityType: "http_request",
						Err:        err,
					})
					continue
				}
				results.HTTPReqsCreated++
			}
		} else {
			// No conflict, create new request
			err := s.httpService.TX(tx).Create(ctx, &req)
			if err != nil {
				results.HTTPReqsFailed++
				results.AddError(ImportError{
					EntityID:   req.ID,
					EntityName: req.Name,
					EntityType: "http_request",
					Err:        err,
				})
				continue
			}
			results.HTTPReqsCreated++
		}
	}

	return nil
}

// importFiles imports file entities with conflict handling
func (s *IOWorkspaceServiceV2) importFiles(
	ctx context.Context,
	tx *sql.Tx,
	files []mfile.File,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	if len(files) == 0 {
		return nil
	}

	// Get existing files to detect conflicts
	existingFiles, err := s.fileService.ListFilesByWorkspace(ctx, options.WorkspaceID)
	if err != nil {
		return fmt.Errorf("failed to get existing files: %w", err)
	}

	// Create map for fast lookup
	existingMap := make(map[string]*mfile.File)
	for _, existing := range existingFiles {
		existingMap[existing.Name] = &existing
	}

	// Process files in batches
	for i := 0; i < len(files); i += options.BatchSize {
		end := i + options.BatchSize
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]
		err := s.processFileBatch(ctx, tx, batch, existingMap, options, results)
		if err != nil {
			return fmt.Errorf("failed to process file batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// processFileBatch processes a batch of files
func (s *IOWorkspaceServiceV2) processFileBatch(
	ctx context.Context,
	tx *sql.Tx,
	batch []mfile.File,
	existingMap map[string]*mfile.File,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	for _, file := range batch {
		file.WorkspaceID = options.WorkspaceID
		if options.FolderID != nil {
			file.FolderID = options.FolderID
		}

		if existing, exists := existingMap[file.Name]; exists {
			// Handle conflict based on strategy
			switch options.MergeStrategy {
			case MergeStrategySkipDuplicates:
				results.FilesSkipped++
				conflict := ImportConflict{
					Type:       ConflictTypeFile,
					EntityID:   existing.ID,
					EntityName: file.Name,
					FieldName:  "name",
					Existing:   existing,
					New:        &file,
					Resolution: ConflictResolutionSkipped,
					Message:    fmt.Sprintf("File '%s' already exists, skipping", file.Name),
				}
				results.AddConflict(conflict)
				continue

			case MergeStrategyReplaceExisting:
				file.ID = existing.ID
				err := s.fileService.TX(tx).UpdateFile(ctx, &file)
				if err != nil {
					results.FilesFailed++
					results.AddError(ImportError{
						EntityID:   file.ID,
						EntityName: file.Name,
						EntityType: "file",
						Err:        err,
					})
					continue
				}
				results.FilesUpdated++
				conflict := ImportConflict{
					Type:       ConflictTypeFile,
					EntityID:   existing.ID,
					EntityName: file.Name,
					FieldName:  "name",
					Existing:   existing,
					New:        &file,
					Resolution: ConflictResolutionReplaced,
					Message:    fmt.Sprintf("Replaced existing file '%s'", file.Name),
				}
				results.AddConflict(conflict)

			default:
				// Create new file (default behavior)
				err := s.fileService.TX(tx).CreateFile(ctx, &file)
				if err != nil {
					results.FilesFailed++
					results.AddError(ImportError{
						EntityID:   file.ID,
						EntityName: file.Name,
						EntityType: "file",
						Err:        err,
					})
					continue
				}
				results.FilesCreated++
			}
		} else {
			// No conflict, create new file
			err := s.fileService.TX(tx).CreateFile(ctx, &file)
			if err != nil {
				results.FilesFailed++
				results.AddError(ImportError{
					EntityID:   file.ID,
					EntityName: file.Name,
					EntityType: "file",
					Err:        err,
				})
				continue
			}
			results.FilesCreated++
		}
	}

	return nil
}

// importFlowNodes imports flow node entities
func (s *IOWorkspaceServiceV2) importFlowNodes(
	ctx context.Context,
	tx *sql.Tx,
	nodes []mnnode.MNode,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	if len(nodes) == 0 {
		return nil
	}

	// Process nodes individually (simplified implementation)
	for _, node := range nodes {
		nodeTx := s.nodeService.TX(tx)
		err := nodeTx.CreateNode(ctx, node)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", node.ID.String(), err)
		}
		results.NodesCreated++
	}

	return nil
}

// importFlowNodeImplementations imports all flow node implementations
func (s *IOWorkspaceServiceV2) importFlowNodeImplementations(
	ctx context.Context,
	tx *sql.Tx,
	resolved yamlflowsimplev2.SimplifiedYAMLResolvedV2,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	// Import request nodes
	if len(resolved.FlowRequestNodes) > 0 {
		// TODO: Implement batch request node creation
		for _, node := range resolved.FlowRequestNodes {
			err := s.requestService.TX(tx).CreateNodeRequest(ctx, node)
			if err != nil {
				return fmt.Errorf("failed to create request node: %w", err)
			}
		}
		results.NodesCreated += len(resolved.FlowRequestNodes)
	}

	// Import condition nodes
	if len(resolved.FlowConditionNodes) > 0 {
		// TODO: Implement batch condition node creation
		for _, node := range resolved.FlowConditionNodes {
			err := s.conditionService.TX(tx).CreateNodeIf(ctx, node)
			if err != nil {
				return fmt.Errorf("failed to create condition node: %w", err)
			}
		}
		results.NodesCreated += len(resolved.FlowConditionNodes)
	}

	// Import noop nodes
	if len(resolved.FlowNoopNodes) > 0 {
		err := s.noopService.TX(tx).CreateNodeNoopBulk(ctx, resolved.FlowNoopNodes)
		if err != nil {
			return fmt.Errorf("failed to create noop nodes: %w", err)
		}
		results.NodesCreated += len(resolved.FlowNoopNodes)
	}

	// Import for nodes
	if len(resolved.FlowForNodes) > 0 {
		err := s.forService.TX(tx).CreateNodeForBulk(ctx, resolved.FlowForNodes)
		if err != nil {
			return fmt.Errorf("failed to create for nodes: %w", err)
		}
		results.NodesCreated += len(resolved.FlowForNodes)
	}

	// Import for each nodes
	if len(resolved.FlowForEachNodes) > 0 {
		err := s.forEachService.TX(tx).CreateNodeForEachBulk(ctx, resolved.FlowForEachNodes)
		if err != nil {
			return fmt.Errorf("failed to create for each nodes: %w", err)
		}
		results.NodesCreated += len(resolved.FlowForEachNodes)
	}

	// Import JS nodes
	if len(resolved.FlowJSNodes) > 0 {
		err := s.jsService.TX(tx).CreateNodeJSBulk(ctx, resolved.FlowJSNodes)
		if err != nil {
			return fmt.Errorf("failed to create JS nodes: %w", err)
		}
		results.NodesCreated += len(resolved.FlowJSNodes)
	}

	return nil
}

// importFlowVariables imports flow variable entities
func (s *IOWorkspaceServiceV2) importFlowVariables(
	ctx context.Context,
	tx *sql.Tx,
	variables []mflowvariable.FlowVariable,
	options WorkspaceImportOptions,
	results *ImportResults,
) error {
	if len(variables) == 0 {
		return nil
	}

	// Process variables in batches
	for i := 0; i < len(variables); i += options.BatchSize {
		end := i + options.BatchSize
		if end > len(variables) {
			end = len(variables)
		}

		batch := variables[i:end]
		// TODO: Implement batch variable creation
		for _, variable := range batch {
			variableTx := s.variableService.TX(tx)
			err := variableTx.CreateFlowVariable(ctx, variable)
			if err != nil {
				return fmt.Errorf("failed to create variable batch %d-%d: %w", i, end, err)
			}
		}
		results.VariablesCreated += len(batch)
	}

	return nil
}

// ImportWorkspaceFromYAML imports a workspace from YAML data
func (s *IOWorkspaceServiceV2) ImportWorkspaceFromYAML(
	ctx context.Context,
	yamlData []byte,
	options WorkspaceImportOptions,
) (*ImportResults, error) {
	// Convert YAML to modern models
	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(yamlData, yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID: options.WorkspaceID,
		FolderID:    options.FolderID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML: %w", err)
	}

	// Import the resolved data
	return s.ImportWorkspace(ctx, *resolved, options)
}

// ConcurrentImportWorkspace imports a workspace with concurrent processing
func (s *IOWorkspaceServiceV2) ConcurrentImportWorkspace(
	ctx context.Context,
	resolved yamlflowsimplev2.SimplifiedYAMLResolvedV2,
	options WorkspaceImportOptions,
	config WorkspaceImportConfig,
) (*ImportResults, error) {
	// Create import context
	importCtx, err := NewImportContext(ctx, options, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create import context: %w", err)
	}
	defer importCtx.CancelFunc()

	// Initialize results
	results := &ImportResults{
		WorkspaceID:  options.WorkspaceID,
		EntityCounts: make(map[string]int),
	}

	// Update entity counts
	results.UpdateEntityCounts(&resolved)

	// Use worker pool for concurrent processing
	return s.concurrentImport(importCtx, resolved, results)
}

// concurrentImport performs concurrent import using worker pools
func (s *IOWorkspaceServiceV2) concurrentImport(
	importCtx *WorkspaceImportContext,
	resolved yamlflowsimplev2.SimplifiedYAMLResolvedV2,
	results *ImportResults,
) (*ImportResults, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, importCtx.Config.MaxConcurrentGoroutines)

	// Import flows first (serial as they're foundational)
	err := s.importFlows(importCtx.Context, nil, resolved.Flows, importCtx.Options, results)
	if err != nil {
		return nil, fmt.Errorf("failed to import flows: %w", err)
	}

	// Concurrent import tasks
	tasks := []func(context.Context) error{
		func(ctx context.Context) error {
			return s.importHTTPRequests(ctx, nil, resolved.HTTPRequests, importCtx.Options, results)
		},
		func(ctx context.Context) error {
			return s.importFiles(ctx, nil, resolved.Files, importCtx.Options, results)
		},
		func(ctx context.Context) error {
			return s.importFlowNodes(ctx, nil, resolved.FlowNodes, importCtx.Options, results)
		},
		func(ctx context.Context) error {
			return s.importFlowVariables(ctx, nil, resolved.FlowVariables, importCtx.Options, results)
		},
	}

	// Execute tasks concurrently
	for _, task := range tasks {
		wg.Add(1)
		go func(t func(context.Context) error) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := t(importCtx.Context)
			if err != nil {
				mu.Lock()
				results.AddError(ImportError{
					EntityType: "concurrent_task",
					Err:        err,
				})
				mu.Unlock()
			}
		}(task)
	}

	wg.Wait()

	// Calculate final duration
	results.Duration = time.Now().UnixMilli() - importCtx.StartTime

	return results, nil
}
