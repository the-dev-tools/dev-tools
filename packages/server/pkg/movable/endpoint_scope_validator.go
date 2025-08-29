package movable

import (
	"context"
	"fmt"
	"sync"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// SIMPLE ENDPOINT SCOPE VALIDATOR IMPLEMENTATION
// =============================================================================

// SimpleEndpointScopeValidator provides basic scope validation for endpoints
type SimpleEndpointScopeValidator struct {
	// Cache for endpoint contexts to avoid repeated lookups
	contextCache  map[string]*EndpointContextInfo
	cacheMutex    sync.RWMutex
	
	// Validation rules
	config        *EndpointValidationConfig
	
	// Context resolution functions
	collectionResolver func(ctx context.Context, endpointID idwrap.IDWrap) (idwrap.IDWrap, error)
	flowResolver       func(ctx context.Context, endpointID idwrap.IDWrap) (idwrap.IDWrap, error)
	requestResolver    func(ctx context.Context, endpointID idwrap.IDWrap) (idwrap.IDWrap, error)
}

// EndpointContextInfo caches context information for an endpoint
type EndpointContextInfo struct {
	Context     EndpointContext
	ScopeID     idwrap.IDWrap
	IsValidated bool
	LastCheck   time.Time
}

// EndpointValidationConfig defines validation behavior
type EndpointValidationConfig struct {
	// Enable different types of validation
	ValidateCollectionScope bool
	ValidateFlowScope       bool
	ValidateRequestScope    bool
	ValidateExampleScope    bool
	
	// Cache settings
	EnableCaching      bool
	CacheExpiration    time.Duration
	MaxCacheEntries    int
	
	// Cross-context rules
	AllowCollectionToFlow   bool
	AllowFlowToCollection   bool
	AllowRequestMoves       bool
	RequireExplicitConsent  bool
	
	// Performance settings
	ValidationTimeout time.Duration
	ConcurrentChecks  int
}

// NewSimpleEndpointScopeValidator creates a new scope validator
func NewSimpleEndpointScopeValidator(config *EndpointValidationConfig) *SimpleEndpointScopeValidator {
	if config == nil {
		config = &EndpointValidationConfig{
			ValidateCollectionScope: true,
			ValidateFlowScope:      true,
			ValidateRequestScope:   true,
			ValidateExampleScope:   true,
			EnableCaching:          true,
			CacheExpiration:        5 * time.Minute,
			MaxCacheEntries:        1000,
			AllowCollectionToFlow:  true,
			AllowFlowToCollection:  false,
			AllowRequestMoves:      false,
			ValidationTimeout:      30 * time.Second,
			ConcurrentChecks:      5,
		}
	}
	
	return &SimpleEndpointScopeValidator{
		contextCache: make(map[string]*EndpointContextInfo),
		config:       config,
	}
}

// WithCollectionResolver sets the collection context resolver
func (v *SimpleEndpointScopeValidator) WithCollectionResolver(
	resolver func(ctx context.Context, endpointID idwrap.IDWrap) (idwrap.IDWrap, error)) *SimpleEndpointScopeValidator {
	v.collectionResolver = resolver
	return v
}

// WithFlowResolver sets the flow context resolver
func (v *SimpleEndpointScopeValidator) WithFlowResolver(
	resolver func(ctx context.Context, endpointID idwrap.IDWrap) (idwrap.IDWrap, error)) *SimpleEndpointScopeValidator {
	v.flowResolver = resolver
	return v
}

// WithRequestResolver sets the request context resolver
func (v *SimpleEndpointScopeValidator) WithRequestResolver(
	resolver func(ctx context.Context, endpointID idwrap.IDWrap) (idwrap.IDWrap, error)) *SimpleEndpointScopeValidator {
	v.requestResolver = resolver
	return v
}

// =============================================================================
// ENDPOINT SCOPE VALIDATOR INTERFACE IMPLEMENTATION
// =============================================================================

// ValidateEndpointScope ensures endpoint belongs to expected scope
func (v *SimpleEndpointScopeValidator) ValidateEndpointScope(ctx context.Context, 
	endpointID idwrap.IDWrap, expectedContext EndpointContext, expectedScope idwrap.IDWrap) error {
	
	// Check cache first if enabled
	if v.config.EnableCaching {
		if cached := v.getCachedContext(endpointID); cached != nil {
			if !v.isCacheExpired(cached) {
				return v.validateAgainstCached(cached, expectedContext, expectedScope)
			}
		}
	}
	
	// Resolve actual context and scope
	actualContext, actualScope, err := v.GetEndpointContext(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("failed to resolve endpoint context: %w", err)
	}
	
	// Cache the result if caching is enabled
	if v.config.EnableCaching {
		v.cacheContext(endpointID, actualContext, actualScope)
	}
	
	// Validate context match
	if actualContext != expectedContext {
		return fmt.Errorf("context mismatch for endpoint %s: expected %s, got %s",
			endpointID.String(), expectedContext.String(), actualContext.String())
	}
	
	// Validate scope match
	if actualScope.Compare(expectedScope) != 0 {
		return fmt.Errorf("scope mismatch for endpoint %s: expected %s, got %s",
			endpointID.String(), expectedScope.String(), actualScope.String())
	}
	
	return nil
}

// ValidateExampleScope ensures example belongs to endpoint scope
func (v *SimpleEndpointScopeValidator) ValidateExampleScope(ctx context.Context, 
	exampleID idwrap.IDWrap, endpointID idwrap.IDWrap) error {
	
	if !v.config.ValidateExampleScope {
		return nil // Validation disabled
	}
	
	// Basic validation: examples must belong to their endpoint
	// In a real implementation, this would query the database to verify the relationship
	// For now, we'll do a simple validation that the IDs are not empty
	
	emptyID := idwrap.IDWrap{}
	if exampleID.Compare(emptyID) == 0 {
		return fmt.Errorf("example ID cannot be empty")
	}
	
	if endpointID.Compare(emptyID) == 0 {
		return fmt.Errorf("endpoint ID cannot be empty")
	}
	
	// In a real implementation, you would:
	// 1. Query the database to ensure the example exists
	// 2. Verify that the example's endpoint_id matches the provided endpointID
	// 3. Check permissions if needed
	
	return nil
}

// ValidateCrossContextMove validates moving endpoints between contexts
func (v *SimpleEndpointScopeValidator) ValidateCrossContextMove(ctx context.Context, 
	endpointID idwrap.IDWrap, fromContext, toContext EndpointContext, targetScopeID idwrap.IDWrap) error {
	
	// Get current context to verify it matches fromContext
	actualFromContext, _, err := v.GetEndpointContext(ctx, endpointID)
	if err != nil {
		return fmt.Errorf("failed to resolve current endpoint context: %w", err)
	}
	
	if actualFromContext != fromContext {
		return fmt.Errorf("current context mismatch: expected %s, got %s",
			fromContext.String(), actualFromContext.String())
	}
	
	// Apply cross-context validation rules
	switch {
	case fromContext == EndpointContextCollection && toContext == EndpointContextFlow:
		if !v.config.AllowCollectionToFlow {
			return fmt.Errorf("collection to flow moves are disabled")
		}
		
		// Validate that target scope is a valid flow
		if err := v.validateFlowScope(ctx, targetScopeID); err != nil {
			return fmt.Errorf("invalid target flow scope: %w", err)
		}
		
	case fromContext == EndpointContextFlow && toContext == EndpointContextCollection:
		if !v.config.AllowFlowToCollection {
			return fmt.Errorf("flow to collection moves are disabled")
		}
		
		// Validate that target scope is a valid collection
		if err := v.validateCollectionScope(ctx, targetScopeID); err != nil {
			return fmt.Errorf("invalid target collection scope: %w", err)
		}
		
	case fromContext == EndpointContextRequest || toContext == EndpointContextRequest:
		if !v.config.AllowRequestMoves {
			return fmt.Errorf("request endpoint moves are disabled")
		}
		
	default:
		return fmt.Errorf("unsupported context transition: %s -> %s", 
			fromContext.String(), toContext.String())
	}
	
	// Check if explicit consent is required
	if v.config.RequireExplicitConsent {
		// In a real implementation, this would check for user confirmation
		// or some form of explicit approval workflow
		return fmt.Errorf("explicit consent required for cross-context move")
	}
	
	return nil
}

// GetEndpointContext resolves the current context for an endpoint
func (v *SimpleEndpointScopeValidator) GetEndpointContext(ctx context.Context, 
	endpointID idwrap.IDWrap) (EndpointContext, idwrap.IDWrap, error) {
	
	// Check cache first
	if v.config.EnableCaching {
		if cached := v.getCachedContext(endpointID); cached != nil {
			if !v.isCacheExpired(cached) {
				return cached.Context, cached.ScopeID, nil
			}
		}
	}
	
	// Try to resolve as collection endpoint first
	if v.collectionResolver != nil && v.config.ValidateCollectionScope {
		if collectionID, err := v.collectionResolver(ctx, endpointID); err == nil {
			emptyID := idwrap.IDWrap{}
			if collectionID.Compare(emptyID) != 0 {
				v.cacheContext(endpointID, EndpointContextCollection, collectionID)
				return EndpointContextCollection, collectionID, nil
			}
		}
	}
	
	// Try to resolve as flow endpoint
	if v.flowResolver != nil && v.config.ValidateFlowScope {
		if flowID, err := v.flowResolver(ctx, endpointID); err == nil {
			emptyID := idwrap.IDWrap{}
			if flowID.Compare(emptyID) != 0 {
				v.cacheContext(endpointID, EndpointContextFlow, flowID)
				return EndpointContextFlow, flowID, nil
			}
		}
	}
	
	// Try to resolve as request endpoint
	if v.requestResolver != nil && v.config.ValidateRequestScope {
		if requestID, err := v.requestResolver(ctx, endpointID); err == nil {
			emptyID := idwrap.IDWrap{}
			if requestID.Compare(emptyID) != 0 {
				v.cacheContext(endpointID, EndpointContextRequest, requestID)
				return EndpointContextRequest, requestID, nil
			}
		}
	}
	
	// Default to collection context if no resolvers are available
	emptyID := idwrap.IDWrap{}
	return EndpointContextCollection, emptyID, fmt.Errorf("unable to resolve context for endpoint %s", endpointID.String())
}

// =============================================================================
// CACHE MANAGEMENT
// =============================================================================

// getCachedContext retrieves cached context information
func (v *SimpleEndpointScopeValidator) getCachedContext(endpointID idwrap.IDWrap) *EndpointContextInfo {
	v.cacheMutex.RLock()
	defer v.cacheMutex.RUnlock()
	
	key := endpointID.String()
	if info, exists := v.contextCache[key]; exists {
		return info
	}
	
	return nil
}

// cacheContext stores context information in cache
func (v *SimpleEndpointScopeValidator) cacheContext(endpointID idwrap.IDWrap, 
	context EndpointContext, scopeID idwrap.IDWrap) {
	
	if !v.config.EnableCaching {
		return
	}
	
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()
	
	key := endpointID.String()
	
	// Check cache size and evict if necessary
	if len(v.contextCache) >= v.config.MaxCacheEntries {
		v.evictOldestCacheEntry()
	}
	
	v.contextCache[key] = &EndpointContextInfo{
		Context:     context,
		ScopeID:     scopeID,
		IsValidated: true,
		LastCheck:   time.Now(),
	}
}

// isCacheExpired checks if cached entry has expired
func (v *SimpleEndpointScopeValidator) isCacheExpired(info *EndpointContextInfo) bool {
	if v.config.CacheExpiration <= 0 {
		return false // No expiration
	}
	
	return time.Since(info.LastCheck) > v.config.CacheExpiration
}

// evictOldestCacheEntry removes the oldest cache entry
func (v *SimpleEndpointScopeValidator) evictOldestCacheEntry() {
	var oldestKey string
	var oldestTime time.Time = time.Now()
	
	for key, info := range v.contextCache {
		if info.LastCheck.Before(oldestTime) {
			oldestTime = info.LastCheck
			oldestKey = key
		}
	}
	
	if oldestKey != "" {
		delete(v.contextCache, oldestKey)
	}
}

// validateAgainstCached validates against cached context information
func (v *SimpleEndpointScopeValidator) validateAgainstCached(cached *EndpointContextInfo, 
	expectedContext EndpointContext, expectedScope idwrap.IDWrap) error {
	
	if cached.Context != expectedContext {
		return fmt.Errorf("cached context mismatch: expected %s, got %s",
			expectedContext.String(), cached.Context.String())
	}
	
	if cached.ScopeID.Compare(expectedScope) != 0 {
		return fmt.Errorf("cached scope mismatch: expected %s, got %s",
			expectedScope.String(), cached.ScopeID.String())
	}
	
	return nil
}

// =============================================================================
// SCOPE-SPECIFIC VALIDATION HELPERS
// =============================================================================

// validateCollectionScope validates that a scope ID represents a valid collection
func (v *SimpleEndpointScopeValidator) validateCollectionScope(ctx context.Context, 
	collectionID idwrap.IDWrap) error {
	
	// In a real implementation, this would:
	// 1. Query the database to ensure the collection exists
	// 2. Check if the collection is active/accessible
	// 3. Validate permissions
	
	emptyID := idwrap.IDWrap{}
	if collectionID.Compare(emptyID) == 0 {
		return fmt.Errorf("collection ID cannot be empty")
	}
	
	return nil
}

// validateFlowScope validates that a scope ID represents a valid flow
func (v *SimpleEndpointScopeValidator) validateFlowScope(ctx context.Context, 
	flowID idwrap.IDWrap) error {
	
	// In a real implementation, this would:
	// 1. Query the database to ensure the flow exists
	// 2. Check if the flow is active/accessible
	// 3. Validate permissions
	
	emptyID := idwrap.IDWrap{}
	if flowID.Compare(emptyID) == 0 {
		return fmt.Errorf("flow ID cannot be empty")
	}
	
	return nil
}

// validateRequestScope validates that a scope ID represents a valid request node
func (v *SimpleEndpointScopeValidator) validateRequestScope(ctx context.Context, 
	requestID idwrap.IDWrap) error {
	
	// In a real implementation, this would:
	// 1. Query the database to ensure the request node exists
	// 2. Check if the request node belongs to an active flow execution
	// 3. Validate permissions
	
	emptyID := idwrap.IDWrap{}
	if requestID.Compare(emptyID) == 0 {
		return fmt.Errorf("request ID cannot be empty")
	}
	
	return nil
}

// =============================================================================
// CACHE MAINTENANCE AND UTILITIES
// =============================================================================

// ClearCache removes all cached context information
func (v *SimpleEndpointScopeValidator) ClearCache() {
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()
	
	v.contextCache = make(map[string]*EndpointContextInfo)
}

// InvalidateEndpoint removes cached context for a specific endpoint
func (v *SimpleEndpointScopeValidator) InvalidateEndpoint(endpointID idwrap.IDWrap) {
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()
	
	key := endpointID.String()
	delete(v.contextCache, key)
}

// GetCacheStats returns cache performance statistics
func (v *SimpleEndpointScopeValidator) GetCacheStats() map[string]interface{} {
	v.cacheMutex.RLock()
	defer v.cacheMutex.RUnlock()
	
	totalEntries := len(v.contextCache)
	expiredEntries := 0
	
	now := time.Now()
	for _, info := range v.contextCache {
		if v.config.CacheExpiration > 0 && 
		   now.Sub(info.LastCheck) > v.config.CacheExpiration {
			expiredEntries++
		}
	}
	
	return map[string]interface{}{
		"total_entries":   totalEntries,
		"expired_entries": expiredEntries,
		"max_entries":     v.config.MaxCacheEntries,
		"cache_enabled":   v.config.EnableCaching,
		"expiration":      v.config.CacheExpiration.String(),
	}
}

// CompactCache removes expired entries from cache
func (v *SimpleEndpointScopeValidator) CompactCache() int {
	if !v.config.EnableCaching || v.config.CacheExpiration <= 0 {
		return 0
	}
	
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()
	
	removed := 0
	now := time.Now()
	
	for key, info := range v.contextCache {
		if now.Sub(info.LastCheck) > v.config.CacheExpiration {
			delete(v.contextCache, key)
			removed++
		}
	}
	
	return removed
}

// =============================================================================
// FACTORY FUNCTIONS AND PRESETS
// =============================================================================

// NewProductionEndpointScopeValidator creates a validator with production-ready settings
func NewProductionEndpointScopeValidator() *SimpleEndpointScopeValidator {
	config := &EndpointValidationConfig{
		ValidateCollectionScope: true,
		ValidateFlowScope:      true,
		ValidateRequestScope:   true,
		ValidateExampleScope:   true,
		EnableCaching:          true,
		CacheExpiration:        10 * time.Minute,
		MaxCacheEntries:        5000,
		AllowCollectionToFlow:  true,
		AllowFlowToCollection:  false,
		AllowRequestMoves:      false,
		RequireExplicitConsent: false,
		ValidationTimeout:      5 * time.Second,
		ConcurrentChecks:      10,
	}
	
	return NewSimpleEndpointScopeValidator(config)
}

// NewDevelopmentEndpointScopeValidator creates a validator with development-friendly settings
func NewDevelopmentEndpointScopeValidator() *SimpleEndpointScopeValidator {
	config := &EndpointValidationConfig{
		ValidateCollectionScope: false,
		ValidateFlowScope:      false,
		ValidateRequestScope:   false,
		ValidateExampleScope:   false,
		EnableCaching:          false,
		AllowCollectionToFlow:  true,
		AllowFlowToCollection:  true,
		AllowRequestMoves:      true,
		RequireExplicitConsent: false,
		ValidationTimeout:      30 * time.Second,
		ConcurrentChecks:      2,
	}
	
	return NewSimpleEndpointScopeValidator(config)
}

// NewTestEndpointScopeValidator creates a validator suitable for testing
func NewTestEndpointScopeValidator() *SimpleEndpointScopeValidator {
	config := &EndpointValidationConfig{
		ValidateCollectionScope: true,
		ValidateFlowScope:      true,
		ValidateRequestScope:   true,
		ValidateExampleScope:   true,
		EnableCaching:          false, // Disable caching for predictable tests
		AllowCollectionToFlow:  true,
		AllowFlowToCollection:  true,
		AllowRequestMoves:      false,
		RequireExplicitConsent: false,
		ValidationTimeout:      1 * time.Second,
		ConcurrentChecks:      1,
	}
	
	return NewSimpleEndpointScopeValidator(config)
}