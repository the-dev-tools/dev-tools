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
	"the-dev-tools/server/pkg/model/postman/v21/mpostmancollection"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/flow/sedge"
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
	changev1 "the-dev-tools/spec/dist/buf/go/change/v1"
	collectionv1 "the-dev-tools/spec/dist/buf/go/collection/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
	importv1 "the-dev-tools/spec/dist/buf/go/import/v1"
	"the-dev-tools/spec/dist/buf/go/import/v1/importv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/anypb"
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
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, cs scollection.CollectionService, us suser.UserService,
	ifs sitemfolder.ItemFolderService, ias sitemapi.ItemApiService,
	iaes sitemapiexample.ItemApiExampleService, res sexampleresp.ExampleRespService,
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
	collectionID := idwrap.NewNow()

	// If no filter provided, we need to parse and present filter options
	if len(req.Msg.Filter) == 0 {
		// Handle curl import
		if len(textData) > 0 {
			curlResolved, err := tcurl.ConvertCurl(textData)
			if err != nil {
				return nil, err
			}

			changes, err := c.ImportCurl(ctx, wsUlid, collectionID, "curl", curlResolved)
			if err != nil {
				return nil, err
			}

			resp.Changes = changes
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
	changes, err := c.ImportHar(ctx, wsUlid, collectionID, req.Msg.Name, &lastHar)
	if err == nil {
		resp.Changes = changes
		return connect.NewResponse(resp), nil
	}

	// Try to import as Postman Collection
	postman, err := tpostman.ParsePostmanCollection(data)
	if err != nil {
		return nil, err
	}

	changes, err = c.ImportPostmanCollection(ctx, wsUlid, collectionID, req.Msg.Name, postman)
	if err == nil {
		resp.Changes = changes
		return connect.NewResponse(resp), nil
	}

	return nil, errors.New("invalid file")
}

func (c *ImportRPC) ImportCurl(ctx context.Context, workspaceID, CollectionID idwrap.IDWrap, name string, resolvedCurl tcurl.CurlResolved) ([]*changev1.Change, error) {
	collection := mcollection.Collection{
		ID:      CollectionID,
		Name:    name,
		OwnerID: workspaceID,
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
	err = txCollectionService.CreateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemApiService.CreateItemApiBulk(ctx, resolvedCurl.Apis)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, resolvedCurl.Examples)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// START BODY
	txBodyRawService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyRawService.CreateBulkBodyRaw(ctx, resolvedCurl.RawBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService, err := sbodyform.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyFormService.CreateBulkBodyForm(ctx, resolvedCurl.FormBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService, err := sbodyurl.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, resolvedCurl.UrlEncodedBodies)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// END BODY

	txHeaderService, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txHeaderService.CreateBulkHeader(ctx, resolvedCurl.Headers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	txQueriesService, err := sexamplequery.NewTX(ctx, tx)
	if err != nil {
		return nil, err
	}
	err = txQueriesService.CreateBulkQuery(ctx, resolvedCurl.Queries)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	ws.CollectionCount++
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Changes
	collectionListItem := &collectionv1.CollectionListItem{
		CollectionId: CollectionID.Bytes(),
		Name:         name,
	}

	changeCollectionListResp := collectionv1.CollectionListResponse{
		WorkspaceId: workspaceID.Bytes(),
		Items:       []*collectionv1.CollectionListItem{collectionListItem},
	}

	changeCollectionListRespAny, err := anypb.New(&changeCollectionListResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	listCollectionChanges := []*changev1.ListChange{
		{
			Kind:   changev1.ListChangeKind_LIST_CHANGE_KIND_APPEND,
			Parent: changeCollectionListRespAny,
		},
	}

	collectionChangeAnyData, err := anypb.New(collectionListItem)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changeCollection := &changev1.Change{
		Kind: new(changev1.ChangeKind),
		List: listCollectionChanges,
		Data: collectionChangeAnyData,
	}

	return []*changev1.Change{changeCollection}, nil
}

func (c *ImportRPC) ImportPostmanCollection(ctx context.Context, workspaceID, CollectionID idwrap.IDWrap, name string, collectionData mpostmancollection.Collection) ([]*changev1.Change, error) {
	collection := mcollection.Collection{
		ID:      CollectionID,
		Name:    name,
		OwnerID: workspaceID,
	}

	items, err := tpostman.ConvertPostmanCollection(collectionData, CollectionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	err = txCollectionService.CreateCollection(ctx, &collection)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemFolderService, err := sitemfolder.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemFolderService.CreateItemFolderBulk(ctx, items.Folders)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txItemApiService.CreateItemApiBulk(ctx, items.Apis)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, items.ApiExamples)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// START BODY
	txBodyRawService, err := sbodyraw.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyRawService.CreateBulkBodyRaw(ctx, items.BodyRaw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyFormService, err := sbodyform.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyFormService.CreateBulkBodyForm(ctx, items.BodyForm)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txBodyUrlEncodedService, err := sbodyurl.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txBodyUrlEncodedService.CreateBulkBodyURLEncoded(ctx, items.BodyUrlEncoded)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// END BODY

	txHeaderService, err := sexampleheader.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txHeaderService.CreateBulkHeader(ctx, items.Headers)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	txQueriesService, err := sexamplequery.NewTX(ctx, tx)
	if err != nil {
		return nil, err
	}
	err = txQueriesService.CreateBulkQuery(ctx, items.Queries)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	ws.CollectionCount++
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Changes
	collectionListItem := &collectionv1.CollectionListItem{
		CollectionId: CollectionID.Bytes(),
		Name:         name,
	}

	changeCollectionListResp := collectionv1.CollectionListResponse{
		WorkspaceId: workspaceID.Bytes(),
		Items:       []*collectionv1.CollectionListItem{collectionListItem},
	}

	changeCollectionListRespAny, err := anypb.New(&changeCollectionListResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	listCollectionChanges := []*changev1.ListChange{
		{
			Kind:   changev1.ListChangeKind_LIST_CHANGE_KIND_APPEND,
			Parent: changeCollectionListRespAny,
		},
	}

	collectionChangeAnyData, err := anypb.New(collectionListItem)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changeCollection := &changev1.Change{
		Kind: new(changev1.ChangeKind),
		List: listCollectionChanges,
		Data: collectionChangeAnyData,
	}

	return []*changev1.Change{changeCollection}, nil
}

func (c *ImportRPC) ImportHar(ctx context.Context, workspaceID, CollectionID idwrap.IDWrap, name string, harData *thar.HAR) ([]*changev1.Change, error) {
	resolved, err := thar.ConvertHAR(harData, CollectionID, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if len(resolved.Apis) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("no apis found to create in har"))
	}

	collectionData := mcollection.Collection{
		ID:      CollectionID,
		Name:    name,
		OwnerID: workspaceID,
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
	err = txCollectionService.CreateCollection(ctx, &collectionData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiService, err := sitemapi.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiService.CreateItemApiBulk(ctx, resolved.Apis)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txItemApiExampleService, err := sitemapiexample.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txItemApiExampleService.CreateApiExampleBulk(ctx, resolved.Examples)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	// Flow Creation
	//
	// Flow
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
	txFlowRequestService, err := snoderequest.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = txFlowRequestService.CreateNodeRequestBulk(ctx, resolved.RequestNodes)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

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

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Changes
	collectionListItem := &collectionv1.CollectionListItem{
		CollectionId: CollectionID.Bytes(),
		Name:         name,
	}

	changeCollectionListResp := collectionv1.CollectionListResponse{
		WorkspaceId: workspaceID.Bytes(),
		Items:       []*collectionv1.CollectionListItem{collectionListItem},
	}

	flowListItem := &flowv1.FlowListItem{
		FlowId: resolved.Flow.ID.Bytes(),
		Name:   resolved.Flow.Name,
	}

	changeFlowListResp := &flowv1.FlowListResponse{
		WorkspaceId: workspaceID.Bytes(),
		Items:       []*flowv1.FlowListItem{flowListItem},
	}

	changeFlowListRespAny, err := anypb.New(changeFlowListResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changeCollectionListRespAny, err := anypb.New(&changeCollectionListResp)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	listFlowChanges := []*changev1.ListChange{
		{
			Kind:   changev1.ListChangeKind_LIST_CHANGE_KIND_APPEND,
			Parent: changeFlowListRespAny,
		},
	}

	listCollectionChanges := []*changev1.ListChange{
		{
			Kind:   changev1.ListChangeKind_LIST_CHANGE_KIND_APPEND,
			Parent: changeCollectionListRespAny,
		},
	}

	flowChangeAnyData, err := anypb.New(flowListItem)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	collectionChangeAnyData, err := anypb.New(collectionListItem)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changeFlow := &changev1.Change{
		Kind: new(changev1.ChangeKind),
		List: listFlowChanges,
		Data: flowChangeAnyData,
	}

	changeCollection := &changev1.Change{
		Kind: new(changev1.ChangeKind),
		List: listCollectionChanges,
		Data: collectionChangeAnyData,
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	ws.CollectionCount++
	ws.FlowCount++
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	changes := []*changev1.Change{changeFlow, changeCollection}

	return changes, nil
}
