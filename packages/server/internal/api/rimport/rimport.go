package rimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	yamlflowsimple "the-dev-tools/server/pkg/io/yamlflow/yamlflowsimple"
	"the-dev-tools/server/pkg/model/mcollection"
	"the-dev-tools/server/pkg/model/mexampleheader"
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
	"the-dev-tools/server/pkg/service/scollectionitem"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
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
	"gopkg.in/yaml.v3"
)

// TODO: this is need be switch to id based system later
var lastHar thar.HAR

// Custom error types to distinguish between parsing and database errors
var (
	ErrHARParsing        = errors.New("failed to parse HAR file")
	ErrPostmanParsing    = errors.New("failed to parse Postman collection")
	ErrDatabaseOperation = errors.New("database operation failed")
)

type ImportRPC struct {
	DB                    *sql.DB
	ws                    sworkspace.WorkspaceService
	cs                    scollection.CollectionService
	us                    suser.UserService
	ifs                   sitemfolder.ItemFolderService
	ias                   sitemapi.ItemApiService
	iaes                  sitemapiexample.ItemApiExampleService
	res                   sexampleresp.ExampleRespService
	as                    sassert.AssertService
	cis                   *scollectionitem.CollectionItemService
	bodyRawService        sbodyraw.BodyRawService
	bodyFormService       sbodyform.BodyFormService
	bodyURLEncodedService sbodyurl.BodyURLEncodedService
	headerService         sexampleheader.HeaderService
	queryService          sexamplequery.ExampleQueryService
	flowService           sflow.FlowService
	nodeService           snode.NodeService
	nodeRequestService    snoderequest.NodeRequestService
	nodeNoopService       snodenoop.NodeNoopService
	edgeService           sedge.EdgeService
	flowVariableService   sflowvariable.FlowVariableService
	nodeForService        snodefor.NodeForService
	nodeJSService         snodejs.NodeJSService
	nodeForEachService    snodeforeach.NodeForEachService
	nodeIfService         *snodeif.NodeIfService
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, cs scollection.CollectionService, us suser.UserService,
	ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService,
	iaes sitemapiexample.ItemApiExampleService, res sexampleresp.ExampleRespService,
	as sassert.AssertService, cis *scollectionitem.CollectionItemService,
	bodyRawService sbodyraw.BodyRawService, bodyFormService sbodyform.BodyFormService,
	bodyURLEncodedService sbodyurl.BodyURLEncodedService, headerService sexampleheader.HeaderService,
	queryService sexamplequery.ExampleQueryService, flowService sflow.FlowService,
	nodeService snode.NodeService, nodeRequestService snoderequest.NodeRequestService,
	nodeNoopService snodenoop.NodeNoopService, edgeService sedge.EdgeService,
	flowVariableService sflowvariable.FlowVariableService, nodeForService snodefor.NodeForService,
	nodeJSService snodejs.NodeJSService, nodeForEachService snodeforeach.NodeForEachService,
	nodeIfService *snodeif.NodeIfService,
) ImportRPC {
	return ImportRPC{
		DB:                    db,
		ws:                    ws,
		cs:                    cs,
		us:                    us,
		ifs:                   ifs,
		ias:                   ias,
		iaes:                  iaes,
		res:                   res,
		as:                    as,
		cis:                   cis,
		bodyRawService:        bodyRawService,
		bodyFormService:       bodyFormService,
		bodyURLEncodedService: bodyURLEncodedService,
		headerService:         headerService,
		queryService:          queryService,
		flowService:           flowService,
		nodeService:           nodeService,
		nodeRequestService:    nodeRequestService,
		nodeNoopService:       nodeNoopService,
		edgeService:           edgeService,
		flowVariableService:   flowVariableService,
		nodeForService:        nodeForService,
		nodeJSService:         nodeJSService,
		nodeForEachService:    nodeForEachService,
		nodeIfService:         nodeIfService,
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
	// For other imports (curl with textData), use the provided name or generate default
	collectionName := req.Msg.Name
	// Check if this is a HAR import (either initial parse or filtered import)
	isHARImport := len(textData) == 0 && (json.Valid(data) || len(req.Msg.Filter) > 0)
	if isHARImport {
		// This is a HAR import, use "Imported" as collection name
		collectionName = "Imported"
	} else if len(textData) > 0 {
		// This is a curl import - validate and generate name if needed
		if strings.TrimSpace(collectionName) == "" {
			collectionName = generateCurlCollectionName(textData)
		}
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

			err = c.ImportCurl(ctx, wsUlid, collectionID, collectionName, curlResolved)
			if err != nil {
				return nil, err
			}

			return connect.NewResponse(resp), nil
		}

		// Try to detect simplified YAML format first
		var yamlCheck map[string]any
		if err := yaml.Unmarshal(data, &yamlCheck); err == nil {
			// Check if it has the markers for simplified format
			if _, hasWorkspace := yamlCheck["workspace_name"]; hasWorkspace {
				if _, hasFlows := yamlCheck["flows"]; hasFlows {
					// This appears to be a simplified workflow YAML
					resolvedYAML, err := yamlflowsimple.ConvertSimplifiedYAML(data, collectionID, wsUlid)
					if err != nil {
						return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to convert simplified workflow: %w", err))
					}

					// Import the simplified YAML data
					err = c.ImportSimplifiedYAML(ctx, wsUlid, resolvedYAML)
					if err != nil {
						return nil, err
					}

					// Return the first flow if any
					if len(resolvedYAML.Flows) > 0 {
						flow := resolvedYAML.Flows[0]
						resp.Flow = &flowv1.FlowListItem{
							FlowId: flow.ID.Bytes(),
							Name:   flow.Name,
						}
					}

					return connect.NewResponse(resp), nil
				}
			}
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

	txCollectionService := c.cs.TX(tx)

	// Only create collection if it doesn't exist
	if !collectionExists {
		err = txCollectionService.CreateCollection(ctx, &collection)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// Create endpoints using collection item service for proper ordering
	txCollectionItemService := c.cis.TX(tx)
	for _, api := range resolvedCurl.Apis {
		if err := txCollectionItemService.CreateEndpointTX(ctx, tx, &api); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemApiExampleService := c.iaes.TX(tx)

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, resolvedCurl.Examples)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// START BODY
	txBodyRawService := c.bodyRawService.TX(tx)
	err = txBodyRawService.CreateBulkBodyRaw(ctx, resolvedCurl.RawBodies)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService := c.bodyFormService.TX(tx)
	err = txBodyFormService.CreateBulkBodyForm(ctx, resolvedCurl.FormBodies)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService := c.bodyURLEncodedService.TX(tx)
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, resolvedCurl.UrlEncodedBodies)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// END BODY

	txHeaderService := c.headerService.TX(tx)
	err = txHeaderService.AppendBulkHeader(ctx, resolvedCurl.Headers)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	txQueriesService := c.queryService.TX(tx)
	err = txQueriesService.CreateBulkQuery(ctx, resolvedCurl.Queries)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Update workspace counts and timestamp inside transaction
	txWorkspaceService := c.ws.TX(tx)

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

	txCollectionService := c.cs.TX(tx)

	// Only create collection if it doesn't exist
	if !collectionExists {
		err = txCollectionService.CreateCollection(ctx, &collection)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemFolderService := c.ifs.TX(tx)
	err = txItemFolderService.CreateItemFolderBulk(ctx, items.Folders)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService := c.ias.TX(tx)
	err = txItemApiService.CreateItemApiBulk(ctx, items.Apis)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService := c.iaes.TX(tx)

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, items.ApiExamples)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// START BODY
	txBodyRawService := c.bodyRawService.TX(tx)
	err = txBodyRawService.CreateBulkBodyRaw(ctx, items.BodyRaw)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService := c.bodyFormService.TX(tx)
	err = txBodyFormService.CreateBulkBodyForm(ctx, items.BodyForm)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService := c.bodyURLEncodedService.TX(tx)
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, items.BodyUrlEncoded)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	// END BODY

	txHeaderService := c.headerService.TX(tx)
	err = txHeaderService.AppendBulkHeader(ctx, items.Headers)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	txQueriesService := c.queryService.TX(tx)
	err = txQueriesService.CreateBulkQuery(ctx, items.Queries)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Update workspace counts and timestamp inside transaction
	txWorkspaceService := c.ws.TX(tx)

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

	txCollectionService := c.cs.TX(tx)

	// Only create collection if it doesn't exist
	if !collectionExists {
		err = txCollectionService.CreateCollection(ctx, &collectionData)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemApiService := c.ias.TX(tx)

	txItemFolderService := c.ifs.TX(tx)

	txCollectionItemService := c.cis.TX(tx)

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

	folderToCollectionItemMapping := make(map[idwrap.IDWrap]idwrap.IDWrap)

	if len(existingFoldersList) > 0 {
		foldersProcessed := make(map[idwrap.IDWrap]bool)

		for _, folder := range existingFoldersList {
			ciID, mapErr := txCollectionItemService.GetCollectionItemIDByLegacyID(ctx, folder.ID)
			if mapErr == nil {
				folderToCollectionItemMapping[folder.ID] = ciID
				foldersProcessed[folder.ID] = true
				continue
			}
			if !errors.Is(mapErr, scollectionitem.ErrCollectionItemNotFound) {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to map folder %s: %w", folder.ID.String(), mapErr))
			}
		}

		for len(foldersProcessed) < len(existingFoldersList) {
			progressMade := false
			for _, folder := range existingFoldersList {
				if foldersProcessed[folder.ID] {
					continue
				}

				parentReady := folder.ParentID == nil
				if folder.ParentID != nil {
					if _, ok := folderToCollectionItemMapping[*folder.ParentID]; ok {
						parentReady = true
					}
				}

				if !parentReady {
					continue
				}

				if err := txCollectionItemService.CreateFolderTX(ctx, tx, &folder); err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to backfill folder %s: %w", folder.ID.String(), err))
				}

				ciID, mapErr := txCollectionItemService.GetCollectionItemIDByLegacyID(ctx, folder.ID)
				if mapErr != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to map folder %s after creation: %w", folder.ID.String(), mapErr))
				}

				folderToCollectionItemMapping[folder.ID] = ciID
				foldersProcessed[folder.ID] = true
				progressMade = true
			}

			if !progressMade {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("circular dependency detected in folder hierarchy or orphaned folders"))
			}
		}
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
	missingEndpointItems := make(map[idwrap.IDWrap]mitemapi.ItemApi)

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

			if _, alreadyScheduled := missingEndpointItems[existingApi.ID]; !alreadyScheduled {
				if _, err := txCollectionItemService.GetCollectionItemIDByLegacyID(ctx, existingApi.ID); err != nil {
					if errors.Is(err, scollectionitem.ErrCollectionItemNotFound) {
						missingEndpointItems[existingApi.ID] = *existingApi
					} else {
						return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check collection item for endpoint %s: %w", existingApi.ID.String(), err))
					}
				}
			}
		} else {
			// New endpoint - create it
			apiMapping[api.ID] = api.ID
			apisToCreate = append(apisToCreate, api)
		}
	}

	// For HAR imports, delta endpoints should not be created as separate APIs
	// They are meant to be variations/modifications of original endpoints in the flow system
	// Just map them to their original APIs for example linking purposes
	for _, api := range resolved.Apis {
		if api.DeltaParentID != nil {
			// Don't create delta endpoints as separate APIs in HAR import
			// Instead, find the corresponding original API and map to it
			if originalID, exists := apiMapping[*api.DeltaParentID]; exists {
				apiMapping[api.ID] = originalID
			}
			// Skip creating delta APIs as separate database entities
		}
	}

	for _, api := range apisToCreate {
		if err := txCollectionItemService.CreateEndpointTX(ctx, tx, &api); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create endpoint %s: %w", api.ID.String(), err))
		}
	}

	for _, api := range missingEndpointItems {
		if err := txCollectionItemService.CreateEndpointTX(ctx, tx, &api); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to recreate collection item for API %s: %w", api.ID.String(), err))
		}
	}

	// Update existing endpoints
	for _, api := range apisToUpdate {
		err = txItemApiService.UpdateItemApi(ctx, &api)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	txItemApiExampleService := c.iaes.TX(tx)

	// Update example API IDs based on mapping
	var updatedExamples []mitemapiexample.ItemApiExample
	for _, example := range resolved.Examples {
		if mappedID, ok := apiMapping[example.ItemApiID]; ok {
			example.ItemApiID = mappedID
			updatedExamples = append(updatedExamples, example)
		}
		// Skip examples that don't have a corresponding API in this collection
		// This can happen when filtering by domain in HAR import
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

	txExampleHeaderService := c.headerService.TX(tx)

	// Separate headers into base headers and delta headers to avoid FK constraint violations
	// Base headers must be created before delta headers that reference them
	var baseHeaders []mexampleheader.Header
	var deltaHeaders []mexampleheader.Header

	for _, header := range resolved.Headers {
		if header.DeltaParentID == nil {
			baseHeaders = append(baseHeaders, header)
		} else {
			deltaHeaders = append(deltaHeaders, header)
		}
	}

	// Create base headers first
	if len(baseHeaders) > 0 {
		err = txExampleHeaderService.AppendBulkHeader(ctx, baseHeaders)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Create delta headers second (they can now reference existing base headers)
	if len(deltaHeaders) > 0 {
		err = txExampleHeaderService.AppendBulkHeader(ctx, deltaHeaders)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	txExampleQueryService := c.queryService.TX(tx)
	err = txExampleQueryService.CreateBulkQuery(ctx, resolved.Queries)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyRawService := c.bodyRawService.TX(tx)

	err = txBodyRawService.CreateBulkBodyRaw(ctx, resolved.RawBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService := c.bodyFormService.TX(tx)
	err = txBodyFormService.CreateBulkBodyForm(ctx, resolved.FormBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService := c.bodyURLEncodedService.TX(tx)
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, resolved.UrlEncodedBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Assertions
	if len(resolved.Asserts) > 0 {
		txAssertService := c.as.TX(tx)
		err = txAssertService.CreateBulkAssert(ctx, resolved.Asserts)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Flow Creation
	txFlowService := c.flowService.TX(tx)
	err = txFlowService.CreateFlow(ctx, resolved.Flow)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Flow Node
	txFlowNodeService := c.nodeService.TX(tx)
	err = txFlowNodeService.CreateNodeBulk(ctx, resolved.Nodes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Flow Request Nodes
	// Create flow request nodes
	txFlowRequestService := c.nodeRequestService.TX(tx)

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
	txFlowNoopService := c.nodeNoopService.TX(tx)
	err = txFlowNoopService.CreateNodeNoopBulk(ctx, resolved.NoopNodes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Edges
	txFlowEdgeService := c.edgeService.TX(tx)
	err = txFlowEdgeService.CreateEdgeBulk(ctx, resolved.Edges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update workspace counts and timestamp inside transaction
	txWorkspaceService := c.ws.TX(tx)

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

func (c *ImportRPC) ImportSimplifiedYAML(ctx context.Context, workspaceID idwrap.IDWrap, resolved yamlflowsimple.SimplifiedYAMLResolved) error {
	tx, err := c.DB.Begin()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	// Import collections
	if len(resolved.Collections) > 0 {
		txCollectionService := c.cs.TX(tx)
		for _, collection := range resolved.Collections {
			collection.WorkspaceID = workspaceID
			err = txCollectionService.CreateCollection(ctx, &collection)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// Import endpoints
	if len(resolved.Endpoints) > 0 {
		txEndpointService := c.ias.TX(tx)
		err = txEndpointService.CreateItemApiBulk(ctx, resolved.Endpoints)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// Import examples
	if len(resolved.Examples) > 0 {
		txExampleService := c.iaes.TX(tx)
		err = txExampleService.CreateApiExampleBulk(ctx, resolved.Examples)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// Import headers
	if len(resolved.Headers) > 0 {
		txHeaderService := c.headerService.TX(tx)
		err = txHeaderService.AppendBulkHeader(ctx, resolved.Headers)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// Import queries
	if len(resolved.Queries) > 0 {
		txQueryService := c.queryService.TX(tx)
		err = txQueryService.CreateBulkQuery(ctx, resolved.Queries)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// Import raw bodies
	if len(resolved.RawBodies) > 0 {
		txBodyService := c.bodyRawService.TX(tx)
		err = txBodyService.CreateBulkBodyRaw(ctx, resolved.RawBodies)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// Import flows
	if len(resolved.Flows) > 0 {
		txFlowService := c.flowService.TX(tx)
		for i := range resolved.Flows {
			resolved.Flows[i].WorkspaceID = workspaceID
			err = txFlowService.CreateFlow(ctx, resolved.Flows[i])
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// Import flow nodes
	if len(resolved.FlowNodes) > 0 {
		txNodeService := c.nodeService.TX(tx)
		err = txNodeService.CreateNodeBulk(ctx, resolved.FlowNodes)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	// Import flow edges
	if len(resolved.FlowEdges) > 0 {
		txEdgeService := c.edgeService.TX(tx)
		for _, edge := range resolved.FlowEdges {
			err = txEdgeService.CreateEdge(ctx, edge)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// Import flow variables
	if len(resolved.FlowVariables) > 0 {
		txFlowVariableService := c.flowVariableService.TX(tx)
		for _, v := range resolved.FlowVariables {
			err = txFlowVariableService.CreateFlowVariable(ctx, v)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// Import node implementations
	if len(resolved.FlowRequestNodes) > 0 {
		txRequestService := c.nodeRequestService.TX(tx)
		for _, r := range resolved.FlowRequestNodes {
			err = txRequestService.CreateNodeRequest(ctx, r)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if len(resolved.FlowConditionNodes) > 0 {
		txConditionService := c.nodeIfService.TX(tx)
		for _, cnode := range resolved.FlowConditionNodes {
			err = txConditionService.CreateNodeIf(ctx, cnode)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if len(resolved.FlowNoopNodes) > 0 {
		txNoopService := c.nodeNoopService.TX(tx)
		for _, n := range resolved.FlowNoopNodes {
			err = txNoopService.CreateNodeNoop(ctx, n)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if len(resolved.FlowForNodes) > 0 {
		txForService := c.nodeForService.TX(tx)
		for _, f := range resolved.FlowForNodes {
			err = txForService.CreateNodeFor(ctx, f)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if len(resolved.FlowJSNodes) > 0 {
		txJsService := c.nodeJSService.TX(tx)
		for _, j := range resolved.FlowJSNodes {
			err = txJsService.CreateNodeJS(ctx, j)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	if len(resolved.FlowForEachNodes) > 0 {
		txForEachService := c.nodeForEachService.TX(tx)
		for _, fe := range resolved.FlowForEachNodes {
			err = txForEachService.CreateNodeForEach(ctx, fe)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// Update workspace counts and timestamp
	txWorkspaceService := c.ws.TX(tx)

	ws, err := txWorkspaceService.Get(ctx, workspaceID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if len(resolved.Collections) > 0 {
		ws.CollectionCount += int32(len(resolved.Collections))
	}
	if len(resolved.Flows) > 0 {
		ws.FlowCount += int32(len(resolved.Flows))
	}
	ws.Updated = dbtime.DBNow()

	err = txWorkspaceService.Update(ctx, ws)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	return nil
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

// generateCurlCollectionName generates a meaningful collection name from a curl command
// by extracting the hostname from the URL, providing a fallback if extraction fails
func generateCurlCollectionName(curlStr string) string {
	// Use the tcurl package to extract the URL from the curl command
	extractedURL := tcurl.ExtractURLForTesting(curlStr)

	if extractedURL == "" {
		// Fallback to default name if URL extraction fails
		return "Imported from cURL"
	}

	var hostname string

	// First try to parse the URL as-is (works for URLs with protocols)
	if parsedURL, err := url.Parse(extractedURL); err == nil && parsedURL.Host != "" {
		hostname = parsedURL.Host
	} else {
		// For protocol-less URLs like "google.com", url.Parse treats them as paths
		// Try prepending a protocol and parsing again
		prefixedURL := "https://" + extractedURL
		if parsedURL, err := url.Parse(prefixedURL); err == nil && parsedURL.Host != "" {
			hostname = parsedURL.Host
		} else {
			// If both parsing attempts fail, try to extract hostname manually
			// Remove any path components (everything after first '/')
			if slashIndex := strings.Index(extractedURL, "/"); slashIndex != -1 {
				hostname = extractedURL[:slashIndex]
			} else {
				hostname = extractedURL
			}
		}
	}

	if hostname == "" {
		// Fallback if no hostname found
		return "Imported from cURL"
	}

	// Remove port if present and clean up hostname
	if colonIndex := strings.Index(hostname, ":"); colonIndex != -1 {
		hostname = hostname[:colonIndex]
	}

	// Remove www prefix if present
	if strings.HasPrefix(hostname, "www.") {
		hostname = hostname[4:]
	}

	// Create a user-friendly collection name
	if hostname != "" {
		// Capitalize first letter of each word manually to avoid deprecated strings.Title
		words := strings.Split(hostname, ".")
		for i, word := range words {
			if len(word) > 0 {
				words[i] = strings.ToUpper(word[:1]) + word[1:]
			}
		}
		return fmt.Sprintf("%s API", strings.Join(words, "."))
	}

	// Final fallback
	return "Imported from cURL"
}
