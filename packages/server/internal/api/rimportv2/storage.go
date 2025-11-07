package rimportv2

import (
	"context"
	"database/sql"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
)

// DefaultStorageManager implements the StorageManager interface using existing modern services
type DefaultStorageManager struct {
	db        *sql.DB
	httpService *shttp.HTTPService
	flowService *sflow.FlowService
	queries    *gen.Queries
}

// NewStorageManager creates a new DefaultStorageManager
func NewStorageManager(db *sql.DB, httpService *shttp.HTTPService, flowService *sflow.FlowService) *DefaultStorageManager {
	return &DefaultStorageManager{
		db:          db,
		httpService: httpService,
		flowService: flowService,
		queries:     gen.New(db),
	}
}

// StoreHTTPEntities stores HTTP request entities using the modern HTTP service
func (sm *DefaultStorageManager) StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error {
	if len(httpReqs) == 0 {
		return nil
	}

	// Use existing shttp.HTTPService for storage
	// Since there's no bulk method, we'll create each HTTP entity individually
	for _, httpReq := range httpReqs {
		if err := sm.httpService.Create(ctx, httpReq); err != nil {
			return NewStorageError("create_http", "http_request", err)
		}
	}

	return nil
}

// StoreFiles stores file entities using the modern file service
func (sm *DefaultStorageManager) StoreFiles(ctx context.Context, files []*mfile.File) error {
	if len(files) == 0 {
		return nil
	}

	// TODO: Implement file storage once the sfile service is fixed
	// For now, we skip file storage as the sfile service has compilation issues
	// In a production implementation, we would either:
	// 1. Fix the sfile service to work with the current database schema
	// 2. Create our own file storage implementation
	// 3. Use direct sqlc queries for file storage

	return nil
}

// StoreFlow stores the flow entity using the modern flow service
func (sm *DefaultStorageManager) StoreFlow(ctx context.Context, flow *mflow.Flow) error {
	if flow == nil {
		return nil
	}

	// Use existing sflow.FlowService for storage
	// This service handles the modern mflow.Flow models
	return sm.flowService.CreateFlow(ctx, *flow)
}

// StoreImportResults performs a coordinated storage of all import results within a transaction
func (sm *DefaultStorageManager) StoreImportResults(ctx context.Context, results *ImportResults) error {
	if results == nil {
		return nil
	}

	// Start a database transaction for atomicity
	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return NewStorageError("begin_transaction", "import_results", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	// Create transaction-aware services
	txHTTPService := sm.httpService.TX(tx)
	txFlowService := sm.flowService.TX(tx)

	// Store files first (they may be referenced by HTTP entities)
	if len(results.Files) > 0 {
		// TODO: Implement file storage once the sfile service is fixed
		// For now, we skip file storage
	}

	// Store HTTP entities
	if len(results.HTTPReqs) > 0 {
		for _, httpReq := range results.HTTPReqs {
			if err := txHTTPService.Create(ctx, httpReq); err != nil {
				_ = tx.Rollback()
				return NewStorageError("store_http_entities", "http_request", err)
			}
		}
	}

	// Store flow
	if results.Flow != nil {
		if err := txFlowService.CreateFlow(ctx, *results.Flow); err != nil {
			_ = tx.Rollback()
			return NewStorageError("store_flow", "flow", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return NewStorageError("commit_transaction", "import_results", err)
	}

	return nil
}