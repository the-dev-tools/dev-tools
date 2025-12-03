package rimportv2

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mvar"

	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/translate/harv2"
)

// DefaultImporter implements the Importer interface using existing modern services
// It coordinates HAR processing and storage operations
type DefaultImporter struct {
	db                        *sql.DB
	httpService               *shttp.HTTPService
	flowService               *sflow.FlowService
	fileService               *sfile.FileService
	httpHeaderService         shttp.HttpHeaderService
	httpSearchParamService    *shttp.HttpSearchParamService
	httpBodyFormService       *shttp.HttpBodyFormService
	httpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService
	bodyService               *shttp.HttpBodyRawService
	httpAssertService         *shttp.HttpAssertService
	nodeService               *snode.NodeService
	nodeRequestService        *snoderequest.NodeRequestService
	nodeNoopService           *snodenoop.NodeNoopService
	edgeService               *sedge.EdgeService
	envService                senv.EnvironmentService
	varService                svar.VarService
	harTranslator             *defaultHARTranslator
}

// NewImporter creates a new DefaultImporter with service dependencies
func NewImporter(
	db *sql.DB,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	httpHeaderService shttp.HttpHeaderService,
	httpSearchParamService *shttp.HttpSearchParamService,
	httpBodyFormService *shttp.HttpBodyFormService,
	httpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService,
	bodyService *shttp.HttpBodyRawService,
	httpAssertService *shttp.HttpAssertService,
	nodeService *snode.NodeService,
	nodeRequestService *snoderequest.NodeRequestService,
	nodeNoopService *snodenoop.NodeNoopService,
	edgeService *sedge.EdgeService,
	envService senv.EnvironmentService,
	varService svar.VarService,
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
		httpAssertService:         httpAssertService,
		nodeService:               nodeService,
		nodeRequestService:        nodeRequestService,
		nodeNoopService:           nodeNoopService,
		edgeService:               edgeService,
		envService:                envService,
		varService:                varService,
		harTranslator:             newHARTranslator(),
	}
}

// StoreDomainVariables adds domain-to-variable mappings to all existing environments
// in the workspace. The domain URL is stored as the variable value so users can
// easily change the base URL by modifying the environment variable.
func (imp *DefaultImporter) StoreDomainVariables(ctx context.Context, workspaceID idwrap.IDWrap, domainData []ImportDomainData) ([]mvar.Var, error) {
	if len(domainData) == 0 {
		return nil, nil
	}

	// Filter to only enabled domain data WITH variable names
	// Skip entries where variable is empty - user didn't want to create an env var for it
	var enabledDomains []ImportDomainData
	for _, dd := range domainData {
		if dd.Enabled && dd.Variable != "" {
			enabledDomains = append(enabledDomains, dd)
		}
	}

	if len(enabledDomains) == 0 {
		return nil, nil
	}

	// Get all environments in the workspace (before transaction)
	environments, err := imp.envService.ListEnvironments(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	if len(environments) == 0 {
		// No environments exist, nothing to add variables to
		return nil, nil
	}

	// Start transaction
	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txVarService := imp.varService.TX(tx)

	// Add variables to each environment
	var allVariables []mvar.Var
	for _, env := range environments {
		// Get existing variables for this environment to check for duplicates
		existingVars, err := txVarService.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing variables for environment %s: %w", env.Name, err)
		}

		// Build a map of existing variable keys to their IDs for quick lookup
		existingKeyToVar := make(map[string]mvar.Var)
		for _, v := range existingVars {
			existingKeyToVar[v.VarKey] = v
		}

		for i, dd := range enabledDomains {
			// Build the full URL value (include https:// prefix for the domain)
			urlValue := "https://" + dd.Domain

			// Check if variable already exists
			if existingVar, exists := existingKeyToVar[dd.Variable]; exists {
				// Update existing variable
				existingVar.Value = urlValue
				existingVar.Description = fmt.Sprintf("Base URL for %s", dd.Domain)
				if err := txVarService.Update(ctx, &existingVar); err != nil {
					return nil, fmt.Errorf("failed to update variable %s for environment %s: %w", dd.Variable, env.Name, err)
				}
				allVariables = append(allVariables, existingVar)
			} else {
				// Create new variable
				variable := mvar.Var{
					ID:          idwrap.NewNow(),
					EnvID:       env.ID,
					VarKey:      dd.Variable,
					Value:       urlValue,
					Enabled:     true,
					Description: fmt.Sprintf("Base URL for %s", dd.Domain),
					Order:       float64(i + 1),
				}

				if err := txVarService.Create(ctx, variable); err != nil {
					return nil, fmt.Errorf("failed to create variable %s for environment %s: %w", dd.Variable, env.Name, err)
				}
				allVariables = append(allVariables, variable)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return allVariables, nil
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
			header := &mhttp.HTTPHeader{
				ID:                 h.ID,
				HttpID:             h.HttpID,
				Key:                h.Key,
				Value:              h.Value,
				Enabled:            h.Enabled,
				Description:        h.Description,
				ParentHttpHeaderID: h.ParentHttpHeaderID,
				// Ensure constraint: is_delta = FALSE OR parent_header_id IS NOT NULL
				IsDelta:          h.IsDelta && h.ParentHttpHeaderID != nil,
				DeltaKey:         h.DeltaKey,
				DeltaValue:       h.DeltaValue,
				DeltaEnabled:     h.DeltaEnabled,
				DeltaDescription: h.DeltaDescription,
				CreatedAt:        h.CreatedAt,
				UpdatedAt:        h.UpdatedAt,
			}
			if err := imp.httpHeaderService.Create(ctx, header); err != nil {
				return fmt.Errorf("failed to store header: %w", err)
			}
		}
	}

	if len(results.HTTPSearchParams) > 0 {
		for _, p := range results.HTTPSearchParams {
			param := &mhttp.HTTPSearchParam{
				ID:                      p.ID,
				HttpID:                  p.HttpID,
				Key:                     p.Key,
				Value:                   p.Value,
				Enabled:                 p.Enabled,
				Description:             p.Description,
				ParentHttpSearchParamID: p.ParentHttpSearchParamID,
				// Ensure constraint: is_delta = FALSE OR parent_id IS NOT NULL
				IsDelta:          p.IsDelta && p.ParentHttpSearchParamID != nil,
				DeltaKey:         p.DeltaKey,
				DeltaValue:       p.DeltaValue,
				DeltaEnabled:     p.DeltaEnabled,
				DeltaDescription: p.DeltaDescription,
				CreatedAt:        p.CreatedAt,
				UpdatedAt:        p.UpdatedAt,
			}
			if err := imp.httpSearchParamService.Create(ctx, param); err != nil {
				return fmt.Errorf("failed to store search param: %w", err)
			}
		}
	}

	if len(results.HTTPBodyForms) > 0 {
		for _, f := range results.HTTPBodyForms {
			form := &mhttp.HTTPBodyForm{
				ID:                   f.ID,
				HttpID:               f.HttpID,
				Key:                  f.Key,
				Value:                f.Value,
				Enabled:              f.Enabled,
				Description:          f.Description,
				ParentHttpBodyFormID: f.ParentHttpBodyFormID,
				// Ensure constraint: is_delta = FALSE OR parent_id IS NOT NULL
				IsDelta:          f.IsDelta && f.ParentHttpBodyFormID != nil,
				DeltaKey:         f.DeltaKey,
				DeltaValue:       f.DeltaValue,
				DeltaEnabled:     f.DeltaEnabled,
				DeltaDescription: f.DeltaDescription,
				CreatedAt:        f.CreatedAt,
				UpdatedAt:        f.UpdatedAt,
			}
			if err := imp.httpBodyFormService.Create(ctx, form); err != nil {
				return fmt.Errorf("failed to store body form: %w", err)
			}
		}
	}

	if len(results.HTTPBodyUrlEncoded) > 0 {
		for _, u := range results.HTTPBodyUrlEncoded {
			urlencoded := &mhttp.HTTPBodyUrlencoded{
				ID:                         u.ID,
				HttpID:                     u.HttpID,
				Key:                        u.Key,
				Value:                      u.Value,
				Enabled:                    u.Enabled,
				Description:                u.Description,
				ParentHttpBodyUrlEncodedID: u.ParentHttpBodyUrlEncodedID,
				// Ensure constraint: is_delta = FALSE OR parent_id IS NOT NULL
				IsDelta:          u.IsDelta && u.ParentHttpBodyUrlEncodedID != nil,
				DeltaKey:         u.DeltaKey,
				DeltaValue:       u.DeltaValue,
				DeltaEnabled:     u.DeltaEnabled,
				DeltaDescription: u.DeltaDescription,
				CreatedAt:        u.CreatedAt,
				UpdatedAt:        u.UpdatedAt,
			}
			if err := imp.httpBodyUrlEncodedService.Create(ctx, urlencoded); err != nil {
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
	registry := NewTranslatorRegistry(imp.httpService)
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
	txNodeService := imp.nodeService.TX(tx)
	txNodeRequestService := imp.nodeRequestService.TX(tx)
	txNodeNoopService := imp.nodeNoopService.TX(tx)
	txEdgeService := imp.edgeService.TX(tx)

	// Store files first (they may be referenced by HTTP entities)
	if len(results.Files) > 0 {
		// Group files by folder to safely calculate orders
		filesByFolder := make(map[string][]*mfile.File)
		for i := range results.Files {
			file := &results.Files[i]
			key := "nil"
			if file.ParentID != nil {
				key = file.ParentID.String()
			}
			filesByFolder[key] = append(filesByFolder[key], file)
		}

		for _, files := range filesByFolder {
			if len(files) == 0 {
				continue
			}

			folderID := files[0].ParentID

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

	// Store nodes
	if len(results.Nodes) > 0 {
		for _, node := range results.Nodes {
			if err := txNodeService.CreateNode(ctx, node); err != nil {
				return fmt.Errorf("failed to store node: %w", err)
			}
		}
	}

	// Store request nodes
	if len(results.RequestNodes) > 0 {
		for _, reqNode := range results.RequestNodes {
			if err := txNodeRequestService.CreateNodeRequest(ctx, reqNode); err != nil {
				return fmt.Errorf("failed to store request node: %w", err)
			}
		}
	}

	// Store no-op nodes
	if len(results.NoOpNodes) > 0 {
		for _, noopNode := range results.NoOpNodes {
			if err := txNodeNoopService.CreateNodeNoop(ctx, noopNode); err != nil {
				return fmt.Errorf("failed to store no-op node: %w", err)
			}
		}
	}

	// Store edges
	if len(results.Edges) > 0 {
		for _, edge := range results.Edges {
			if err := txEdgeService.CreateEdge(ctx, edge); err != nil {
				return fmt.Errorf("failed to store edge: %w", err)
			}
		}
	}

	// Store child entities
	if len(results.Headers) > 0 {
		for _, h := range results.Headers {
			header := mhttp.HTTPHeader{
				ID:                 h.ID,
				HttpID:             h.HttpID,
				Key:                h.Key,
				Value:              h.Value,
				Enabled:            h.Enabled,
				Description:        h.Description,
				ParentHttpHeaderID: h.ParentHttpHeaderID,
				// Ensure constraint: is_delta = FALSE OR parent_header_id IS NOT NULL
				IsDelta:          h.IsDelta && h.ParentHttpHeaderID != nil,
				DeltaKey:         h.DeltaKey,
				DeltaValue:       h.DeltaValue,
				DeltaEnabled:     h.DeltaEnabled,
				DeltaDescription: h.DeltaDescription,
				CreatedAt:        h.CreatedAt,
				UpdatedAt:        h.UpdatedAt,
			}
			if err := txHeaderService.Create(ctx, &header); err != nil {
				return fmt.Errorf("failed to store header: %w", err)
			}
		}
	}

	if len(results.SearchParams) > 0 {
		for _, p := range results.SearchParams {
			param := mhttp.HTTPSearchParam{
				ID:                      p.ID,
				HttpID:                  p.HttpID,
				Key:                     p.Key,
				Value:                   p.Value,
				Enabled:                 p.Enabled,
				Description:             p.Description,
				ParentHttpSearchParamID: p.ParentHttpSearchParamID,
				// Ensure constraint: is_delta = FALSE OR parent_id IS NOT NULL
				IsDelta:          p.IsDelta && p.ParentHttpSearchParamID != nil,
				DeltaKey:         p.DeltaKey,
				DeltaValue:       p.DeltaValue,
				DeltaEnabled:     p.DeltaEnabled,
				DeltaDescription: p.DeltaDescription,
				CreatedAt:        p.CreatedAt,
				UpdatedAt:        p.UpdatedAt,
			}
			if err := txSearchParamService.Create(ctx, &param); err != nil {
				return fmt.Errorf("failed to store search param: %w", err)
			}
		}
	}

	if len(results.BodyForms) > 0 {
		for _, f := range results.BodyForms {
			form := mhttp.HTTPBodyForm{
				ID:                   f.ID,
				HttpID:               f.HttpID,
				Key:                  f.Key,
				Value:                f.Value,
				Enabled:              f.Enabled,
				Description:          f.Description,
				ParentHttpBodyFormID: f.ParentHttpBodyFormID,
				// Ensure constraint: is_delta = FALSE OR parent_id IS NOT NULL
				IsDelta:          f.IsDelta && f.ParentHttpBodyFormID != nil,
				DeltaKey:         f.DeltaKey,
				DeltaValue:       f.DeltaValue,
				DeltaEnabled:     f.DeltaEnabled,
				DeltaDescription: f.DeltaDescription,
				CreatedAt:        f.CreatedAt,
				UpdatedAt:        f.UpdatedAt,
			}
			if err := txBodyFormService.Create(ctx, &form); err != nil {
				return fmt.Errorf("failed to store body form: %w", err)
			}
		}
	}

	if len(results.BodyUrlencoded) > 0 {
		for _, u := range results.BodyUrlencoded {
			urlencoded := mhttp.HTTPBodyUrlencoded{
				ID:                         u.ID,
				HttpID:                     u.HttpID,
				Key:                        u.Key,
				Value:                      u.Value,
				Enabled:                    u.Enabled,
				Description:                u.Description,
				ParentHttpBodyUrlEncodedID: u.ParentHttpBodyUrlEncodedID,
				// Ensure constraint: is_delta = FALSE OR parent_id IS NOT NULL
				IsDelta:          u.IsDelta && u.ParentHttpBodyUrlEncodedID != nil,
				DeltaKey:         u.DeltaKey,
				DeltaValue:       u.DeltaValue,
				DeltaEnabled:     u.DeltaEnabled,
				DeltaDescription: u.DeltaDescription,
				CreatedAt:        u.CreatedAt,
				UpdatedAt:        u.UpdatedAt,
			}
			if err := txBodyUrlEncodedService.Create(ctx, &urlencoded); err != nil {
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

	// Store assertions
	if len(results.Asserts) > 0 {
		txAssertService := imp.httpAssertService.TX(tx)
		for _, a := range results.Asserts {
			assert := mhttp.HTTPAssert{
				ID:                 a.ID,
				HttpID:             a.HttpID,
				Value:              a.Value,
				Enabled:            a.Enabled,
				Description:        a.Description,
				Order:              a.Order,
				ParentHttpAssertID: a.ParentHttpAssertID,
				// Ensure constraint: is_delta = FALSE OR parent_id IS NOT NULL
				IsDelta:          a.IsDelta && a.ParentHttpAssertID != nil,
				DeltaValue:       a.DeltaValue,
				DeltaEnabled:     a.DeltaEnabled,
				DeltaDescription: a.DeltaDescription,
				DeltaOrder:       a.DeltaOrder,
				CreatedAt:        a.CreatedAt,
				UpdatedAt:        a.UpdatedAt,
			}
			if err := txAssertService.Create(ctx, &assert); err != nil {
				return fmt.Errorf("failed to store assertion: %w", err)
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
