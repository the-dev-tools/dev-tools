package movable

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"the-dev-tools/server/pkg/idwrap"
)

// DefaultScopeResolver implements ScopeResolver interface with efficient context detection and caching
type DefaultScopeResolver struct {
	db    *sql.DB
	cache ContextCache
	mu    sync.RWMutex

	// Prepared statements for performance
	stmts struct {
		getCollectionItem    *sql.Stmt
		getFlowNode          *sql.Stmt
		getItemAPI           *sql.Stmt
		getItemAPIExample    *sql.Stmt
		getCollectionContext *sql.Stmt
		getWorkspaceContext  *sql.Stmt
		getFlowContext       *sql.Stmt
		getItemAPIContext    *sql.Stmt
		getExampleContext    *sql.Stmt
	}

	// Performance configuration
	batchSize     int
	enablePrefetch bool
}

// NewDefaultScopeResolver creates a new DefaultScopeResolver with optimal configuration
func NewDefaultScopeResolver(db *sql.DB, cache ContextCache) (*DefaultScopeResolver, error) {
	resolver := &DefaultScopeResolver{
		db:            db,
		cache:         cache,
		batchSize:     100,
		enablePrefetch: true,
	}

	if err := resolver.prepareStatements(); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return resolver, nil
}

// prepareStatements prepares all SQL statements for optimal performance
func (r *DefaultScopeResolver) prepareStatements() error {
	var err error

	// Collection items detection
	r.stmts.getCollectionItem, err = r.db.Prepare(`
		SELECT collection_id, parent_folder_id, item_type, folder_id, endpoint_id
		FROM collection_items 
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getCollectionItem: %w", err)
	}

	// Flow nodes detection
	r.stmts.getFlowNode, err = r.db.Prepare(`
		SELECT flow_id, node_kind 
		FROM flow_node 
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getFlowNode: %w", err)
	}

	// Item API detection with delta support
	r.stmts.getItemAPI, err = r.db.Prepare(`
		SELECT collection_id, folder_id, delta_parent_id, hidden
		FROM item_api 
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getItemAPI: %w", err)
	}

	// Item API examples detection
	r.stmts.getItemAPIExample, err = r.db.Prepare(`
		SELECT item_api_id, collection_id, version_parent_id
		FROM item_api_example 
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getItemAPIExample: %w", err)
	}

	// Context hierarchy queries
	r.stmts.getCollectionContext, err = r.db.Prepare(`
		SELECT c.workspace_id, c.name as collection_name, w.name as workspace_name
		FROM collections c 
		JOIN workspaces w ON c.workspace_id = w.id
		WHERE c.id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getCollectionContext: %w", err)
	}

	r.stmts.getWorkspaceContext, err = r.db.Prepare(`
		SELECT name FROM workspaces WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getWorkspaceContext: %w", err)
	}

	r.stmts.getFlowContext, err = r.db.Prepare(`
		SELECT workspace_id, name FROM flow WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getFlowContext: %w", err)
	}

	r.stmts.getItemAPIContext, err = r.db.Prepare(`
		SELECT ia.collection_id, ia.folder_id, c.workspace_id, c.name as collection_name
		FROM item_api ia
		JOIN collections c ON ia.collection_id = c.id
		WHERE ia.id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getItemAPIContext: %w", err)
	}

	r.stmts.getExampleContext, err = r.db.Prepare(`
		SELECT iae.item_api_id, iae.collection_id, c.workspace_id
		FROM item_api_example iae
		JOIN collections c ON iae.collection_id = c.id
		WHERE iae.id = ?`)
	if err != nil {
		return fmt.Errorf("prepare getExampleContext: %w", err)
	}

	return nil
}

// ResolveContext determines the MovableContext for a given item ID
func (r *DefaultScopeResolver) ResolveContext(ctx context.Context, itemID idwrap.IDWrap) (MovableContext, error) {
	// Check cache first
	if r.cache != nil {
		if metadata, found := r.cache.GetContext(itemID); found {
			return metadata.Type, nil
		}
	}

	// Try each context type in order of likelihood for performance
	movableContext, err := r.detectContextType(ctx, itemID)
	if err != nil {
		return 0, fmt.Errorf("failed to detect context type for item %s: %w", itemID.String(), err)
	}

	// Cache the result if caching is enabled
	if r.cache != nil {
		metadata := &ContextMetadata{
			Type:      movableContext,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		r.cache.SetContext(itemID, metadata)
	}

	return movableContext, nil
}

// detectContextType performs the actual database queries to determine context
func (r *DefaultScopeResolver) detectContextType(ctx context.Context, itemID idwrap.IDWrap) (MovableContext, error) {
	itemIDBytes := itemID.Bytes()

	// Try collection items first (most common)
	var collectionID, parentFolderID, folderID, endpointID []byte
	var itemType int8
	err := r.stmts.getCollectionItem.QueryRowContext(ctx, itemIDBytes).Scan(
		&collectionID, &parentFolderID, &itemType, &folderID, &endpointID)
	if err == nil {
		return ContextCollection, nil
	} else if err != sql.ErrNoRows {
		return 0, fmt.Errorf("error checking collection_items: %w", err)
	}

	// Try flow nodes
	var flowID []byte
	var nodeKind int
	err = r.stmts.getFlowNode.QueryRowContext(ctx, itemIDBytes).Scan(&flowID, &nodeKind)
	if err == nil {
		return ContextFlow, nil
	} else if err != sql.ErrNoRows {
		return 0, fmt.Errorf("error checking flow_node: %w", err)
	}

	// Try item API (endpoints)
	var apiCollectionID, apiFolderID, deltaParentID []byte
	var hidden bool
	err = r.stmts.getItemAPI.QueryRowContext(ctx, itemIDBytes).Scan(
		&apiCollectionID, &apiFolderID, &deltaParentID, &hidden)
	if err == nil {
		return ContextEndpoint, nil
	} else if err != sql.ErrNoRows {
		return 0, fmt.Errorf("error checking item_api: %w", err)
	}

	// Try item API examples
	var exampleAPIID, exampleCollectionID, versionParentID []byte
	err = r.stmts.getItemAPIExample.QueryRowContext(ctx, itemIDBytes).Scan(
		&exampleAPIID, &exampleCollectionID, &versionParentID)
	if err == nil {
		return ContextRequest, nil // Examples belong to request context
	} else if err != sql.ErrNoRows {
		return 0, fmt.Errorf("error checking item_api_example: %w", err)
	}

	// Try workspace context (variables, environments, etc.)
	var workspaceName string
	err = r.stmts.getWorkspaceContext.QueryRowContext(ctx, itemIDBytes).Scan(&workspaceName)
	if err == nil {
		return ContextWorkspace, nil
	} else if err != sql.ErrNoRows {
		return 0, fmt.Errorf("error checking workspaces: %w", err)
	}

	return 0, fmt.Errorf("item %s not found in any known context", itemID.String())
}

// ResolveScopeID determines the scope ID for a given item and context
func (r *DefaultScopeResolver) ResolveScopeID(ctx context.Context, itemID idwrap.IDWrap, contextType MovableContext) (idwrap.IDWrap, error) {
	// Check cache first
	if r.cache != nil {
		if metadata, found := r.cache.GetContext(itemID); found && metadata.Type == contextType {
			if len(metadata.ScopeID) > 0 {
				scopeID, err := idwrap.NewFromBytes(metadata.ScopeID)
				if err != nil {
					return idwrap.IDWrap{}, fmt.Errorf("failed to decode cached scope ID: %w", err)
				}
				return scopeID, nil
			}
		}
	}

	var scopeID idwrap.IDWrap
	var err error

	switch contextType {
	case ContextCollection:
		scopeID, err = r.resolveCollectionScope(ctx, itemID)
	case ContextFlow:
		scopeID, err = r.resolveFlowScope(ctx, itemID)
	case ContextEndpoint:
		scopeID, err = r.resolveEndpointScope(ctx, itemID)
	case ContextRequest:
		scopeID, err = r.resolveRequestScope(ctx, itemID)
	case ContextWorkspace:
		scopeID, err = r.resolveWorkspaceScope(ctx, itemID)
	default:
		return idwrap.IDWrap{}, fmt.Errorf("unsupported context type: %v", contextType)
	}

	if err != nil {
		return idwrap.IDWrap{}, err
	}

	// Cache the result
	if r.cache != nil {
		metadata := &ContextMetadata{
			Type:      contextType,
			ScopeID:   scopeID.Bytes(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		r.cache.SetContext(itemID, metadata)
	}

	return scopeID, nil
}

// resolveCollectionScope determines the scope for collection items
func (r *DefaultScopeResolver) resolveCollectionScope(ctx context.Context, itemID idwrap.IDWrap) (idwrap.IDWrap, error) {
	var collectionID []byte
	var parentFolderID, folderID, endpointID *[]byte
	var itemType int8

	err := r.stmts.getCollectionItem.QueryRowContext(ctx, itemID.Bytes()).Scan(
		&collectionID, &parentFolderID, &itemType, &folderID, &endpointID)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("collection item not found: %w", err)
	}

	scopeID, err := idwrap.NewFromBytes(collectionID)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("failed to decode collection ID: %w", err)
	}
	return scopeID, nil
}

// resolveFlowScope determines the scope for flow nodes
func (r *DefaultScopeResolver) resolveFlowScope(ctx context.Context, itemID idwrap.IDWrap) (idwrap.IDWrap, error) {
	var flowID []byte
	var nodeKind int

	err := r.stmts.getFlowNode.QueryRowContext(ctx, itemID.Bytes()).Scan(&flowID, &nodeKind)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("flow node not found: %w", err)
	}

	scopeID, err := idwrap.NewFromBytes(flowID)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("failed to decode flow ID: %w", err)
	}
	return scopeID, nil
}

// resolveEndpointScope determines the scope for API endpoints
func (r *DefaultScopeResolver) resolveEndpointScope(ctx context.Context, itemID idwrap.IDWrap) (idwrap.IDWrap, error) {
	var collectionID, folderID []byte
	var deltaParentID *[]byte
	var hidden bool

	err := r.stmts.getItemAPI.QueryRowContext(ctx, itemID.Bytes()).Scan(
		&collectionID, &folderID, &deltaParentID, &hidden)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("item api not found: %w", err)
	}

	// If this is a delta (hidden=true), the scope is the collection
	// If it's a regular endpoint, the scope is also the collection
	scopeID, err := idwrap.NewFromBytes(collectionID)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("failed to decode collection ID: %w", err)
	}
	return scopeID, nil
}

// resolveRequestScope determines the scope for API examples/requests
func (r *DefaultScopeResolver) resolveRequestScope(ctx context.Context, itemID idwrap.IDWrap) (idwrap.IDWrap, error) {
	var apiID, collectionID []byte
	var versionParentID *[]byte

	err := r.stmts.getItemAPIExample.QueryRowContext(ctx, itemID.Bytes()).Scan(
		&apiID, &collectionID, &versionParentID)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("item api example not found: %w", err)
	}

	// The scope for examples is the parent API endpoint
	scopeID, err := idwrap.NewFromBytes(apiID)
	if err != nil {
		return idwrap.IDWrap{}, fmt.Errorf("failed to decode API ID: %w", err)
	}
	return scopeID, nil
}

// resolveWorkspaceScope determines the scope for workspace items
func (r *DefaultScopeResolver) resolveWorkspaceScope(ctx context.Context, itemID idwrap.IDWrap) (idwrap.IDWrap, error) {
	// For workspace-level items, the scope is the item itself (they are top-level)
	return itemID, nil
}

// ValidateScope ensures an item belongs to the expected scope
func (r *DefaultScopeResolver) ValidateScope(ctx context.Context, itemID idwrap.IDWrap, expectedScope idwrap.IDWrap) error {
	// First resolve the actual context
	contextType, err := r.ResolveContext(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to resolve context: %w", err)
	}

	// Then resolve the actual scope
	actualScope, err := r.ResolveScopeID(ctx, itemID, contextType)
	if err != nil {
		return fmt.Errorf("failed to resolve scope: %w", err)
	}

	// Compare scopes
	if actualScope.Compare(expectedScope) != 0 {
		return fmt.Errorf("scope mismatch: expected %s, got %s", expectedScope.String(), actualScope.String())
	}

	return nil
}

// GetScopeHierarchy returns the hierarchical scopes for an item
func (r *DefaultScopeResolver) GetScopeHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error) {
	contextType, err := r.ResolveContext(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve context: %w", err)
	}

	var hierarchy []ScopeLevel

	switch contextType {
	case ContextCollection:
		hierarchy, err = r.buildCollectionHierarchy(ctx, itemID)
	case ContextFlow:
		hierarchy, err = r.buildFlowHierarchy(ctx, itemID)
	case ContextEndpoint:
		hierarchy, err = r.buildEndpointHierarchy(ctx, itemID)
	case ContextRequest:
		hierarchy, err = r.buildRequestHierarchy(ctx, itemID)
	case ContextWorkspace:
		hierarchy, err = r.buildWorkspaceHierarchy(ctx, itemID)
	default:
		return nil, fmt.Errorf("unsupported context type: %v", contextType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to build hierarchy for context %v: %w", contextType, err)
	}

	return hierarchy, nil
}

// buildCollectionHierarchy builds hierarchy for collection items
func (r *DefaultScopeResolver) buildCollectionHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error) {
	// First get the collection ID from the collection item
	collectionID, err := r.resolveCollectionScope(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve collection scope: %w", err)
	}

	// Now get workspace and collection names
	var workspaceID []byte
	var collectionName, workspaceName string

	err = r.stmts.getCollectionContext.QueryRowContext(ctx, collectionID.Bytes()).Scan(
		&workspaceID, &collectionName, &workspaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection context: %w", err)
	}

	workspaceIDWrap, err := idwrap.NewFromBytes(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode workspace ID: %w", err)
	}

	return []ScopeLevel{
		{
			Context: ContextWorkspace,
			ScopeID: workspaceIDWrap,
			Name:    workspaceName,
			Level:   0,
		},
		{
			Context: ContextCollection,
			ScopeID: collectionID,
			Name:    collectionName,
			Level:   1,
		},
	}, nil
}

// buildFlowHierarchy builds hierarchy for flow items
func (r *DefaultScopeResolver) buildFlowHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error) {
	var flowID []byte
	var nodeKind int

	err := r.stmts.getFlowNode.QueryRowContext(ctx, itemID.Bytes()).Scan(&flowID, &nodeKind)
	if err != nil {
		return nil, fmt.Errorf("failed to get flow node: %w", err)
	}

	var workspaceID []byte
	var flowName string
	err = r.stmts.getFlowContext.QueryRowContext(ctx, flowID).Scan(&workspaceID, &flowName)
	if err != nil {
		return nil, fmt.Errorf("failed to get flow context: %w", err)
	}

	var workspaceName string
	err = r.stmts.getWorkspaceContext.QueryRowContext(ctx, workspaceID).Scan(&workspaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace context: %w", err)
	}

	workspaceIDWrap, err := idwrap.NewFromBytes(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode workspace ID: %w", err)
	}

	flowIDWrap, err := idwrap.NewFromBytes(flowID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode flow ID: %w", err)
	}

	return []ScopeLevel{
		{
			Context: ContextWorkspace,
			ScopeID: workspaceIDWrap,
			Name:    workspaceName,
			Level:   0,
		},
		{
			Context: ContextFlow,
			ScopeID: flowIDWrap,
			Name:    flowName,
			Level:   1,
		},
	}, nil
}

// buildEndpointHierarchy builds hierarchy for API endpoints
func (r *DefaultScopeResolver) buildEndpointHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error) {
	var collectionID []byte
	var folderID *[]byte
	var workspaceID []byte
	var collectionName string

	err := r.stmts.getItemAPIContext.QueryRowContext(ctx, itemID.Bytes()).Scan(
		&collectionID, &folderID, &workspaceID, &collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get item API context: %w", err)
	}

	var workspaceName string
	err = r.stmts.getWorkspaceContext.QueryRowContext(ctx, workspaceID).Scan(&workspaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace context: %w", err)
	}

	workspaceIDWrap, err := idwrap.NewFromBytes(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode workspace ID: %w", err)
	}

	collectionIDWrap, err := idwrap.NewFromBytes(collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode collection ID: %w", err)
	}

	hierarchy := []ScopeLevel{
		{
			Context: ContextWorkspace,
			ScopeID: workspaceIDWrap,
			Name:    workspaceName,
			Level:   0,
		},
		{
			Context: ContextCollection,
			ScopeID: collectionIDWrap,
			Name:    collectionName,
			Level:   1,
		},
	}

	// Add folder level if present
	if folderID != nil {
		folderIDWrap, err := idwrap.NewFromBytes(*folderID)
		if err != nil {
			return nil, fmt.Errorf("failed to decode folder ID: %w", err)
		}
		hierarchy = append(hierarchy, ScopeLevel{
			Context: ContextEndpoint,
			ScopeID: folderIDWrap,
			Name:    "Folder", // Could be enhanced to get actual folder name
			Level:   2,
		})
	}

	return hierarchy, nil
}

// buildRequestHierarchy builds hierarchy for request examples
func (r *DefaultScopeResolver) buildRequestHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error) {
	var apiID, collectionID, workspaceID []byte

	err := r.stmts.getExampleContext.QueryRowContext(ctx, itemID.Bytes()).Scan(
		&apiID, &collectionID, &workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get example context: %w", err)
	}

	var workspaceName string
	err = r.stmts.getWorkspaceContext.QueryRowContext(ctx, workspaceID).Scan(&workspaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace context: %w", err)
	}

	workspaceIDWrap, err := idwrap.NewFromBytes(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode workspace ID: %w", err)
	}

	collectionIDWrap, err := idwrap.NewFromBytes(collectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode collection ID: %w", err)
	}

	apiIDWrap, err := idwrap.NewFromBytes(apiID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode API ID: %w", err)
	}

	return []ScopeLevel{
		{
			Context: ContextWorkspace,
			ScopeID: workspaceIDWrap,
			Name:    workspaceName,
			Level:   0,
		},
		{
			Context: ContextCollection,
			ScopeID: collectionIDWrap,
			Name:    "Collection", // Could be enhanced to get actual collection name
			Level:   1,
		},
		{
			Context: ContextEndpoint,
			ScopeID: apiIDWrap,
			Name:    "API Endpoint", // Could be enhanced to get actual endpoint name
			Level:   2,
		},
	}, nil
}

// buildWorkspaceHierarchy builds hierarchy for workspace items
func (r *DefaultScopeResolver) buildWorkspaceHierarchy(ctx context.Context, itemID idwrap.IDWrap) ([]ScopeLevel, error) {
	var workspaceName string
	err := r.stmts.getWorkspaceContext.QueryRowContext(ctx, itemID.Bytes()).Scan(&workspaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace context: %w", err)
	}

	return []ScopeLevel{
		{
			Context: ContextWorkspace,
			ScopeID: itemID,
			Name:    workspaceName,
			Level:   0,
		},
	}, nil
}

// GetContextBoundaries defines valid operation boundaries for each context
func (r *DefaultScopeResolver) GetContextBoundaries(ctx context.Context, contextType MovableContext) (map[string]interface{}, error) {
	boundaries := make(map[string]interface{})

	switch contextType {
	case ContextCollection:
		boundaries["canMoveAcrossWorkspaces"] = false
		boundaries["canMoveAcrossCollections"] = false
		boundaries["allowedTargets"] = []string{"folder", "endpoint", "collection_item"}
		boundaries["maxDepth"] = 5

	case ContextFlow:
		boundaries["canMoveAcrossWorkspaces"] = false
		boundaries["canMoveAcrossFlows"] = false
		boundaries["allowedTargets"] = []string{"flow_node", "flow_edge"}
		boundaries["maxNodes"] = 1000

	case ContextEndpoint:
		boundaries["canMoveAcrossWorkspaces"] = false
		boundaries["canMoveAcrossCollections"] = false
		boundaries["allowedTargets"] = []string{"endpoint", "folder"}
		boundaries["maxDepth"] = 3

	case ContextRequest:
		boundaries["canMoveAcrossWorkspaces"] = false
		boundaries["canMoveAcrossEndpoints"] = false
		boundaries["allowedTargets"] = []string{"example", "header", "query", "body_form"}
		boundaries["maxExamples"] = 50

	case ContextWorkspace:
		boundaries["canMoveAcrossWorkspaces"] = false
		boundaries["allowedTargets"] = []string{"workspace", "environment", "variable", "tag"}
		boundaries["maxWorkspaces"] = 100

	default:
		return nil, fmt.Errorf("unsupported context type: %v", contextType)
	}

	return boundaries, nil
}

// BatchResolveContexts efficiently resolves contexts for multiple items
func (r *DefaultScopeResolver) BatchResolveContexts(ctx context.Context, itemIDs []idwrap.IDWrap) (map[idwrap.IDWrap]MovableContext, error) {
	results := make(map[idwrap.IDWrap]MovableContext)
	
	// Process in batches for performance
	batchSize := r.batchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(itemIDs); i += batchSize {
		end := i + batchSize
		if end > len(itemIDs) {
			end = len(itemIDs)
		}

		batch := itemIDs[i:end]
		batchResults, err := r.batchResolveContextsBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve batch %d-%d: %w", i, end, err)
		}

		for id, context := range batchResults {
			results[id] = context
		}
	}

	return results, nil
}

// batchResolveContextsBatch processes a single batch of item IDs
func (r *DefaultScopeResolver) batchResolveContextsBatch(ctx context.Context, itemIDs []idwrap.IDWrap) (map[idwrap.IDWrap]MovableContext, error) {
	results := make(map[idwrap.IDWrap]MovableContext)

	// Check cache first
	uncached := make([]idwrap.IDWrap, 0, len(itemIDs))
	if r.cache != nil {
		for _, itemID := range itemIDs {
			if metadata, found := r.cache.GetContext(itemID); found {
				results[itemID] = metadata.Type
			} else {
				uncached = append(uncached, itemID)
			}
		}
	} else {
		uncached = itemIDs
	}

	// Resolve uncached items
	for _, itemID := range uncached {
		contextType, err := r.detectContextType(ctx, itemID)
		if err != nil {
			// Continue with other items on individual failures
			continue
		}
		
		results[itemID] = contextType

		// Cache the result
		if r.cache != nil {
			metadata := &ContextMetadata{
				Type:      contextType,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			r.cache.SetContext(itemID, metadata)
		}
	}

	return results, nil
}

// Close closes the resolver and releases resources
func (r *DefaultScopeResolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []error

	// Close all prepared statements
	if r.stmts.getCollectionItem != nil {
		if err := r.stmts.getCollectionItem.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getFlowNode != nil {
		if err := r.stmts.getFlowNode.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getItemAPI != nil {
		if err := r.stmts.getItemAPI.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getItemAPIExample != nil {
		if err := r.stmts.getItemAPIExample.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getCollectionContext != nil {
		if err := r.stmts.getCollectionContext.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getWorkspaceContext != nil {
		if err := r.stmts.getWorkspaceContext.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getFlowContext != nil {
		if err := r.stmts.getFlowContext.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getItemAPIContext != nil {
		if err := r.stmts.getItemAPIContext.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	if r.stmts.getExampleContext != nil {
		if err := r.stmts.getExampleContext.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing statements: %v", errors)
	}

	return nil
}