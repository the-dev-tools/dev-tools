package rimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mitemfolder"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/postman/v21/mpostmancollection"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/sitemfolder"
	"the-dev-tools/server/pkg/service/snode"
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
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, cs scollection.CollectionService, us suser.UserService,
	ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService,
	iaes sitemapiexample.ItemApiExampleService, res sexampleresp.ExampleRespService,
	as sassert.AssertService,
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
	existingCollection, err := c.cs.GetCollectionByWorkspaceIDAndName(ctx, wsUlid, req.Msg.Name)
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
	flow, err := c.ImportHar(ctx, wsUlid, collectionID, req.Msg.Name, &lastHar)
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

	// HAR import failed, falling back to Postman Collection import

	// Try to import as Postman Collection
	postman, err := tpostman.ParsePostmanCollection(data)
	if err != nil {
		// Postman collection parsing also failed
		return nil, err
	}

	// Postman collection parsed successfully, attempting import
	err = c.ImportPostmanCollection(ctx, wsUlid, collectionID, req.Msg.Name, postman)
	if err == nil {
		// Postman collection import successful (no flow created)
		return connect.NewResponse(resp), nil
	}

	// Both HAR and Postman imports failed

	return nil, errors.New("invalid file")
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
		return err
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
		return err
	}
	err = txQueriesService.CreateBulkQuery(ctx, resolvedCurl.Queries)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// Only increment collection count if we created a new collection
	if !collectionExists {
		ws.CollectionCount++
	}
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
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

	err = tx.Commit()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// Only increment collection count if we created a new collection
	if !collectionExists {
		ws.CollectionCount++
	}
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
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
	defer devtoolsdb.TxnRollback(tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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

	// Create folders first (they need to exist before APIs reference them)
	if len(resolved.Folders) > 0 {
		// Filter out folders that already exist
		var foldersToCreate []mitemfolder.ItemFolder
		for i := range resolved.Folders {
			folder := &resolved.Folders[i]
			// Check if folder already exists by name
			exists := false
			for _, existing := range existingFoldersList {
				if existing.Name == folder.Name &&
					((existing.ParentID == nil && folder.ParentID == nil) ||
						(existing.ParentID != nil && folder.ParentID != nil && existing.ParentID.Compare(*folder.ParentID) == 0)) {
					exists = true
					// Update the folder ID in resolved.Folders to use existing ID
					folder.ID = existing.ID
					break
				}
			}
			if !exists {
				foldersToCreate = append(foldersToCreate, *folder)
			}
		}

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

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		// Transaction commit failed
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Transaction committed successfully

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		// Failed to get workspace
		return nil, err
	}
	// Only increment collection count if we created a new collection
	if !collectionExists {
		ws.CollectionCount++
	}
	ws.FlowCount++
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		// Failed to update workspace
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// HAR import completed successfully
	// Return a pointer to the Flow
	flow := resolved.Flow
	return &flow, nil
}
