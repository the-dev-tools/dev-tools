package rimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/ioworkspace"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/postman/v21/mpostmancollection"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/positioning"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tcurl"
	"the-dev-tools/server/pkg/translate/thar"
	"the-dev-tools/server/pkg/translate/tpostman"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
	"the-dev-tools/spec/dist/buf/go/import/v1/importv1connect"

	"connectrpc.com/connect"
)

// TODO: this is need be switch to id based system later
var lastHar thar.HAR

// Custom error types to distinguish between parsing and database errors
var (
	ErrHARParsing = errors.New("failed to parse HAR file")
	ErrPostmanParsing = errors.New("failed to parse Postman collection")
	ErrDatabaseOperation = errors.New("database operation failed")
)

type ImportRPC struct {
	DB   *sql.DB
	ws   sworkspace.WorkspaceService
	cs   scollection.CollectionService
	us   suser.UserService
	ifs  sitemfolder.ItemFolderService
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	res  sexampleresp.ExampleRespService
	as   sassert.AssertService
	
	// Additional services needed for workflow YAML import
	ehs  sexampleheader.HeaderService
	eqs  sexamplequery.ExampleQueryService
	rbs  sbodyraw.BodyRawService
	fbs  sbodyform.BodyFormService
	ubs  sbodyurl.BodyURLEncodedService
	rhs  sexamplerespheader.ExampleRespHeaderService
	ars  sassertres.AssertResultService
	
	// Flow services
	fs   sflow.FlowService
	ns   snode.NodeService
	es   sedge.EdgeService
	fvs  sflowvariable.FlowVariableService
	
	frs  snoderequest.NodeRequestService
	fcs  snodeif.NodeIfService
	fns  snodenoop.NodeNoopService
	ffors snodefor.NodeForService
	ffes snodeforeach.NodeForEachService
	fjs  snodejs.NodeJSService
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, cs scollection.CollectionService, us suser.UserService,
	ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService,
	iaes sitemapiexample.ItemApiExampleService, res sexampleresp.ExampleRespService,
	as sassert.AssertService,
	// Additional services
	ehs sexampleheader.HeaderService,
	eqs sexamplequery.ExampleQueryService,
	rbs sbodyraw.BodyRawService,
	fbs sbodyform.BodyFormService,
	ubs sbodyurl.BodyURLEncodedService,
	rhs sexamplerespheader.ExampleRespHeaderService,
	ars sassertres.AssertResultService,
	// Flow services
	fs sflow.FlowService,
	ns snode.NodeService,
	es sedge.EdgeService,
	fvs sflowvariable.FlowVariableService,
	frs snoderequest.NodeRequestService,
	fcs snodeif.NodeIfService,
	fns snodenoop.NodeNoopService,
	ffors snodefor.NodeForService,
	ffes snodeforeach.NodeForEachService,
	fjs snodejs.NodeJSService,
) ImportRPC {
	return ImportRPC{
		DB:   db,
		ws:   ws,
		cs:   cs,
		us:   us,
		ifs:  ifs,
		ias:  ias,
		iaes: iaes,
		res:  res,
		as:   as,
		ehs:  ehs,
		eqs:  eqs,
		rbs:  rbs,
		fbs:  fbs,
		ubs:  ubs,
		rhs:  rhs,
		ars:  ars,
		fs:   fs,
		ns:   ns,
		es:   es,
		fvs:  fvs,
		frs:  frs,
		fcs:  fcs,
		fns:  fns,
		ffors: ffors,
		ffes: ffes,
		fjs:  fjs,
	}
}

func CreateService(srv ImportRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := importv1connect.NewImportServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ImportRPC) Import(ctx context.Context, req *connect.Request[importv1.ImportRequest]) (*connect.Response[importv1.ImportResponse], error) {
	wsUlid, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, wsUlid)); rpcErr != nil {
		return nil, rpcErr
	}

	data := req.Msg.Data
	textData := req.Msg.TextData
	resp := &importv1.ImportResponse{}

	// Check if a collection with this name already exists in the workspace
	var collectionID idwrap.IDWrap
	// Determine collection name based on import type
	// For HAR imports (when we have data that's valid JSON), use "Imported"
	// For other imports (curl with textData), use the provided name
	collectionName := req.Msg.Name
	// Check if this is a HAR import (either initial parse or filtered import)
	isHARImport := len(textData) == 0 && (json.Valid(data) || len(req.Msg.Filter) > 0)
	if isHARImport {
		// This is a HAR import, use "Imported" as collection name
		collectionName = "Imported"
	}
	
	existingCollection, err := c.cs.GetCollectionByWorkspaceIDAndName(ctx, wsUlid, collectionName)
	switch err {
	case nil:
		// Collection exists, use its ID
		collectionID = existingCollection.ID
		// Found existing collection, will merge endpoints into it
	case scollection.ErrNoCollectionFound:
		// Collection doesn't exist, generate new ID
		collectionID = idwrap.NewNow()
		// Collection doesn't exist, will create new one
	default:
		// Some other error occurred
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If no filter provided, we need to parse and present filter options
	if len(req.Msg.Filter) == 0 {
		// Handle curl import
		if len(textData) > 0 {
			curlResolved, err := tcurl.ConvertCurl(textData, collectionID)
			if err != nil {
				return nil, err
			}

			if len(curlResolved.Apis) == 0 {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no api found"))
			}

			err = c.ImportCurl(ctx, wsUlid, collectionID, req.Msg.Name, curlResolved)
			if err != nil {
				return nil, err
			}

			return connect.NewResponse(resp), nil
		}

		// Try to import as simplified workflow YAML first
		workspaceData, err := ioworkspace.UnmarshalWorkflowYAML(data)
		if err == nil {
			// Successfully parsed as workflow YAML, import it
			err = c.ImportWorkflowYAML(ctx, wsUlid, workspaceData)
			if err != nil {
				return nil, err
			}
			return connect.NewResponse(resp), nil
		}

		// Handle other imports
		if !json.Valid(data) {
			return nil, errors.New("invalid json")
		}

		// Determine the type of file (HAR file)
		har, err := thar.ConvertRaw(data)
		if err != nil {
			return nil, err
		}

		// Extract unique domains for filtering
		domains := make(map[string]struct{}, len(har.Log.Entries))
		for _, entry := range har.Log.Entries {
			if thar.IsXHRRequest(entry) {
				urlData, err := url.Parse(entry.Request.URL)
				if err != nil {
					return nil, err
				}
				domains[urlData.Host] = struct{}{}
			}
		}

		// Return filter options to the client
		resp.Kind = importv1.ImportKind_IMPORT_KIND_FILTER
		keys := make([]string, 0, len(domains))
		for k := range domains {
			keys = append(keys, k)
		}
		resp.Filter = keys

		// Save HAR for subsequent filtered import
		lastHar = *har

		return connect.NewResponse(resp), nil
	}

	// Process filtered entries
	var filteredEntries []thar.Entry
	urlMap := make(map[string][]thar.Entry)

	// If lastHar is empty but we have data, parse it
	// This handles cases where the filter request comes from a different context
	if len(lastHar.Log.Entries) == 0 && len(data) > 0 && json.Valid(data) {
		har, err := thar.ConvertRaw(data)
		if err != nil {
			return nil, err
		}
		lastHar = *har
	}

	for _, entry := range lastHar.Log.Entries {
		if thar.IsXHRRequest(entry) {
			urlData, err := url.Parse(entry.Request.URL)
			if err != nil {
				return nil, err
			}

			host := urlData.Host
			entries, ok := urlMap[host]
			if !ok {
				entries = make([]thar.Entry, 0)
			}
			entries = append(entries, entry)
			urlMap[host] = entries
		}
	}

	for _, filter := range req.Msg.Filter {
		if entries, ok := urlMap[filter]; ok {
			filteredEntries = append(filteredEntries, entries...)
		}
	}
	lastHar.Log.Entries = filteredEntries

	// Try to import as HAR
	// Attempt HAR import with filtered entries
	// Use "Imported" as the collection name for HAR imports
	flow, err := c.ImportHar(ctx, wsUlid, collectionID, "Imported", &lastHar)
	if err == nil {
		// For HAR imports, we also create a flow
		if flow != nil {
			resp.Flow = &flowv1.FlowListItem{
				FlowId: flow.ID.Bytes(),
				Name:   flow.Name,
			}
		}

		return connect.NewResponse(resp), nil
	}

	// Check if error is due to database operation failure
	// In this case, we should not attempt fallback
	var connectErr *connect.Error
	if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeInternal {
		// Database error occurred, return immediately without fallback
		return nil, err
	}

	// HAR import failed due to parsing/conversion, try Postman Collection

	// Try to import as Postman Collection
	postman, err := tpostman.ParsePostmanCollection(data)
	if err != nil {
		// Postman collection parsing also failed
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse as HAR or Postman collection: %w", err))
	}

	// Postman collection parsed successfully, attempting import
	// For consistency, use "Imported" collection name if this was originally a HAR import attempt
	postmanCollectionName := req.Msg.Name
	if isHARImport {
		postmanCollectionName = "Imported"
	}
	err = c.ImportPostmanCollection(ctx, wsUlid, collectionID, postmanCollectionName, postman)
	if err == nil {
		// Postman collection import successful (no flow created)
		return connect.NewResponse(resp), nil
	}

	// Return the actual error from import
	return nil, err
}

func (c *ImportRPC) ImportCurl(ctx context.Context, workspaceID, CollectionID idwrap.IDWrap, name string, resolvedCurl tcurl.CurlResolved) error {
	collection := mcollection.Collection{
		ID:          CollectionID,
		Name:        name,
		WorkspaceID: workspaceID,
	}

	// Check if collection already exists
	collectionExists := false
	_, err := c.cs.GetCollection(ctx, CollectionID)
	if err == nil {
		collectionExists = true
		// Collection already exists, will merge endpoints
	} else if err != scollection.ErrNoCollectionFound {
		return connect.NewError(connect.CodeInternal, err)
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	defer devtoolsdb.TxnRollback(tx)

	txCollectionService, err := scollection.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Only create collection if it doesn't exist
	if !collectionExists {
		err = txCollectionService.CreateCollection(ctx, &collection)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txItemApiService.CreateItemApiBulk(ctx, resolvedCurl.Apis)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, resolvedCurl.Examples)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// START BODY
	txBodyRawService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyRawService.CreateBulkBodyRaw(ctx, resolvedCurl.RawBodies)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService, err := sbodyform.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyFormService.CreateBulkBodyForm(ctx, resolvedCurl.FormBodies)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService, err := sbodyurl.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, resolvedCurl.UrlEncodedBodies)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// END BODY

	txHeaderService, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txHeaderService.CreateBulkHeader(ctx, resolvedCurl.Headers)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	txQueriesService, err := sexamplequery.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txQueriesService.CreateBulkQuery(ctx, resolvedCurl.Queries)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Update workspace counts and timestamp inside transaction
	txWorkspaceService, err := sworkspace.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	ws, err := txWorkspaceService.Get(ctx, workspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// Only increment collection count if we created a new collection
	if !collectionExists {
		ws.CollectionCount++
	}
	ws.Updated = dbtime.DBNow()
	err = txWorkspaceService.Update(ctx, ws)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	return nil
}

func (c *ImportRPC) ImportPostmanCollection(ctx context.Context, workspaceID, CollectionID idwrap.IDWrap, name string, collectionData mpostmancollection.Collection) error {
	collection := mcollection.Collection{
		ID:          CollectionID,
		Name:        name,
		WorkspaceID: workspaceID,
	}

	// Check if collection already exists
	collectionExists := false
	_, err := c.cs.GetCollection(ctx, CollectionID)
	if err == nil {
		collectionExists = true
		// Collection already exists, will merge endpoints
	} else if err != scollection.ErrNoCollectionFound {
		return connect.NewError(connect.CodeInternal, err)
	}

	items, err := tpostman.ConvertPostmanCollection(collectionData, CollectionID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txCollectionService, err := scollection.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Only create collection if it doesn't exist
	if !collectionExists {
		err = txCollectionService.CreateCollection(ctx, &collection)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemFolderService, err := sitemfolder.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txItemFolderService.CreateItemFolderBulk(ctx, items.Folders)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txItemApiService.CreateItemApiBulk(ctx, items.Apis)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, items.ApiExamples)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// START BODY
	txBodyRawService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyRawService.CreateBulkBodyRaw(ctx, items.BodyRaw)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService, err := sbodyform.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyFormService.CreateBulkBodyForm(ctx, items.BodyForm)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService, err := sbodyurl.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, items.BodyUrlEncoded)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// END BODY

	txHeaderService, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txHeaderService.CreateBulkHeader(ctx, items.Headers)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	txQueriesService, err := sexamplequery.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	err = txQueriesService.CreateBulkQuery(ctx, items.Queries)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Update workspace counts and timestamp inside transaction
	txWorkspaceService, err := sworkspace.NewTX(ctx, tx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	ws, err := txWorkspaceService.Get(ctx, workspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// Only increment collection count if we created a new collection
	if !collectionExists {
		ws.CollectionCount++
	}
	ws.Updated = dbtime.DBNow()
	err = txWorkspaceService.Update(ctx, ws)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	return nil
}

func (c *ImportRPC) ImportHar(ctx context.Context, workspaceID, CollectionID idwrap.IDWrap, name string, harData *thar.HAR) (*mflow.Flow, error) {
	// Check if collection already exists
	collectionExists := false
	_, err := c.cs.GetCollection(ctx, CollectionID)
	if err == nil {
		collectionExists = true
		// Collection already exists, will merge endpoints
	} else if err != scollection.ErrNoCollectionFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Pre-load existing folders if collection exists
	var existingFolders []mitemfolder.ItemFolder
	if collectionExists {
		existingFolders, err = c.ifs.GetFoldersWithCollectionID(ctx, CollectionID)
		if err != nil && err != sitemfolder.ErrNoItemFolderFound {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Import HAR data into collection with existing folder info
	resolved, err := thar.ConvertHARWithExistingData(harData, CollectionID, workspaceID, existingFolders)
	if err != nil {
		// HAR conversion failed
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// HAR conversion successful

	if len(resolved.Apis) == 0 {
		// No APIs found in HAR conversion
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no apis found to create in har"))
	}

	collectionData := mcollection.Collection{
		ID:          CollectionID,
		Name:        name,
		WorkspaceID: workspaceID,
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	txCollectionService, err := scollection.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Only create collection if it doesn't exist
	if !collectionExists {
		err = txCollectionService.CreateCollection(ctx, &collectionData)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemFolderService, err := sitemfolder.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Pre-load existing folders to optimize lookup
	existingFoldersList, err := txItemFolderService.GetFoldersWithCollectionID(ctx, CollectionID)
	if err != nil && err != sitemfolder.ErrNoItemFolderFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build a map to track folder updates
	folderIDMapping := make(map[idwrap.IDWrap]idwrap.IDWrap) // old ID -> new/existing ID

	// Create folders first (they need to exist before APIs reference them)
	if len(resolved.Folders) > 0 {
		// Build a map of existing folders by ID for quick lookup
		existingFolderByID := make(map[idwrap.IDWrap]*mitemfolder.ItemFolder)
		for i := range existingFoldersList {
			existingFolderByID[existingFoldersList[i].ID] = &existingFoldersList[i]
		}

		// Filter out folders that already exist and build ID mapping
		var foldersToCreate []mitemfolder.ItemFolder
		for i := range resolved.Folders {
			folder := &resolved.Folders[i]
			// Check if folder already exists by name and parent
			exists := false
			for _, existing := range existingFoldersList {
				if existing.Name == folder.Name &&
					((existing.ParentID == nil && folder.ParentID == nil) ||
						(existing.ParentID != nil && folder.ParentID != nil && existing.ParentID.Compare(*folder.ParentID) == 0)) {
					exists = true
					// Map old ID to existing ID
					folderIDMapping[folder.ID] = existing.ID
					// Update the folder ID in resolved.Folders to use existing ID
					folder.ID = existing.ID
					break
				}
			}
			if !exists {
				// Keep the same ID for new folders
				folderIDMapping[folder.ID] = folder.ID
				foldersToCreate = append(foldersToCreate, *folder)
			}
		}

		// Update parent IDs in folders to create based on mapping
		for i := range foldersToCreate {
			if foldersToCreate[i].ParentID != nil {
				if mappedID, ok := folderIDMapping[*foldersToCreate[i].ParentID]; ok {
					foldersToCreate[i].ParentID = &mappedID
				}
			}
		}

		// Sort folders by hierarchy depth to ensure parents are created before children
		sortFoldersByDepth(foldersToCreate)

		// Only create folders that don't exist
		if len(foldersToCreate) > 0 {
			// CreateItemFolderBulk expects exactly 10 items, so we need to batch or create individually
			for i := 0; i < len(foldersToCreate); i += 10 {
				end := i + 10
				if end > len(foldersToCreate) {
					// Create remaining items individually
					for j := i; j < len(foldersToCreate); j++ {
						err = txItemFolderService.CreateItemFolder(ctx, &foldersToCreate[j])
						if err != nil {
							return nil, connect.NewError(connect.CodeInternal, err)
						}
					}
				} else {
					// Create batch of exactly 10
					batch := foldersToCreate[i:end]
					err = txItemFolderService.CreateItemFolderBulk(ctx, batch)
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				}
			}
		}
	}

	// Reload folders after creation to get updated list
	existingFoldersList, err = txItemFolderService.GetFoldersWithCollectionID(ctx, CollectionID)
	if err != nil && err != sitemfolder.ErrNoItemFolderFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Build a map for quick folder lookup by path
	existingFolderMap := make(map[string]idwrap.IDWrap)
	for _, folder := range existingFoldersList {
		// We'll need to reconstruct the folder path - for now just use the folder ID
		existingFolderMap[folder.Name] = folder.ID
	}

	// Update API folder references based on folder mapping (if folders were processed)
	if len(resolved.Folders) > 0 && len(folderIDMapping) > 0 {
		for i := range resolved.Apis {
			if resolved.Apis[i].FolderID != nil {
				if mappedID, ok := folderIDMapping[*resolved.Apis[i].FolderID]; ok {
					resolved.Apis[i].FolderID = &mappedID
				}
			}
		}
	}

	// Handle endpoint creation/update with duplicate checking
	apiMapping := make(map[idwrap.IDWrap]idwrap.IDWrap) // Map old API ID to new/existing API ID
	var apisToCreate []mitemapi.ItemApi
	var apisToUpdate []mitemapi.ItemApi

	// Batch load all existing endpoints for this collection
	existingApis, err := txItemApiService.GetApisWithCollectionID(ctx, CollectionID)
	if err != nil && err != sitemapi.ErrNoItemApiFound {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create a map for quick lookup
	existingApiMap := make(map[string]*mitemapi.ItemApi)
	for i := range existingApis {
		api := &existingApis[i]
		key := api.Url + "|" + api.Method
		existingApiMap[key] = api
	}

	// Check each endpoint to see if it already exists
	for _, api := range resolved.Apis {
		// Skip delta endpoints for now - handle them separately
		if api.DeltaParentID != nil {
			continue
		}

		key := api.Url + "|" + api.Method
		existingApi := existingApiMap[key]

		if existingApi != nil {
			// Endpoint exists - check if update is needed
			needsUpdate := false
			if existingApi.Name != api.Name {
				needsUpdate = true
				existingApi.Name = api.Name
			}
			if (existingApi.FolderID == nil && api.FolderID != nil) ||
				(existingApi.FolderID != nil && api.FolderID == nil) ||
				(existingApi.FolderID != nil && api.FolderID != nil && existingApi.FolderID.Compare(*api.FolderID) != 0) {
				needsUpdate = true
				existingApi.FolderID = api.FolderID
			}

			apiMapping[api.ID] = existingApi.ID
			if needsUpdate {
				apisToUpdate = append(apisToUpdate, *existingApi)
			}
		} else {
			// New endpoint - create it
			apiMapping[api.ID] = api.ID
			apisToCreate = append(apisToCreate, api)
		}
	}

	// Handle delta endpoints
	for _, api := range resolved.Apis {
		if api.DeltaParentID != nil {
			// Map delta parent to the actual/existing API ID
			if mappedID, ok := apiMapping[*api.DeltaParentID]; ok {
				newDeltaParentID := mappedID
				api.DeltaParentID = &newDeltaParentID
			}
			apisToCreate = append(apisToCreate, api)
		}
	}

	// Create new endpoints
	if len(apisToCreate) > 0 {
		// CreateItemApiBulk expects exactly 10 items, so we need to batch or create individually
		for i := 0; i < len(apisToCreate); i += 10 {
			end := i + 10
			if end > len(apisToCreate) {
				// Create remaining items individually
				for j := i; j < len(apisToCreate); j++ {
					err = txItemApiService.CreateItemApi(ctx, &apisToCreate[j])
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				}
			} else {
				// Create batch of exactly 10
				batch := apisToCreate[i:end]
				err = txItemApiService.CreateItemApiBulk(ctx, batch)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	// Update existing endpoints
	for _, api := range apisToUpdate {
		err = txItemApiService.UpdateItemApi(ctx, &api)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update example API IDs based on mapping
	var updatedExamples []mitemapiexample.ItemApiExample
	for _, example := range resolved.Examples {
		if mappedID, ok := apiMapping[example.ItemApiID]; ok {
			example.ItemApiID = mappedID
		}
		updatedExamples = append(updatedExamples, example)
	}

	// TODO: For existing endpoints, we should check if we need to delete old examples
	// For now, just create new examples
	if len(updatedExamples) > 0 {
		// CreateApiExampleBulk expects exactly 10 items, so we need to batch or create individually
		for i := 0; i < len(updatedExamples); i += 10 {
			end := i + 10
			if end > len(updatedExamples) {
				// Create remaining items individually
				for j := i; j < len(updatedExamples); j++ {
					err = txItemApiExampleService.CreateApiExample(ctx, &updatedExamples[j])
					if err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				}
			} else {
				// Create batch of exactly 10
				batch := updatedExamples[i:end]
				err = txItemApiExampleService.CreateApiExampleBulk(ctx, batch)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	txExampleHeaderService, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txExampleHeaderService.CreateBulkHeader(ctx, resolved.Headers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txExampleQueryService, err := sexamplequery.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txExampleQueryService.CreateBulkQuery(ctx, resolved.Queries)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyRawService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txBodyRawService.CreateBulkBodyRaw(ctx, resolved.RawBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService, err := sbodyform.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyFormService.CreateBulkBodyForm(ctx, resolved.FormBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService, err := sbodyurl.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, resolved.UrlEncodedBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Assertions
	if len(resolved.Asserts) > 0 {
		txAssertService, err := sassert.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = txAssertService.CreateBulkAssert(ctx, resolved.Asserts)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Flow Creation
	txFlowService, err := sflow.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txFlowService.CreateFlow(ctx, resolved.Flow)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Flow Node
	txFlowNodeService, err := snode.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txFlowNodeService.CreateNodeBulk(ctx, resolved.Nodes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Flow Request Nodes
	// Create flow request nodes
	txFlowRequestService, err := snoderequest.NewTX(ctx, tx)
	if err != nil {
		// Failed to create txFlowRequestService
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update request nodes with mapped endpoint IDs
	var updatedRequestNodes []mnrequest.MNRequest
	for _, node := range resolved.RequestNodes {
		if node.EndpointID != nil {
			if mappedID, ok := apiMapping[*node.EndpointID]; ok {
				node.EndpointID = &mappedID
			}
		}
		if node.DeltaEndpointID != nil {
			if mappedID, ok := apiMapping[*node.DeltaEndpointID]; ok {
				node.DeltaEndpointID = &mappedID
			}
		}
		updatedRequestNodes = append(updatedRequestNodes, node)
	}

	err = txFlowRequestService.CreateNodeRequestBulk(ctx, updatedRequestNodes)
	if err != nil {
		// CreateNodeRequestBulk failed
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Flow request nodes created successfully

	// Flow Noop Nodes
	txFlowNoopService, err := snodenoop.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txFlowNoopService.CreateNodeNoopBulk(ctx, resolved.NoopNodes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Edges
	txFlowEdgeService, err := sedge.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txFlowEdgeService.CreateEdgeBulk(ctx, resolved.Edges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update workspace counts and timestamp inside transaction
	txWorkspaceService, err := sworkspace.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ws, err := txWorkspaceService.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Only increment collection count if we created a new collection
	if !collectionExists {
		ws.CollectionCount++
	}
	ws.FlowCount++
	ws.Updated = dbtime.DBNow()
	err = txWorkspaceService.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		// Transaction commit failed
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// HAR import completed successfully
	// Return a pointer to the Flow
	flow := resolved.Flow
	return &flow, nil
}

// sortFoldersByDepth sorts folders so that parent folders come before their children
func sortFoldersByDepth(folders []mitemfolder.ItemFolder) {
	// Build a map of folder IDs to their indices
	folderIndex := make(map[idwrap.IDWrap]int)
	for i, folder := range folders {
		folderIndex[folder.ID] = i
	}

	// Calculate depth for each folder
	depths := make([]int, len(folders))
	for i := range folders {
		depths[i] = calculateFolderDepth(&folders[i], folders, folderIndex, make(map[idwrap.IDWrap]bool))
	}

	// Sort by depth (parents first)
	sort.SliceStable(folders, func(i, j int) bool {
		return depths[i] < depths[j]
	})
}

// calculateFolderDepth calculates the depth of a folder in the hierarchy
func calculateFolderDepth(folder *mitemfolder.ItemFolder, allFolders []mitemfolder.ItemFolder, folderIndex map[idwrap.IDWrap]int, visited map[idwrap.IDWrap]bool) int {
	if folder.ParentID == nil {
		return 0
	}

	// Check for circular references
	if visited[folder.ID] {
		return 0
	}
	visited[folder.ID] = true

	// Find parent in the list
	if parentIdx, ok := folderIndex[*folder.ParentID]; ok {
		return 1 + calculateFolderDepth(&allFolders[parentIdx], allFolders, folderIndex, visited)
	}

	// Parent not in list, assume it exists in DB
	return 1
}

// ImportWorkflowYAML imports a workspace from the simplified workflow YAML format
func (c *ImportRPC) ImportWorkflowYAML(ctx context.Context, workspaceID idwrap.IDWrap, workspaceData *ioworkspace.WorkspaceData) error {
	// Regenerate all IDs to avoid conflicts
	idMap := make(map[idwrap.IDWrap]idwrap.IDWrap)
	
	// Helper function to get new ID or create one if not exists
	getNewID := func(oldID idwrap.IDWrap) idwrap.IDWrap {
		// Handle zero/empty IDs
		if oldID == (idwrap.IDWrap{}) {
			return idwrap.IDWrap{}
		}
		if newID, exists := idMap[oldID]; exists {
			return newID
		}
		newID := idwrap.NewNow()
		idMap[oldID] = newID
		return newID
	}
	
	// Update workspace ID
	oldWorkspaceID := workspaceData.Workspace.ID
	workspaceData.Workspace.ID = workspaceID
	idMap[oldWorkspaceID] = workspaceID

	// Regenerate collection IDs
	for i := range workspaceData.Collections {
		oldID := workspaceData.Collections[i].ID
		workspaceData.Collections[i].ID = getNewID(oldID)
		workspaceData.Collections[i].WorkspaceID = workspaceID
	}

	// Regenerate flow IDs
	for i := range workspaceData.Flows {
		oldID := workspaceData.Flows[i].ID
		workspaceData.Flows[i].ID = getNewID(oldID)
		workspaceData.Flows[i].WorkspaceID = workspaceID
	}

	// Regenerate flow node IDs
	for i := range workspaceData.FlowNodes {
		oldID := workspaceData.FlowNodes[i].ID
		workspaceData.FlowNodes[i].ID = getNewID(oldID)
		if workspaceData.FlowNodes[i].FlowID != (idwrap.IDWrap{}) {
			workspaceData.FlowNodes[i].FlowID = getNewID(workspaceData.FlowNodes[i].FlowID)
		}
	}

	// Regenerate flow edge IDs
	for i := range workspaceData.FlowEdges {
		oldID := workspaceData.FlowEdges[i].ID
		workspaceData.FlowEdges[i].ID = getNewID(oldID)
		workspaceData.FlowEdges[i].FlowID = getNewID(workspaceData.FlowEdges[i].FlowID)
		workspaceData.FlowEdges[i].SourceID = getNewID(workspaceData.FlowEdges[i].SourceID)
		workspaceData.FlowEdges[i].TargetID = getNewID(workspaceData.FlowEdges[i].TargetID)
	}

	// Regenerate flow variable IDs
	for i := range workspaceData.FlowVariables {
		oldID := workspaceData.FlowVariables[i].ID
		workspaceData.FlowVariables[i].ID = getNewID(oldID)
		workspaceData.FlowVariables[i].FlowID = getNewID(workspaceData.FlowVariables[i].FlowID)
	}

	// Regenerate folder IDs
	for i := range workspaceData.Folders {
		oldID := workspaceData.Folders[i].ID
		workspaceData.Folders[i].ID = getNewID(oldID)
		workspaceData.Folders[i].CollectionID = getNewID(workspaceData.Folders[i].CollectionID)
		if workspaceData.Folders[i].ParentID != nil {
			newParentID := getNewID(*workspaceData.Folders[i].ParentID)
			workspaceData.Folders[i].ParentID = &newParentID
		}
	}

	// Regenerate endpoint IDs
	for i := range workspaceData.Endpoints {
		oldID := workspaceData.Endpoints[i].ID
		workspaceData.Endpoints[i].ID = getNewID(oldID)
		workspaceData.Endpoints[i].CollectionID = getNewID(workspaceData.Endpoints[i].CollectionID)
		if workspaceData.Endpoints[i].FolderID != nil {
			newFolderID := getNewID(*workspaceData.Endpoints[i].FolderID)
			workspaceData.Endpoints[i].FolderID = &newFolderID
		}
	}

	// Regenerate example IDs
	for i := range workspaceData.Examples {
		oldID := workspaceData.Examples[i].ID
		workspaceData.Examples[i].ID = getNewID(oldID)
		workspaceData.Examples[i].CollectionID = getNewID(workspaceData.Examples[i].CollectionID)
		workspaceData.Examples[i].ItemApiID = getNewID(workspaceData.Examples[i].ItemApiID)
	}

	// Update flow request nodes
	for i := range workspaceData.FlowRequestNodes {
		workspaceData.FlowRequestNodes[i].FlowNodeID = getNewID(workspaceData.FlowRequestNodes[i].FlowNodeID)
		if workspaceData.FlowRequestNodes[i].EndpointID != nil {
			newEndpointID := getNewID(*workspaceData.FlowRequestNodes[i].EndpointID)
			workspaceData.FlowRequestNodes[i].EndpointID = &newEndpointID
		}
		if workspaceData.FlowRequestNodes[i].DeltaEndpointID != nil {
			newDeltaEndpointID := getNewID(*workspaceData.FlowRequestNodes[i].DeltaEndpointID)
			workspaceData.FlowRequestNodes[i].DeltaEndpointID = &newDeltaEndpointID
		}
		if workspaceData.FlowRequestNodes[i].ExampleID != nil {
			newExampleID := getNewID(*workspaceData.FlowRequestNodes[i].ExampleID)
			workspaceData.FlowRequestNodes[i].ExampleID = &newExampleID
		}
		if workspaceData.FlowRequestNodes[i].DeltaExampleID != nil {
			newDeltaExampleID := getNewID(*workspaceData.FlowRequestNodes[i].DeltaExampleID)
			workspaceData.FlowRequestNodes[i].DeltaExampleID = &newDeltaExampleID
		}
	}

	// Update flow condition nodes
	for i := range workspaceData.FlowConditionNodes {
		workspaceData.FlowConditionNodes[i].FlowNodeID = getNewID(workspaceData.FlowConditionNodes[i].FlowNodeID)
	}

	// Update flow noop nodes
	for i := range workspaceData.FlowNoopNodes {
		workspaceData.FlowNoopNodes[i].FlowNodeID = getNewID(workspaceData.FlowNoopNodes[i].FlowNodeID)
	}

	// Update flow for nodes
	for i := range workspaceData.FlowForNodes {
		workspaceData.FlowForNodes[i].FlowNodeID = getNewID(workspaceData.FlowForNodes[i].FlowNodeID)
	}

	// Update flow for each nodes
	for i := range workspaceData.FlowForEachNodes {
		workspaceData.FlowForEachNodes[i].FlowNodeID = getNewID(workspaceData.FlowForEachNodes[i].FlowNodeID)
	}

	// Update flow JS nodes
	for i := range workspaceData.FlowJSNodes {
		workspaceData.FlowJSNodes[i].FlowNodeID = getNewID(workspaceData.FlowJSNodes[i].FlowNodeID)
	}

	// Update example headers
	for i := range workspaceData.ExampleHeaders {
		oldID := workspaceData.ExampleHeaders[i].ID
		workspaceData.ExampleHeaders[i].ID = getNewID(oldID)
		workspaceData.ExampleHeaders[i].ExampleID = getNewID(workspaceData.ExampleHeaders[i].ExampleID)
	}

	// Update example queries
	for i := range workspaceData.ExampleQueries {
		oldID := workspaceData.ExampleQueries[i].ID
		workspaceData.ExampleQueries[i].ID = getNewID(oldID)
		workspaceData.ExampleQueries[i].ExampleID = getNewID(workspaceData.ExampleQueries[i].ExampleID)
	}

	// Update raw bodies
	for i := range workspaceData.Rawbodies {
		oldID := workspaceData.Rawbodies[i].ID
		workspaceData.Rawbodies[i].ID = getNewID(oldID)
		workspaceData.Rawbodies[i].ExampleID = getNewID(workspaceData.Rawbodies[i].ExampleID)
	}

	// Update form bodies
	for i := range workspaceData.FormBodies {
		oldID := workspaceData.FormBodies[i].ID
		workspaceData.FormBodies[i].ID = getNewID(oldID)
		workspaceData.FormBodies[i].ExampleID = getNewID(workspaceData.FormBodies[i].ExampleID)
	}

	// Update URL bodies
	for i := range workspaceData.UrlBodies {
		oldID := workspaceData.UrlBodies[i].ID
		workspaceData.UrlBodies[i].ID = getNewID(oldID)
		workspaceData.UrlBodies[i].ExampleID = getNewID(workspaceData.UrlBodies[i].ExampleID)
	}

	// Update example responses
	for i := range workspaceData.ExampleResponses {
		oldID := workspaceData.ExampleResponses[i].ID
		workspaceData.ExampleResponses[i].ID = getNewID(oldID)
		workspaceData.ExampleResponses[i].ExampleID = getNewID(workspaceData.ExampleResponses[i].ExampleID)
	}

	// Update example response headers
	for i := range workspaceData.ExampleResponseHeaders {
		oldID := workspaceData.ExampleResponseHeaders[i].ID
		workspaceData.ExampleResponseHeaders[i].ID = getNewID(oldID)
		workspaceData.ExampleResponseHeaders[i].ExampleRespID = getNewID(workspaceData.ExampleResponseHeaders[i].ExampleRespID)
	}

	// Update example asserts
	for i := range workspaceData.ExampleAsserts {
		oldID := workspaceData.ExampleAsserts[i].ID
		workspaceData.ExampleAsserts[i].ID = getNewID(oldID)
		workspaceData.ExampleAsserts[i].ExampleID = getNewID(workspaceData.ExampleAsserts[i].ExampleID)
	}

	// Update response assert results
	for i := range workspaceData.ExampleResponseAsserts {
		oldID := workspaceData.ExampleResponseAsserts[i].ID
		workspaceData.ExampleResponseAsserts[i].ID = getNewID(oldID)
		workspaceData.ExampleResponseAsserts[i].AssertID = getNewID(workspaceData.ExampleResponseAsserts[i].AssertID)
		workspaceData.ExampleResponseAsserts[i].ResponseID = getNewID(workspaceData.ExampleResponseAsserts[i].ResponseID)
	}

	// Position the nodes before importing
	fmt.Printf("Starting node positioning for %d nodes, %d edges\n", len(workspaceData.FlowNodes), len(workspaceData.FlowEdges))
	positioner := positioning.NewNodePositioner()
	posErr := positioner.PositionNodes(workspaceData.FlowNodes, workspaceData.FlowEdges, workspaceData.FlowNoopNodes)
	if posErr != nil {
		// Log the error but don't fail the import - positioning is not critical
		fmt.Printf("Warning: Failed to position nodes: %v\n", posErr)
	}
	fmt.Printf("Node positioning completed\n")

	// Create all required services
	ioWorkspace := ioworkspace.NewIOWorkspaceService(
		c.DB,
		c.ws,
		c.cs,
		c.ifs,
		c.ias,
		c.iaes,
		c.ehs,
		c.eqs,
		c.as,
		c.rbs,
		c.fbs,
		c.ubs,
		c.res,
		c.rhs,
		c.ars,
		// Flow services
		c.fs,
		c.ns,
		c.es,
		c.fvs,
		c.frs,
		c.fcs,
		c.fns,
		c.ffors,
		c.ffes,
		c.fjs,
	)

	// Import the workspace data into the existing workspace
	err := ioWorkspace.ImportIntoWorkspace(ctx, *workspaceData)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	return nil
}
