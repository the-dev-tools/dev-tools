//nolint:revive // exported
package rimportv2

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/harv2"
)

// DefaultImporter implements the Importer interface using existing modern services
// It coordinates HAR processing and storage operations
type DefaultImporter struct {
	db                        *sql.DB
	workspaceService          sworkspace.WorkspaceService
	httpService               *shttp.HTTPService
	flowService               *sflow.FlowService
	fileService               *sfile.FileService
	httpHeaderService         shttp.HttpHeaderService
	httpSearchParamService    *shttp.HttpSearchParamService
	httpBodyFormService       *shttp.HttpBodyFormService
	httpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService
	bodyService               *shttp.HttpBodyRawService
	httpAssertService         *shttp.HttpAssertService
	nodeService               *sflow.NodeService
	nodeRequestService        *sflow.NodeRequestService
	edgeService               *sflow.EdgeService
	envService                senv.EnvironmentService
	varService                senv.VariableService
	harTranslator             *defaultHARTranslator
	dedup                     *Deduplicator
}

// NewImporter creates a new DefaultImporter with service dependencies
func NewImporter(
	db *sql.DB,
	workspaceService sworkspace.WorkspaceService,
	httpService *shttp.HTTPService,
	flowService *sflow.FlowService,
	fileService *sfile.FileService,
	httpHeaderService shttp.HttpHeaderService,
	httpSearchParamService *shttp.HttpSearchParamService,
	httpBodyFormService *shttp.HttpBodyFormService,
	httpBodyUrlEncodedService *shttp.HttpBodyUrlEncodedService,
	bodyService *shttp.HttpBodyRawService,
	httpAssertService *shttp.HttpAssertService,
	nodeService *sflow.NodeService,
	nodeRequestService *sflow.NodeRequestService,
	edgeService *sflow.EdgeService,
	envService senv.EnvironmentService,
	varService senv.VariableService,
) *DefaultImporter {
	return &DefaultImporter{
		db:                        db,
		workspaceService:          workspaceService,
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
		edgeService:               edgeService,
		envService:                envService,
		varService:                varService,
		harTranslator:             newHARTranslator(),
		dedup:                     NewDeduplicator(*httpService, *fileService, nil),
	}
}

// StoreDomainVariables adds domain-to-variable mappings to all existing environments
// in the workspace. The domain URL is stored as the variable value so users can
// easily change the base URL by modifying the environment variable.
func (imp *DefaultImporter) StoreDomainVariables(ctx context.Context, workspaceID idwrap.IDWrap, domainData []ImportDomainData) ([]menv.Env, []menv.Variable, []menv.Variable, error) {
	if len(domainData) == 0 {
		return nil, nil, nil, nil
	}

	var enabledDomains []ImportDomainData
	for _, dd := range domainData {
		if dd.Enabled && dd.Variable != "" {
			enabledDomains = append(enabledDomains, dd)
		}
	}

	if len(enabledDomains) == 0 {
		return nil, nil, nil, nil
	}

	environments, err := imp.envService.ListEnvironments(ctx, workspaceID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list environments: %w", err)
	}

	var createdEnvs []menv.Env
	if len(environments) == 0 {
		defaultEnv := menv.Env{
			ID:          idwrap.NewNow(),
			WorkspaceID: workspaceID,
			Name:        "Default Environment",
			Description: "Created automatically during import",
			Type:        menv.EnvNormal,
		}

		if err := imp.envService.CreateEnvironment(ctx, &defaultEnv); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create default environment: %w", err)
		}

		environments = append(environments, defaultEnv)
		createdEnvs = append(createdEnvs, defaultEnv)
	}

	existingVarsByEnv := make(map[string]map[string]menv.Variable)
	for _, env := range environments {
		vars, err := imp.varService.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get variables for environment %s: %w", env.Name, err)
		}
		varMap := make(map[string]menv.Variable)
		for _, v := range vars {
			varMap[v.VarKey] = v
		}
		existingVarsByEnv[env.ID.String()] = varMap
	}

	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txVarWriter := senv.NewVariableWriter(tx)
	var createdVars []menv.Variable
	var updatedVars []menv.Variable
	for _, env := range environments {
		existingVars := existingVarsByEnv[env.ID.String()]
		for i, dd := range enabledDomains {
			scheme := "https://"
			if strings.HasPrefix(dd.Domain, "localhost") || strings.HasPrefix(dd.Domain, "127.") || strings.HasPrefix(dd.Domain, "::1") {
				scheme = "http://"
			}

			urlValue := scheme + dd.Domain
			existingVar, exists := existingVars[dd.Variable]

			if exists {
				variable := menv.Variable{
					ID:          existingVar.ID,
					EnvID:       env.ID,
					VarKey:      dd.Variable,
					Value:       urlValue,
					Enabled:     true,
					Description: fmt.Sprintf("Base URL for %s", dd.Domain),
					Order:       existingVar.Order,
				}

				if err := txVarWriter.Update(ctx, &variable); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to update variable %s for environment %s: %w", dd.Variable, env.Name, err)
				}
				updatedVars = append(updatedVars, variable)
			} else {
				variable := menv.Variable{
					ID:          idwrap.NewNow(),
					EnvID:       env.ID,
					VarKey:      dd.Variable,
					Value:       urlValue,
					Enabled:     true,
					Description: fmt.Sprintf("Base URL for %s", dd.Domain),
					Order:       float64(i + 1),
				}

				if err := txVarWriter.Create(ctx, variable); err != nil {
					return nil, nil, nil, fmt.Errorf("failed to create variable %s for environment %s: %w", dd.Variable, env.Name, err)
				}
				createdVars = append(createdVars, variable)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return createdEnvs, createdVars, updatedVars, nil
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

	// For legacy StoreImportResults, we use the coordinated StoreUnifiedResults logic
	// by converting ImportResults back to a TranslationResult or just implementing
	// the storage directly here. Since StoreUnifiedResults is more robust,
	// let's ensure this one at least covers the basics for the tests.

	if len(results.Files) > 0 {
		for _, file := range results.Files {
			if err := imp.fileService.CreateFile(ctx, file); err != nil {
				return fmt.Errorf("failed to store files: %w", err)
			}
		}
	}

	if len(results.HTTPReqs) > 0 {
		for _, httpReq := range results.HTTPReqs {
			if err := imp.httpService.Create(ctx, httpReq); err != nil {
				return fmt.Errorf("failed to store HTTP entities: %w", err)
			}
		}
	}

	// Store child entities
	for _, h := range results.HTTPHeaders {
		if err := imp.httpHeaderService.Create(ctx, h); err != nil {
			return fmt.Errorf("failed to store header: %w", err)
		}
	}
	for _, p := range results.HTTPSearchParams {
		if err := imp.httpSearchParamService.Create(ctx, p); err != nil {
			return fmt.Errorf("failed to store search param: %w", err)
		}
	}
	for _, f := range results.HTTPBodyForms {
		if err := imp.httpBodyFormService.Create(ctx, f); err != nil {
			return fmt.Errorf("failed to store body form: %w", err)
		}
	}
	for _, u := range results.HTTPBodyUrlEncoded {
		if err := imp.httpBodyUrlEncodedService.Create(ctx, u); err != nil {
			return fmt.Errorf("failed to store body urlencoded: %w", err)
		}
	}
	for _, r := range results.HTTPBodyRaws {
		if _, err := imp.bodyService.CreateFull(ctx, r); err != nil {
			return fmt.Errorf("failed to store body raw: %w", err)
		}
	}
	for _, a := range results.HTTPAsserts {
		if err := imp.httpAssertService.Create(ctx, a); err != nil {
			return fmt.Errorf("failed to store assertion: %w", err)
		}
	}

	if results.Flow != nil {
		if err := imp.flowService.CreateFlow(ctx, *results.Flow); err != nil {
			return fmt.Errorf("failed to store flow: %w", err)
		}
	}

	// Store flow nodes
	for _, node := range results.Nodes {
		if err := imp.nodeService.CreateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to store node: %w", err)
		}
	}

	// Store request nodes
	for _, reqNode := range results.RequestNodes {
		if err := imp.nodeRequestService.CreateNodeRequest(ctx, reqNode); err != nil {
			return fmt.Errorf("failed to store request node: %w", err)
		}
	}

	// Store edges
	for _, edge := range results.Edges {
		if err := imp.edgeService.CreateEdge(ctx, edge); err != nil {
			return fmt.Errorf("failed to store edge: %w", err)
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
func (imp *DefaultImporter) StoreUnifiedResults(ctx context.Context, results *TranslationResult) (map[idwrap.IDWrap]bool, map[idwrap.IDWrap]bool, []menv.Variable, []menv.Variable, error) {
	if results == nil {
		return nil, nil, nil, nil, nil
	}

	// PHASE 1: Pre-Resolution (Read-only)
	// We perform all deduplication checks BEFORE starting the write transaction
	// to keep the transaction as short as possible.

	// 1.0 Pre-fetch workspace for GlobalEnv (needed for variable storage)
	// CRITICAL: This MUST be done BEFORE the transaction to avoid SQLite deadlocks
	var targetEnvID idwrap.IDWrap
	if len(results.Variables) > 0 {
		workspace, err := imp.workspaceService.Get(ctx, results.WorkspaceID)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to get workspace for variables: %w", err)
		}
		targetEnvID = workspace.GlobalEnv
		if targetEnvID.Compare(idwrap.IDWrap{}) == 0 {
			return nil, nil, nil, nil, fmt.Errorf("workspace has no global environment")
		}
	}

	httpIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	httpContentHashMap := make(map[idwrap.IDWrap]string)
	deduplicatedHttpIDs := make(map[idwrap.IDWrap]bool)

	// 1.1 Resolve HTTP Requests (Read-only lookup)
	if len(results.HTTPRequests) > 0 {
		for i := range results.HTTPRequests {
			req := &results.HTTPRequests[i]
			oldID := req.ID

			var parentContentHash string
			if req.ParentHttpID != nil {
				if _, ok := httpIDMap[*req.ParentHttpID]; ok {
					parentContentHash = httpContentHashMap[*req.ParentHttpID]
				}
			}

			var reqHeaders []mhttp.HTTPHeader
			for _, h := range results.Headers {
				if h.HttpID == oldID {
					reqHeaders = append(reqHeaders, h)
				}
			}
			var reqParams []mhttp.HTTPSearchParam
			for _, p := range results.SearchParams {
				if p.HttpID == oldID {
					reqParams = append(reqParams, p)
				}
			}
			var reqBodyRaw *mhttp.HTTPBodyRaw
			for _, r := range results.BodyRaw {
				if r.HttpID == oldID {
					reqBodyRaw = &r
					break
				}
			}
			var reqBodyForms []mhttp.HTTPBodyForm
			for _, f := range results.BodyForms {
				if f.HttpID == oldID {
					reqBodyForms = append(reqBodyForms, f)
				}
			}
			var reqBodyUrlEncoded []mhttp.HTTPBodyUrlencoded
			for _, u := range results.BodyUrlencoded {
				if u.HttpID == oldID {
					reqBodyUrlEncoded = append(reqBodyUrlEncoded, u)
				}
			}

			// Use FindHTTP instead of ResolveHTTP to avoid writes
			existingID, contentHash, err := imp.dedup.FindHTTP(ctx, req, reqHeaders, reqParams, reqBodyRaw, reqBodyForms, reqBodyUrlEncoded, parentContentHash)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to pre-resolve HTTP request %s: %w", req.Name, err)
			}

			if existingID.Compare(idwrap.IDWrap{}) != 0 {
				httpIDMap[oldID] = existingID
				deduplicatedHttpIDs[existingID] = true
			} else {
				httpIDMap[oldID] = oldID
			}
			httpContentHashMap[oldID] = contentHash
		}
	}

	// 1.2 Resolve Files (Read-only lookup)
	fileIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	deduplicatedFileIDs := make(map[idwrap.IDWrap]bool)

	if len(results.Files) > 0 {
		// Build full paths for all files recursively
		filesMap := make(map[idwrap.IDWrap]*mfile.File)
		for i := range results.Files {
			filesMap[results.Files[i].ID] = &results.Files[i]
		}

		oldIDToLogicalPath := make(map[idwrap.IDWrap]string)
		var buildPath func(id idwrap.IDWrap) string
		buildPath = func(id idwrap.IDWrap) string {
			if p, ok := oldIDToLogicalPath[id]; ok {
				return p
			}
			f := filesMap[id]
			if f == nil {
				return "imported"
			}
			if f.ParentID == nil {
				oldIDToLogicalPath[id] = f.Name
				return f.Name
			}
			p := buildPath(*f.ParentID) + "/" + f.Name
			oldIDToLogicalPath[id] = p
			return p
		}
		for i := range results.Files {
			buildPath(results.Files[i].ID)
		}

		// Sort by path depth to ensure parents are processed before children
		sort.SliceStable(results.Files, func(i, j int) bool {
			pathI := oldIDToLogicalPath[results.Files[i].ID]
			pathJ := oldIDToLogicalPath[results.Files[j].ID]
			depthI := strings.Count(pathI, "/")
			depthJ := strings.Count(pathJ, "/")
			if depthI != depthJ {
				return depthI < depthJ
			}
			if results.Files[i].ContentType != results.Files[j].ContentType {
				return results.Files[i].ContentType == mfile.ContentTypeFolder
			}
			return false
		})

		logicalPathToID := make(map[string]idwrap.IDWrap)
		for i := range results.Files {
			file := &results.Files[i]
			oldID := file.ID
			logicalPath := oldIDToLogicalPath[oldID]

			// Include content type in the key to avoid deduplicating different file types
			// with the same name (e.g., a flow file and a folder named "Test Flow")
			pathKey := fmt.Sprintf("%s:%d", logicalPath, file.ContentType)

			// First check if we've already seen this logical path + type in the SAME import
			if newID, ok := logicalPathToID[pathKey]; ok {
				fileIDMap[oldID] = newID
				deduplicatedFileIDs[newID] = true
				continue
			}

			// Use FindFile instead of ResolveFile
			existingID, pathHash, err := imp.dedup.FindFile(ctx, file, logicalPath)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to pre-resolve file %s: %w", file.Name, err)
			}

			// Fresh variable for pointer
			currentPathHash := pathHash
			file.PathHash = &currentPathHash

			if existingID.Compare(idwrap.IDWrap{}) != 0 {
				fileIDMap[oldID] = existingID
				deduplicatedFileIDs[existingID] = true
				imp.dedup.UpdatePathCache(pathHash, existingID)
				logicalPathToID[pathKey] = existingID
			} else {
				fileIDMap[oldID] = oldID
				imp.dedup.UpdatePathCache(pathHash, oldID)
				logicalPathToID[pathKey] = oldID
			}
		}
	}

	// 1.3 Build Header ID Mapping for Deduplicated HTTPs (Read-only)
	// CRITICAL: When an HTTP request is deduplicated, delta headers may reference
	// parent headers that belong to the deduplicated HTTP. We need to map these
	// parent header IDs to the existing headers in the database.
	headerIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	searchParamIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	bodyFormIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	bodyUrlencodedIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	assertIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)

	for oldHttpID, newHttpID := range httpIDMap {
		if !deduplicatedHttpIDs[newHttpID] {
			continue // Not deduplicated, child entities will be inserted fresh
		}

		// Find headers from this import batch that belong to this HTTP
		var importHeaders []mhttp.HTTPHeader
		for _, h := range results.Headers {
			if h.HttpID == oldHttpID {
				importHeaders = append(importHeaders, h)
			}
		}

		if len(importHeaders) > 0 {
			// Fetch existing headers from DB for the deduplicated HTTP
			existingHeaders, err := imp.httpHeaderService.GetByHttpID(ctx, newHttpID)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to fetch existing headers for deduplicated HTTP: %w", err)
			}

			// Build mapping by matching on Key
			existingByKey := make(map[string]idwrap.IDWrap)
			for _, eh := range existingHeaders {
				existingByKey[eh.Key] = eh.ID
			}

			for _, ih := range importHeaders {
				if existingID, ok := existingByKey[ih.Key]; ok {
					headerIDMap[ih.ID] = existingID
				}
			}
		}

		// Find search params from this import batch
		var importParams []mhttp.HTTPSearchParam
		for _, p := range results.SearchParams {
			if p.HttpID == oldHttpID {
				importParams = append(importParams, p)
			}
		}

		if len(importParams) > 0 {
			existingParams, err := imp.httpSearchParamService.GetByHttpID(ctx, newHttpID)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to fetch existing search params for deduplicated HTTP: %w", err)
			}

			existingByKey := make(map[string]idwrap.IDWrap)
			for _, ep := range existingParams {
				existingByKey[ep.Key] = ep.ID
			}

			for _, ip := range importParams {
				if existingID, ok := existingByKey[ip.Key]; ok {
					searchParamIDMap[ip.ID] = existingID
				}
			}
		}

		// Find body forms from this import batch
		var importForms []mhttp.HTTPBodyForm
		for _, f := range results.BodyForms {
			if f.HttpID == oldHttpID {
				importForms = append(importForms, f)
			}
		}

		if len(importForms) > 0 {
			existingForms, err := imp.httpBodyFormService.GetByHttpID(ctx, newHttpID)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to fetch existing body forms for deduplicated HTTP: %w", err)
			}

			existingByKey := make(map[string]idwrap.IDWrap)
			for _, ef := range existingForms {
				existingByKey[ef.Key] = ef.ID
			}

			for _, imf := range importForms {
				if existingID, ok := existingByKey[imf.Key]; ok {
					bodyFormIDMap[imf.ID] = existingID
				}
			}
		}

		// Find body urlencoded from this import batch
		var importUrlEncoded []mhttp.HTTPBodyUrlencoded
		for _, u := range results.BodyUrlencoded {
			if u.HttpID == oldHttpID {
				importUrlEncoded = append(importUrlEncoded, u)
			}
		}

		if len(importUrlEncoded) > 0 {
			existingUrlEncoded, err := imp.httpBodyUrlEncodedService.GetByHttpID(ctx, newHttpID)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to fetch existing body urlencoded for deduplicated HTTP: %w", err)
			}

			existingByKey := make(map[string]idwrap.IDWrap)
			for _, eu := range existingUrlEncoded {
				existingByKey[eu.Key] = eu.ID
			}

			for _, iu := range importUrlEncoded {
				if existingID, ok := existingByKey[iu.Key]; ok {
					bodyUrlencodedIDMap[iu.ID] = existingID
				}
			}
		}

		// Find asserts from this import batch
		var importAsserts []mhttp.HTTPAssert
		for _, a := range results.Asserts {
			if a.HttpID == oldHttpID {
				importAsserts = append(importAsserts, a)
			}
		}

		if len(importAsserts) > 0 {
			existingAsserts, err := imp.httpAssertService.GetByHttpID(ctx, newHttpID)
			if err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to fetch existing asserts for deduplicated HTTP: %w", err)
			}

			// For asserts, match by Value since that's the assertion expression
			existingByValue := make(map[string]idwrap.IDWrap)
			for _, ea := range existingAsserts {
				existingByValue[ea.Value] = ea.ID
			}

			for _, ia := range importAsserts {
				if existingID, ok := existingByValue[ia.Value]; ok {
					assertIDMap[ia.ID] = existingID
				}
			}
		}
	}

	// PHASE 2: Storage (Write)
	// Now we start the transaction and perform only necessary inserts

	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txFileService := imp.fileService.TX(tx)
	txHttpService := imp.httpService.TX(tx)
	txFlowService := imp.flowService.TX(tx)
	txHeaderWriter := shttp.NewHeaderWriter(tx)
	txSearchParamWriter := shttp.NewSearchParamWriter(tx)
	txBodyFormWriter := shttp.NewBodyFormWriter(tx)
	txBodyUrlEncodedWriter := shttp.NewBodyUrlEncodedWriter(tx)
	txBodyRawWriter := shttp.NewBodyRawWriter(tx)
	txNodeService := imp.nodeService.TX(tx)
	txNodeRequestService := imp.nodeRequestService.TX(tx)
	txEdgeService := imp.edgeService.TX(tx)
	txNodeJsWriter := sflow.NewNodeJsWriter(tx)
	txNodeIfWriter := sflow.NewNodeIfWriter(tx)
	txNodeForWriter := sflow.NewNodeForWriter(tx)
	txNodeForEachWriter := sflow.NewNodeForEachWriter(tx)
	txNodeAIWriter := sflow.NewNodeAIWriter(tx)
	txFlowVariableWriter := sflow.NewFlowVariableWriter(tx)

	// 2.1 Update IDs and Store HTTP Requests
	for i := range results.HTTPRequests {
		req := &results.HTTPRequests[i]
		oldID := req.ID
		newID := httpIDMap[oldID]
		hash := httpContentHashMap[oldID]
		req.ContentHash = &hash

		if req.ParentHttpID != nil {
			if mappedParent, ok := httpIDMap[*req.ParentHttpID]; ok {
				req.ParentHttpID = &mappedParent
			}
		}

		if !deduplicatedHttpIDs[newID] {
			if err := txHttpService.Create(ctx, req); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store HTTP request %s: %w", req.Name, err)
			}
		}
		req.ID = newID
	}

	// 2.2 Update IDs and Store Files
	for i := range results.Files {
		file := &results.Files[i]
		oldID := file.ID
		newID := fileIDMap[oldID]

		if file.ParentID != nil {
			if mappedParent, ok := fileIDMap[*file.ParentID]; ok {
				file.ParentID = &mappedParent
			}
		}
		if file.ContentID != nil {
			if mappedContent, ok := httpIDMap[*file.ContentID]; ok {
				file.ContentID = &mappedContent
			}
		}

		if !deduplicatedFileIDs[newID] {
			if err := txFileService.CreateFile(ctx, file); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store file %s: %w", file.Name, err)
			}
		}
		file.ID = newID
	}

	// 2.3 Update HTTP FolderIDs
	for i := range results.HTTPRequests {
		req := &results.HTTPRequests[i]
		if req.FolderID != nil {
			if newFolderID, ok := fileIDMap[*req.FolderID]; ok {
				req.FolderID = &newFolderID
			}
		}
	}

	// 2.4 Update IDs in Child Entities and Store
	for i := range results.Headers {
		if newID, ok := httpIDMap[results.Headers[i].HttpID]; ok {
			results.Headers[i].HttpID = newID
		}
		// Remap ParentHttpHeaderID if it points to a header that was deduplicated
		if results.Headers[i].ParentHttpHeaderID != nil {
			if newParentID, ok := headerIDMap[*results.Headers[i].ParentHttpHeaderID]; ok {
				results.Headers[i].ParentHttpHeaderID = &newParentID
			}
		}
	}
	for i := range results.SearchParams {
		if newID, ok := httpIDMap[results.SearchParams[i].HttpID]; ok {
			results.SearchParams[i].HttpID = newID
		}
		// Remap ParentHttpSearchParamID if it points to a param that was deduplicated
		if results.SearchParams[i].ParentHttpSearchParamID != nil {
			if newParentID, ok := searchParamIDMap[*results.SearchParams[i].ParentHttpSearchParamID]; ok {
				results.SearchParams[i].ParentHttpSearchParamID = &newParentID
			}
		}
	}
	for i := range results.BodyForms {
		if newID, ok := httpIDMap[results.BodyForms[i].HttpID]; ok {
			results.BodyForms[i].HttpID = newID
		}
		// Remap ParentHttpBodyFormID if it points to a form that was deduplicated
		if results.BodyForms[i].ParentHttpBodyFormID != nil {
			if newParentID, ok := bodyFormIDMap[*results.BodyForms[i].ParentHttpBodyFormID]; ok {
				results.BodyForms[i].ParentHttpBodyFormID = &newParentID
			}
		}
	}
	for i := range results.BodyUrlencoded {
		if newID, ok := httpIDMap[results.BodyUrlencoded[i].HttpID]; ok {
			results.BodyUrlencoded[i].HttpID = newID
		}
		// Remap ParentHttpBodyUrlEncodedID if it points to a urlencoded that was deduplicated
		if results.BodyUrlencoded[i].ParentHttpBodyUrlEncodedID != nil {
			if newParentID, ok := bodyUrlencodedIDMap[*results.BodyUrlencoded[i].ParentHttpBodyUrlEncodedID]; ok {
				results.BodyUrlencoded[i].ParentHttpBodyUrlEncodedID = &newParentID
			}
		}
	}
	for i := range results.BodyRaw {
		if newID, ok := httpIDMap[results.BodyRaw[i].HttpID]; ok {
			results.BodyRaw[i].HttpID = newID
		}
		// Note: BodyRaw doesn't need parent ID remapping here as it's unique per HTTP
	}
	for i := range results.Asserts {
		if newID, ok := httpIDMap[results.Asserts[i].HttpID]; ok {
			results.Asserts[i].HttpID = newID
		}
		// Remap ParentHttpAssertID if it points to an assert that was deduplicated
		if results.Asserts[i].ParentHttpAssertID != nil {
			if newParentID, ok := assertIDMap[*results.Asserts[i].ParentHttpAssertID]; ok {
				results.Asserts[i].ParentHttpAssertID = &newParentID
			}
		}
	}

	if err := storeUnifiedChildren(ctx, results, txHeaderWriter, txSearchParamWriter, txBodyFormWriter, txBodyUrlEncodedWriter, txBodyRawWriter, shttp.NewAssertWriter(tx), deduplicatedHttpIDs); err != nil {
		return nil, nil, nil, nil, err
	}

	// 2.5 Update Flow Entities
	for i := range results.RequestNodes {
		rn := &results.RequestNodes[i]
		if rn.HttpID != nil {
			if newID, ok := httpIDMap[*rn.HttpID]; ok {
				rn.HttpID = &newID
			}
		}
		if rn.DeltaHttpID != nil {
			if newID, ok := httpIDMap[*rn.DeltaHttpID]; ok {
				rn.DeltaHttpID = &newID
			}
		}
	}

	if len(results.Flows) > 0 {
		if err := txFlowService.CreateFlowBulk(ctx, results.Flows); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to store flows: %w", err)
		}

		// Create File entries for flows that don't already have entries in results.Files
		// (HAR imports already include flow files, YAML imports don't)
		existingFlowFiles := make(map[idwrap.IDWrap]bool)
		for i := range results.Files {
			if results.Files[i].ContentType == mfile.ContentTypeFlow {
				existingFlowFiles[results.Files[i].ID] = true
			}
		}

		// Create flow files for flows that don't have entries, append to Files
		for i, flow := range results.Flows {
			if existingFlowFiles[flow.ID] {
				continue // Already in results.Files
			}
			flowFile := mfile.File{
				ID:          flow.ID,
				WorkspaceID: flow.WorkspaceID,
				ContentType: mfile.ContentTypeFlow,
				Name:        flow.Name,
				Order:       float64(i + 1),
			}
			if err := txFileService.CreateFile(ctx, &flowFile); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store flow file entry %s: %w", flow.Name, err)
			}
			results.Files = append(results.Files, flowFile) // Append to unified Files array
		}
	}
	if len(results.Nodes) > 0 {
		if err := txNodeService.CreateNodeBulk(ctx, results.Nodes); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to store nodes: %w", err)
		}
	}
	if len(results.RequestNodes) > 0 {
		for _, reqNode := range results.RequestNodes {
			if err := txNodeRequestService.CreateNodeRequest(ctx, reqNode); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store request node: %w", err)
			}
		}
	}
	// Store JS nodes
	if len(results.JSNodes) > 0 {
		for _, jsNode := range results.JSNodes {
			if err := txNodeJsWriter.CreateNodeJS(ctx, jsNode); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store JS node: %w", err)
			}
		}
	}
	// Store condition/if nodes
	if len(results.ConditionNodes) > 0 {
		for _, condNode := range results.ConditionNodes {
			if err := txNodeIfWriter.CreateNodeIf(ctx, condNode); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store condition node: %w", err)
			}
		}
	}
	// Store for nodes
	if len(results.ForNodes) > 0 {
		for _, forNode := range results.ForNodes {
			if err := txNodeForWriter.CreateNodeFor(ctx, forNode); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store for node: %w", err)
			}
		}
	}
	// Store foreach nodes
	if len(results.ForEachNodes) > 0 {
		for _, forEachNode := range results.ForEachNodes {
			if err := txNodeForEachWriter.CreateNodeForEach(ctx, forEachNode); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store foreach node: %w", err)
			}
		}
	}
	// Store AI nodes
	if len(results.AINodes) > 0 {
		for _, aiNode := range results.AINodes {
			if err := txNodeAIWriter.CreateNodeAI(ctx, aiNode); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store AI node: %w", err)
			}
		}
	}
	// Store flow variables
	if len(results.FlowVariables) > 0 {
		for _, flowVar := range results.FlowVariables {
			if err := txFlowVariableWriter.CreateFlowVariable(ctx, flowVar); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store flow variable: %w", err)
			}
		}
	}
	if len(results.Edges) > 0 {
		for _, edge := range results.Edges {
			if err := txEdgeService.CreateEdge(ctx, edge); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store edge: %w", err)
			}
		}
	}

	// 2.6 Update and Store Variables
	// NOTE: targetEnvID was pre-fetched in PHASE 1 to avoid SQLite deadlock
	var storedCreatedVars []menv.Variable
	var storedUpdatedVars []menv.Variable
	if len(results.Variables) > 0 {
		txVarWriter := senv.NewVariableWriter(tx)
		for _, v := range results.Variables {
			v.EnvID = targetEnvID
			if err := txVarWriter.Upsert(ctx, v); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store variable %s: %w", v.VarKey, err)
			}
			// For simplicity, we treat all as created for now in the return slice
			// because Upsert doesn't tell us if it was an insert or update easily
			// and we just want them to appear on the frontend.
			storedCreatedVars = append(storedCreatedVars, v)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return deduplicatedFileIDs, deduplicatedHttpIDs, storedCreatedVars, storedUpdatedVars, nil
}

func storeUnifiedChildren(
	ctx context.Context,
	results *TranslationResult,
	headerSvc *shttp.HeaderWriter,
	paramSvc *shttp.SearchParamWriter,
	formSvc *shttp.BodyFormWriter,
	urlSvc *shttp.BodyUrlEncodedWriter,
	bodyRawSvc *shttp.BodyRawWriter,
	assertSvc *shttp.AssertWriter,
	deduplicatedIDs map[idwrap.IDWrap]bool,
) error {
	// IMPORTANT: We use topological sort to handle arbitrary-depth parent-child relationships.
	// This enables delta chains of any length (delta of delta of delta...).
	// Entities with external parents (not in batch) are treated as roots.

	if len(results.Headers) > 0 {
		// Filter to non-deduplicated headers and update IsDelta
		var headersToInsert []mhttp.HTTPHeader
		for i := range results.Headers {
			h := &results.Headers[i]
			if deduplicatedIDs[h.HttpID] {
				continue
			}
			h.IsDelta = h.IsDelta && h.ParentHttpHeaderID != nil
			headersToInsert = append(headersToInsert, *h)
		}

		// Topologically sort headers
		sortedHeaders := TopologicalSortWithFallback(
			headersToInsert,
			func(h mhttp.HTTPHeader) idwrap.IDWrap { return h.ID },
			func(h mhttp.HTTPHeader) *idwrap.IDWrap { return h.ParentHttpHeaderID },
			nil,
		)

		// Group by HttpID for bulk insert (maintaining sorted order within groups)
		headersByHttpID := make(map[string][]mhttp.HTTPHeader)
		var httpIDOrder []string
		httpIDSeen := make(map[string]bool)
		for _, h := range sortedHeaders {
			key := h.HttpID.String()
			if !httpIDSeen[key] {
				httpIDSeen[key] = true
				httpIDOrder = append(httpIDOrder, key)
			}
			headersByHttpID[key] = append(headersByHttpID[key], h)
		}

		// Insert in topological order
		for _, httpIDStr := range httpIDOrder {
			headers := headersByHttpID[httpIDStr]
			if len(headers) > 0 {
				if err := headerSvc.CreateBulk(ctx, headers[0].HttpID, headers); err != nil {
					return fmt.Errorf("failed to store headers: %w", err)
				}
			}
		}
	}

	if len(results.SearchParams) > 0 {
		// Filter to non-deduplicated search params and update IsDelta
		var paramsToInsert []mhttp.HTTPSearchParam
		for i := range results.SearchParams {
			p := &results.SearchParams[i]
			if deduplicatedIDs[p.HttpID] {
				continue
			}
			p.IsDelta = p.IsDelta && p.ParentHttpSearchParamID != nil
			paramsToInsert = append(paramsToInsert, *p)
		}

		// Topologically sort search params
		sortedParams := TopologicalSortWithFallback(
			paramsToInsert,
			func(p mhttp.HTTPSearchParam) idwrap.IDWrap { return p.ID },
			func(p mhttp.HTTPSearchParam) *idwrap.IDWrap { return p.ParentHttpSearchParamID },
			nil,
		)

		// Group by HttpID for bulk insert (maintaining sorted order within groups)
		paramsByHttpID := make(map[string][]mhttp.HTTPSearchParam)
		var httpIDOrder []string
		httpIDSeen := make(map[string]bool)
		for _, p := range sortedParams {
			key := p.HttpID.String()
			if !httpIDSeen[key] {
				httpIDSeen[key] = true
				httpIDOrder = append(httpIDOrder, key)
			}
			paramsByHttpID[key] = append(paramsByHttpID[key], p)
		}

		// Insert in topological order
		for _, httpIDStr := range httpIDOrder {
			params := paramsByHttpID[httpIDStr]
			if len(params) > 0 {
				if err := paramSvc.CreateBulk(ctx, params[0].HttpID, params); err != nil {
					return fmt.Errorf("failed to store search params: %w", err)
				}
			}
		}
	}

	if len(results.BodyForms) > 0 {
		// Filter to non-deduplicated body forms and update IsDelta
		var formsToInsert []mhttp.HTTPBodyForm
		for i := range results.BodyForms {
			f := &results.BodyForms[i]
			if deduplicatedIDs[f.HttpID] {
				continue
			}
			f.IsDelta = f.IsDelta && f.ParentHttpBodyFormID != nil
			formsToInsert = append(formsToInsert, *f)
		}

		// Topologically sort body forms
		sortedForms := TopologicalSortWithFallback(
			formsToInsert,
			func(f mhttp.HTTPBodyForm) idwrap.IDWrap { return f.ID },
			func(f mhttp.HTTPBodyForm) *idwrap.IDWrap { return f.ParentHttpBodyFormID },
			nil,
		)

		// Insert in topological order
		if len(sortedForms) > 0 {
			if err := formSvc.CreateBulk(ctx, sortedForms); err != nil {
				return fmt.Errorf("failed to store body forms: %w", err)
			}
		}
	}

	if len(results.BodyUrlencoded) > 0 {
		// Filter to non-deduplicated body urlencoded and update IsDelta
		var urlEncodedToInsert []mhttp.HTTPBodyUrlencoded
		for i := range results.BodyUrlencoded {
			u := &results.BodyUrlencoded[i]
			if deduplicatedIDs[u.HttpID] {
				continue
			}
			u.IsDelta = u.IsDelta && u.ParentHttpBodyUrlEncodedID != nil
			urlEncodedToInsert = append(urlEncodedToInsert, *u)
		}

		// Topologically sort body urlencoded
		sortedUrlEncoded := TopologicalSortWithFallback(
			urlEncodedToInsert,
			func(u mhttp.HTTPBodyUrlencoded) idwrap.IDWrap { return u.ID },
			func(u mhttp.HTTPBodyUrlencoded) *idwrap.IDWrap { return u.ParentHttpBodyUrlEncodedID },
			nil,
		)

		// Insert in topological order
		if len(sortedUrlEncoded) > 0 {
			if err := urlSvc.CreateBulk(ctx, sortedUrlEncoded); err != nil {
				return fmt.Errorf("failed to store body urlencoded: %w", err)
			}
		}
	}

	if len(results.BodyRaw) > 0 {
		// Filter to non-deduplicated body raw and update IsDelta
		var bodyRawToInsert []mhttp.HTTPBodyRaw
		for i := range results.BodyRaw {
			body := &results.BodyRaw[i]
			if deduplicatedIDs[body.HttpID] {
				continue
			}
			body.IsDelta = body.IsDelta && body.ParentBodyRawID != nil
			bodyRawToInsert = append(bodyRawToInsert, *body)
		}

		// Topologically sort body raw
		sortedBodyRaw := TopologicalSortWithFallback(
			bodyRawToInsert,
			func(b mhttp.HTTPBodyRaw) idwrap.IDWrap { return b.ID },
			func(b mhttp.HTTPBodyRaw) *idwrap.IDWrap { return b.ParentBodyRawID },
			nil,
		)

		// Insert in topological order
		for _, body := range sortedBodyRaw {
			if _, err := bodyRawSvc.CreateFull(ctx, &body); err != nil {
				return fmt.Errorf("failed to store body raw: %w", err)
			}
		}
	}

	if len(results.Asserts) > 0 {
		// Filter to non-deduplicated assertions and update IsDelta
		var assertsToInsert []mhttp.HTTPAssert
		for i := range results.Asserts {
			a := &results.Asserts[i]
			if deduplicatedIDs[a.HttpID] {
				continue
			}
			a.IsDelta = a.IsDelta && a.ParentHttpAssertID != nil
			assertsToInsert = append(assertsToInsert, *a)
		}

		// Topologically sort assertions
		sortedAsserts := TopologicalSortWithFallback(
			assertsToInsert,
			func(a mhttp.HTTPAssert) idwrap.IDWrap { return a.ID },
			func(a mhttp.HTTPAssert) *idwrap.IDWrap { return a.ParentHttpAssertID },
			nil,
		)

		// Insert in topological order
		if len(sortedAsserts) > 0 {
			if err := assertSvc.CreateBulk(ctx, sortedAsserts); err != nil {
				return fmt.Errorf("failed to store assertions: %w", err)
			}
		}
	}

	return nil
}
