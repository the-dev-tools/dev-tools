package rimportv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"the-dev-tools/db"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mhttpbodyform"
	"the-dev-tools/server/pkg/model/mhttpbodyurlencoded"
	"the-dev-tools/server/pkg/model/mhttpheader"
	"the-dev-tools/server/pkg/model/mhttpsearchparam"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/shttpbodyform"
	"the-dev-tools/server/pkg/service/shttpbodyurlencoded"
	"the-dev-tools/server/pkg/service/shttpheader"
	"the-dev-tools/server/pkg/service/shttpsearchparam"
	"the-dev-tools/server/pkg/translate/harv2"
)

// DefaultImporter implements the Importer interface using existing modern services
// It coordinates HAR processing and storage operations
type DefaultImporter struct {
	db                        *sql.DB
	httpService               *shttp.HTTPService
	flowService               *sflow.FlowService
	fileService               *sfile.FileService
	httpHeaderService         shttpheader.HttpHeaderService
	httpSearchParamService    shttpsearchparam.HttpSearchParamService
	httpBodyFormService       shttpbodyform.HttpBodyFormService
	httpBodyUrlEncodedService shttpbodyurlencoded.HttpBodyUrlEncodedService
	bodyService               *shttp.HttpBodyRawService
	harTranslator             *defaultHARTranslator
}

// NewImporter creates a new DefaultImporter with service dependencies
func NewImporter(
	db *sql.DB,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	httpHeaderService shttpheader.HttpHeaderService,
	httpSearchParamService shttpsearchparam.HttpSearchParamService,
	httpBodyFormService shttpbodyform.HttpBodyFormService,
	httpBodyUrlEncodedService shttpbodyurlencoded.HttpBodyUrlEncodedService,
	bodyService *shttp.HttpBodyRawService,
) *DefaultImporter {
	return &DefaultImporter{
		db:                        db,
		httpService:               httpService,
		flowService:               flowService,
		fileService:               fileService,
		httpHeaderService:         httpHeaderService,
		httpSearchParamService:    httpSearchParamService,
		httpBodyFormService:       httpBodyFormService,
		httpBodyUrlEncodedService: httpBodyUrlEncodedService,
		bodyService:               bodyService,
		harTranslator:             newHARTranslator(),
	}
}

// ImportAndStore processes HAR data and returns resolved models
func (imp *DefaultImporter) ImportAndStore(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	return imp.harTranslator.convertHAR(ctx, data, workspaceID)
}

// StoreHTTPEntities stores HTTP request entities using the modern HTTP service
func (imp *DefaultImporter) StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error {
	if len(httpReqs) == 0 {
		return nil
	}

	for _, httpReq := range httpReqs {
		if err := imp.httpService.Create(ctx, httpReq); err != nil {
			return fmt.Errorf("failed to store HTTP request: %w", err)
		}
	}

	return nil
}

// StoreFiles stores file entities using the modern file service
func (imp *DefaultImporter) StoreFiles(ctx context.Context, files []*mfile.File) error {
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		if err := imp.fileService.CreateFile(ctx, file); err != nil {
			return fmt.Errorf("failed to store file: %w", err)
		}
	}

	return nil
}

// StoreFlow stores the flow entity using the modern flow service
func (imp *DefaultImporter) StoreFlow(ctx context.Context, flow *mflow.Flow) error {
	if flow == nil {
		return nil
	}

	return imp.flowService.CreateFlow(ctx, *flow)
}

// StoreImportResults performs a coordinated storage of all import results
func (imp *DefaultImporter) StoreImportResults(ctx context.Context, results *ImportResults) error {
	if results == nil {
		return nil
	}

	// Store files first (they may be referenced by HTTP entities)
	if len(results.Files) > 0 {
		for _, file := range results.Files {
			if err := imp.fileService.CreateFile(ctx, file); err != nil {
				return fmt.Errorf("failed to store files: %w", err)
			}
		}
	}

	// Store HTTP entities
	if len(results.HTTPReqs) > 0 {
		for _, httpReq := range results.HTTPReqs {
			if err := imp.httpService.Create(ctx, httpReq); err != nil {
				return fmt.Errorf("failed to store HTTP entities: %w", err)
			}
		}
	}

	// Store child entities
	if len(results.HTTPHeaders) > 0 {
		for _, h := range results.HTTPHeaders {
			header := &mhttpheader.HttpHeader{
				ID:          h.ID,
				HttpID:      h.HttpID,
				Key:         h.HeaderKey,
				Value:       h.HeaderValue,
				Enabled:     h.Enabled,
				Description: h.Description,
				CreatedAt:   h.CreatedAt,
				UpdatedAt:   h.UpdatedAt,
			}
			if err := imp.httpHeaderService.Create(ctx, header); err != nil {
				return fmt.Errorf("failed to store header: %w", err)
			}
		}
	}

	if len(results.HTTPSearchParams) > 0 {
		for _, p := range results.HTTPSearchParams {
			param := &mhttpsearchparam.HttpSearchParam{
				ID:          p.ID,
				HttpID:      p.HttpID,
				Key:         p.ParamKey,
				Value:       p.ParamValue,
				Enabled:     p.Enabled,
				Description: p.Description,
				CreatedAt:   p.CreatedAt,
				UpdatedAt:   p.UpdatedAt,
			}
			if err := imp.httpSearchParamService.Create(ctx, param); err != nil {
				return fmt.Errorf("failed to store search param: %w", err)
			}
		}
	}

	if len(results.HTTPBodyForms) > 0 {
		for _, f := range results.HTTPBodyForms {
			form := &mhttpbodyform.HttpBodyForm{
				ID:          f.ID,
				HttpID:      f.HttpID,
				Key:         f.FormKey,
				Value:       f.FormValue,
				Enabled:     f.Enabled,
				Description: f.Description,
				CreatedAt:   f.CreatedAt,
				UpdatedAt:   f.UpdatedAt,
			}
			if err := imp.httpBodyFormService.CreateHttpBodyForm(ctx, form); err != nil {
				return fmt.Errorf("failed to store body form: %w", err)
			}
		}
	}

	if len(results.HTTPBodyUrlEncoded) > 0 {
		for _, u := range results.HTTPBodyUrlEncoded {
			urlencoded := &mhttpbodyurlencoded.HttpBodyUrlEncoded{
				ID:          u.ID,
				HttpID:      u.HttpID,
				Key:         u.UrlencodedKey,
				Value:       u.UrlencodedValue,
				Enabled:     u.Enabled,
				Description: u.Description,
				CreatedAt:   u.CreatedAt,
				UpdatedAt:   u.UpdatedAt,
			}
			if err := imp.httpBodyUrlEncodedService.CreateHttpBodyUrlEncoded(ctx, urlencoded); err != nil {
				return fmt.Errorf("failed to store body urlencoded: %w", err)
			}
		}
	}

	if len(results.HTTPBodyRaws) > 0 {
		for _, r := range results.HTTPBodyRaws {
			// Note: bodyService.Create generates a new ID
			if _, err := imp.bodyService.Create(ctx, r.HttpID, r.RawData, r.ContentType); err != nil {
				return fmt.Errorf("failed to store body raw: %w", err)
			}
		}
	}

	// Store flow
	if results.Flow != nil {
		if err := imp.flowService.CreateFlow(ctx, *results.Flow); err != nil {
			return fmt.Errorf("failed to store flow: %w", err)
		}
	}

	return nil
}

// ImportAndStoreUnified processes any supported format and returns unified translation results
func (imp *DefaultImporter) ImportAndStoreUnified(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	registry := NewTranslatorRegistry()
	return registry.DetectAndTranslate(ctx, data, workspaceID)
}

// StoreFlows stores multiple flow entities using the modern flow service
func (imp *DefaultImporter) StoreFlows(ctx context.Context, flows []*mflow.Flow) error {
	if len(flows) == 0 {
		return nil
	}

	for _, flow := range flows {
		if err := imp.flowService.CreateFlow(ctx, *flow); err != nil {
			return fmt.Errorf("failed to store flow: %w", err)
		}
	}

	return nil
}

// StoreUnifiedResults performs a coordinated storage of all unified translation results
func (imp *DefaultImporter) StoreUnifiedResults(ctx context.Context, results *TranslationResult) error {
	if results == nil {
		return nil
	}

	// Start transaction
	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Create transactional services
	txFileService := imp.fileService.TX(tx)
	txHttpService := imp.httpService.TX(tx)
	txFlowService := imp.flowService.TX(tx)
	txHeaderService := imp.httpHeaderService.TX(tx)
	txSearchParamService := imp.httpSearchParamService.TX(tx)
	txBodyFormService := imp.httpBodyFormService.TX(tx)
	txBodyUrlEncodedService := imp.httpBodyUrlEncodedService.TX(tx)
	txBodyRawService := imp.bodyService.TX(tx)

	// Store files first (they may be referenced by HTTP entities)
	if len(results.Files) > 0 {
		// Group files by folder to safely calculate orders
		filesByFolder := make(map[string][]*mfile.File)
		for i := range results.Files {
			file := &results.Files[i]
			key := "nil"
			if file.FolderID != nil {
				key = file.FolderID.String()
			}
			filesByFolder[key] = append(filesByFolder[key], file)
		}

		for _, files := range filesByFolder {
			if len(files) == 0 {
				continue
			}
			
			folderID := files[0].FolderID
			
			// Get starting order once for this folder using the transactional service
			startOrder, err := txFileService.NextDisplayOrder(ctx, results.WorkspaceID, folderID)
			if err != nil {
				return fmt.Errorf("failed to get display order: %w", err)
			}

			for i, file := range files {
				file.Order = startOrder + float64(i)
				if err := txFileService.CreateFile(ctx, file); err != nil {
					return fmt.Errorf("failed to store file: %w", err)
				}
			}
		}
	}

	// Store HTTP entities
	if len(results.HTTPRequests) > 0 {
		for _, httpReq := range results.HTTPRequests {
			if err := txHttpService.Create(ctx, &httpReq); err != nil {
				return fmt.Errorf("failed to store HTTP entity: %w", err)
			}
		}
	}

	// Store flows
	if len(results.Flows) > 0 {
		for _, flow := range results.Flows {
			if err := txFlowService.CreateFlow(ctx, flow); err != nil {
				return fmt.Errorf("failed to store flow: %w", err)
			}
		}
	}

	// Store child entities
	if len(results.Headers) > 0 {
		for _, h := range results.Headers {
			header := mhttpheader.HttpHeader{
				ID:          h.ID,
				HttpID:      h.HttpID,
				Key:         h.HeaderKey,
				Value:       h.HeaderValue,
				Enabled:     h.Enabled,
				Description: h.Description,
				CreatedAt:   h.CreatedAt,
				UpdatedAt:   h.UpdatedAt,
			}
			if err := txHeaderService.Create(ctx, &header); err != nil {
				return fmt.Errorf("failed to store header: %w", err)
			}
		}
	}

	if len(results.SearchParams) > 0 {
		for _, p := range results.SearchParams {
			param := mhttpsearchparam.HttpSearchParam{
				ID:          p.ID,
				HttpID:      p.HttpID,
				Key:         p.ParamKey,
				Value:       p.ParamValue,
				Enabled:     p.Enabled,
				Description: p.Description,
				CreatedAt:   p.CreatedAt,
				UpdatedAt:   p.UpdatedAt,
			}
			if err := txSearchParamService.Create(ctx, &param); err != nil {
				return fmt.Errorf("failed to store search param: %w", err)
			}
		}
	}

	if len(results.BodyForms) > 0 {
		for _, f := range results.BodyForms {
			form := mhttpbodyform.HttpBodyForm{
				ID:          f.ID,
				HttpID:      f.HttpID,
				Key:         f.FormKey,
				Value:       f.FormValue,
				Enabled:     f.Enabled,
				Description: f.Description,
				CreatedAt:   f.CreatedAt,
				UpdatedAt:   f.UpdatedAt,
			}
			if err := txBodyFormService.CreateHttpBodyForm(ctx, &form); err != nil {
				return fmt.Errorf("failed to store body form: %w", err)
			}
		}
	}

	if len(results.BodyUrlencoded) > 0 {
		for _, u := range results.BodyUrlencoded {
			urlencoded := mhttpbodyurlencoded.HttpBodyUrlEncoded{
				ID:          u.ID,
				HttpID:      u.HttpID,
				Key:         u.UrlencodedKey,
				Value:       u.UrlencodedValue,
				Enabled:     u.Enabled,
				Description: u.Description,
				CreatedAt:   u.CreatedAt,
				UpdatedAt:   u.UpdatedAt,
			}
			if err := txBodyUrlEncodedService.CreateHttpBodyUrlEncoded(ctx, &urlencoded); err != nil {
				return fmt.Errorf("failed to store body urlencoded: %w", err)
			}
		}
	}

	if len(results.BodyRaw) > 0 {
		for _, r := range results.BodyRaw {
			if _, err := txBodyRawService.Create(ctx, r.HttpID, r.RawData, r.ContentType); err != nil {
				return fmt.Errorf("failed to store body raw: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Private HAR translator methods moved from translator.go

// newHARTranslator creates a new HAR translator (private method)
func newHARTranslator() *defaultHARTranslator {
	return &defaultHARTranslator{}
}

// defaultHARTranslator handles HAR file processing using the existing harv2 package (private struct)
type defaultHARTranslator struct{}

// convertHAR converts HAR data to modern models using the harv2 package (private method)
func (t *defaultHARTranslator) convertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	// Validate basic HAR structure before parsing
	if err := t.validateHARStructure(data); err != nil {
		return nil, err
	}

	// Parse HAR data from bytes
	har, err := harv2.ConvertRaw(data)
	if err != nil {
		return nil, fmt.Errorf("HAR conversion failed: %w", err)
	}

	// Use the existing harv2 package which already implements modern HAR translation
	// harv2.ConvertHAR returns HarResolved with modern mhttp.HTTP and mfile.File models
	resolved, err := harv2.ConvertHAR(har, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("HAR processing failed: %w", err)
	}

	return resolved, nil
}

// validateHARStructure validates basic HAR structure (private method)
func (t *defaultHARTranslator) validateHARStructure(data []byte) error {
	var har map[string]interface{}
	if err := json.Unmarshal(data, &har); err != nil {
		return ErrInvalidHARFormat
	}

	// Basic HAR structure validation
	log, ok := har["log"]
	if !ok {
		return ErrInvalidHARFormat
	}

	logMap, ok := log.(map[string]interface{})
	if !ok {
		return ErrInvalidHARFormat
	}

	if _, ok := logMap["entries"]; !ok {
		return ErrInvalidHARFormat
	}

	// Validate version field type - must be a string according to HAR spec
	if version, ok := logMap["version"]; ok {
		if _, ok := version.(string); !ok {
			return ErrInvalidHARFormat
		}
	}

	return nil
}

// NewHARTranslatorForTesting creates a new HAR translator for testing purposes
// This provides access to the HAR translator for test files while keeping the main implementation private
func NewHARTranslatorForTesting() *defaultHARTranslator {
	return newHARTranslator()
}

// ConvertHARForTesting exposes the ConvertHAR method for testing purposes
func (t *defaultHARTranslator) ConvertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	return t.convertHAR(ctx, data, workspaceID)
}
