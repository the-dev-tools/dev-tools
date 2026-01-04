//nolint:revive // exported
package rimportv2

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"

	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/harv2"
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
	}
	for i := range results.SearchParams {
		if newID, ok := httpIDMap[results.SearchParams[i].HttpID]; ok {
			results.SearchParams[i].HttpID = newID
		}
	}
	for i := range results.BodyForms {
		if newID, ok := httpIDMap[results.BodyForms[i].HttpID]; ok {
			results.BodyForms[i].HttpID = newID
		}
	}
	for i := range results.BodyUrlencoded {
		if newID, ok := httpIDMap[results.BodyUrlencoded[i].HttpID]; ok {
			results.BodyUrlencoded[i].HttpID = newID
		}
	}
	for i := range results.BodyRaw {
		if newID, ok := httpIDMap[results.BodyRaw[i].HttpID]; ok {
			results.BodyRaw[i].HttpID = newID
		}
	}
	for i := range results.Asserts {
		if newID, ok := httpIDMap[results.Asserts[i].HttpID]; ok {
			results.Asserts[i].HttpID = newID
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
	if len(results.Edges) > 0 {
		for _, edge := range results.Edges {
			if err := txEdgeService.CreateEdge(ctx, edge); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("failed to store edge: %w", err)
			}
		}
	}

	// 2.6 Update and Store Variables
	var storedCreatedVars []menv.Variable
	var storedUpdatedVars []menv.Variable
	if len(results.Variables) > 0 {
		workspace, err := imp.workspaceService.Get(ctx, results.WorkspaceID)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to get workspace for variables: %w", err)
		}

		targetEnvID := workspace.GlobalEnv
		if targetEnvID.Compare(idwrap.IDWrap{}) == 0 {
			// No global environment? This shouldn't happen but let's be safe
			return nil, nil, nil, nil, fmt.Errorf("workspace has no global environment")
		}

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
	if len(results.Headers) > 0 {
		headersByHttpID := make(map[string][]mhttp.HTTPHeader)
		for i := range results.Headers {
			h := &results.Headers[i]
			if deduplicatedIDs[h.HttpID] {
				continue
			}
			h.IsDelta = h.IsDelta && h.ParentHttpHeaderID != nil
			headersByHttpID[h.HttpID.String()] = append(headersByHttpID[h.HttpID.String()], *h)
		}
		for _, headers := range headersByHttpID {
			if len(headers) > 0 {
				if err := headerSvc.CreateBulk(ctx, headers[0].HttpID, headers); err != nil {
					return fmt.Errorf("failed to store headers: %w", err)
				}
			}
		}
	}

	if len(results.SearchParams) > 0 {
		paramsByHttpID := make(map[string][]mhttp.HTTPSearchParam)
		for i := range results.SearchParams {
			p := &results.SearchParams[i]
			if deduplicatedIDs[p.HttpID] {
				continue
			}
			p.IsDelta = p.IsDelta && p.ParentHttpSearchParamID != nil
			paramsByHttpID[p.HttpID.String()] = append(paramsByHttpID[p.HttpID.String()], *p)
		}
		for _, params := range paramsByHttpID {
			if len(params) > 0 {
				if err := paramSvc.CreateBulk(ctx, params[0].HttpID, params); err != nil {
					return fmt.Errorf("failed to store search params: %w", err)
				}
			}
		}
	}

	if len(results.BodyForms) > 0 {
		var toInsert []mhttp.HTTPBodyForm
		for i := range results.BodyForms {
			f := &results.BodyForms[i]
			if !deduplicatedIDs[f.HttpID] {
				f.IsDelta = f.IsDelta && f.ParentHttpBodyFormID != nil
				toInsert = append(toInsert, *f)
			}
		}
		if len(toInsert) > 0 {
			if err := formSvc.CreateBulk(ctx, toInsert); err != nil {
				return fmt.Errorf("failed to store body forms: %w", err)
			}
		}
	}

	if len(results.BodyUrlencoded) > 0 {
		var toInsert []mhttp.HTTPBodyUrlencoded
		for i := range results.BodyUrlencoded {
			u := &results.BodyUrlencoded[i]
			if !deduplicatedIDs[u.HttpID] {
				u.IsDelta = u.IsDelta && u.ParentHttpBodyUrlEncodedID != nil
				toInsert = append(toInsert, *u)
			}
		}
		if len(toInsert) > 0 {
			if err := urlSvc.CreateBulk(ctx, toInsert); err != nil {
				return fmt.Errorf("failed to store body urlencoded: %w", err)
			}
		}
	}

	if len(results.BodyRaw) > 0 {
		for i := range results.BodyRaw {
			body := &results.BodyRaw[i]
			if !deduplicatedIDs[body.HttpID] {
				if _, err := bodyRawSvc.CreateFull(ctx, body); err != nil {
					return fmt.Errorf("failed to store body raw: %w", err)
				}
			}
		}
	}

	if len(results.Asserts) > 0 {
		var toInsert []mhttp.HTTPAssert
		for i := range results.Asserts {
			a := &results.Asserts[i]
			if !deduplicatedIDs[a.HttpID] {
				a.IsDelta = a.IsDelta && a.ParentHttpAssertID != nil
				toInsert = append(toInsert, *a)
			}
		}
		if len(toInsert) > 0 {
			if err := assertSvc.CreateBulk(ctx, toInsert); err != nil {
				return fmt.Errorf("failed to store assertions: %w", err)
			}
		}
	}

	return nil
}
