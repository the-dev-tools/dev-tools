package rrequest

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/tassert"
	"the-dev-tools/server/pkg/translate/tcondition"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"the-dev-tools/server/pkg/translate/theader"
	"the-dev-tools/server/pkg/translate/tquery"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/request/v1/requestv1connect"

	"connectrpc.com/connect"
)

type RequestRPC struct {
	DB   *sql.DB
	cs   scollection.CollectionService
	us   suser.UserService
	iaes sitemapiexample.ItemApiExampleService

	// Sub
	ehs sexampleheader.HeaderService
	eqs sexamplequery.ExampleQueryService

	// Assert
	as sassert.AssertService
}

func New(db *sql.DB, cs scollection.CollectionService, us suser.UserService, iaes sitemapiexample.ItemApiExampleService,
	ehs sexampleheader.HeaderService, eqs sexamplequery.ExampleQueryService, as sassert.AssertService,
) RequestRPC {
	return RequestRPC{
		DB:   db,
		cs:   cs,
		us:   us,
		iaes: iaes,
		ehs:  ehs,
		eqs:  eqs,
		as:   as,
	}
}

func CreateService(srv RequestRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := requestv1connect.NewRequestServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func CheckOwnerHeader(ctx context.Context, hs sexampleheader.HeaderService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, headerUlid idwrap.IDWrap) (bool, error) {
	header, err := hs.GetHeaderByID(ctx, headerUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, header.ExampleID)
}

func CheckOwnerQuery(ctx context.Context, qs sexamplequery.ExampleQueryService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, queryUlid idwrap.IDWrap) (bool, error) {
	query, err := qs.GetExampleQuery(ctx, queryUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, query.ExampleID)
}

func CheckOwnerAssert(ctx context.Context, as sassert.AssertService, iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService, assertUlid idwrap.IDWrap) (bool, error) {
	assert, err := as.GetAssert(ctx, assertUlid)
	if err != nil {
		return false, err
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, assert.ExampleID)
}

func (c RequestRPC) QueryList(ctx context.Context, req *connect.Request[requestv1.QueryListRequest]) (*connect.Response[requestv1.QueryListResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(
		ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	allQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Filter to only include origin queries (source = 1)
	var originQueries []mexamplequery.Query
	for _, query := range allQueries {
		if query.Source == mexamplequery.QuerySourceOrigin {
			originQueries = append(originQueries, query)
		}
	}

	rpcQueries := tgeneric.MassConvert(originQueries, tquery.SerializeQueryModelToRPCItem)
	resp := &requestv1.QueryListResponse{
		ExampleId: exID.Bytes(),
		Items:     rpcQueries,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) QueryCreate(ctx context.Context, req *connect.Request[requestv1.QueryCreateRequest]) (*connect.Response[requestv1.QueryCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(
		ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	reqQuery := requestv1.Query{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	query, err := tquery.SerlializeQueryRPCtoModelNoID(&reqQuery, exID)
	if err != nil {
		return nil, err
	}
	queryID := idwrap.NewNow()
	query.ID = queryID

	// Set source as origin for regular query creation
	query.Source = mexamplequery.QuerySourceOrigin

	err = c.eqs.CreateExampleQuery(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryCreateResponse{QueryId: queryID.Bytes()}), nil
}

func (c RequestRPC) QueryUpdate(ctx context.Context, req *connect.Request[requestv1.QueryUpdateRequest]) (*connect.Response[requestv1.QueryUpdateResponse], error) {
	reqQuery := requestv1.Query{
		QueryId:     req.Msg.GetQueryId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	query, err := tquery.SerlializeQueryRPCtoModel(&reqQuery, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerQuery(ctx, c.eqs, c.iaes, c.cs, c.us, query.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Update the origin query
	query.Source = mexamplequery.QuerySourceOrigin
	err = c.eqs.UpdateExampleQuery(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Propagate changes to delta items with "origin" source that reference this query
	// We need to find all queries in all examples that have this query as DeltaParentID and source="origin"
	// For now, we'll implement a simple approach by getting the example and finding related queries
	originQuery, err := c.eqs.GetExampleQuery(ctx, query.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get all queries from this example to find any that reference this origin query
	allQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, originQuery.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update any delta queries that reference this origin query with source="origin"
	for _, deltaQuery := range allQueries {
		if deltaQuery.DeltaParentID != nil &&
			deltaQuery.DeltaParentID.Compare(query.ID) == 0 &&
			deltaQuery.Source == mexamplequery.QuerySourceOrigin {
			// Update the delta query to match the origin
			deltaQuery.QueryKey = query.QueryKey
			deltaQuery.Enable = query.Enable
			deltaQuery.Description = query.Description
			deltaQuery.Value = query.Value

			err = c.eqs.UpdateExampleQuery(ctx, deltaQuery)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	return connect.NewResponse(&requestv1.QueryUpdateResponse{}), nil
}

func (c RequestRPC) QueryDelete(ctx context.Context, req *connect.Request[requestv1.QueryDeleteRequest]) (*connect.Response[requestv1.QueryDeleteResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerQuery(ctx, c.eqs, c.iaes, c.cs, c.us, queryID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the query to check if it's an origin query and get its example ID
	originQuery, err := c.eqs.GetExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If this is an origin query, delete all delta items with "origin" or "mixed" source that reference it
	if originQuery.Source == mexamplequery.QuerySourceOrigin {
		// Get all queries from this example to find any that reference this origin query
		allQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, originQuery.ExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Delete any delta queries that reference this origin query with source="origin" or "mixed"
		for _, deltaQuery := range allQueries {
			if deltaQuery.DeltaParentID != nil &&
				deltaQuery.DeltaParentID.Compare(queryID) == 0 &&
				(deltaQuery.Source == mexamplequery.QuerySourceOrigin || deltaQuery.Source == mexamplequery.QuerySourceMixed) {
				err = c.eqs.DeleteExampleQuery(ctx, deltaQuery.ID)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	// Delete the origin query itself
	err = c.eqs.DeleteExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.QueryDeleteResponse{}), nil
}

// QueryDeltaExampleCopy copies all queries from an origin example to a delta example
// This implements the "Delta example create" functionality
func (c RequestRPC) QueryDeltaExampleCopy(ctx context.Context, originExampleID, deltaExampleID idwrap.IDWrap) error {
	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return rpcErr
	}

	// Get all queries from the origin example
	originQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, originExampleID)
	if err != nil {
		return err
	}

	// Create corresponding queries in the delta example
	var deltaQueries []mexamplequery.Query
	for _, originQuery := range originQueries {
		// Only copy origin queries (not mixed or delta queries)
		if originQuery.Source == mexamplequery.QuerySourceOrigin {
			deltaQuery := mexamplequery.Query{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				DeltaParentID: &originQuery.ID, // Reference the origin query
				QueryKey:      originQuery.QueryKey,
				Enable:        originQuery.Enable,
				Description:   originQuery.Description,
				Value:         originQuery.Value,
				Source:        mexamplequery.QuerySourceOrigin, // Mark as "origin" in delta context
			}
			deltaQueries = append(deltaQueries, deltaQuery)
		}
	}

	// Bulk create all delta queries
	if len(deltaQueries) > 0 {
		err = c.eqs.CreateBulkQuery(ctx, deltaQueries)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c RequestRPC) QueryDeltaDelete(ctx context.Context, req *connect.Request[requestv1.QueryDeltaDeleteRequest]) (*connect.Response[requestv1.QueryDeltaDeleteResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerQuery(ctx, c.eqs, c.iaes, c.cs, c.us, queryID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.eqs.DeleteExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryDeltaDeleteResponse{}), nil
}

func (c RequestRPC) QueryDeltaList(ctx context.Context, req *connect.Request[requestv1.QueryDeltaListRequest]) (*connect.Response[requestv1.QueryDeltaListResponse], error) {
	// Parse both example IDs
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get queries from both origin and delta examples
	originQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	deltaQueries, err := c.eqs.GetExampleQueriesByExampleID(ctx, deltaExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Combine all queries and build maps for lookup
	allQueries := append(originQueries, deltaQueries...)
	queryMap := make(map[idwrap.IDWrap]mexamplequery.Query)
	originMap := make(map[idwrap.IDWrap]*requestv1.Query)
	replacedOrigins := make(map[idwrap.IDWrap]bool)

	for _, query := range allQueries {
		queryMap[query.ID] = query
		originMap[query.ID] = tquery.SerializeQueryModelToRPC(query)

		// Mark origin queries that have been replaced by mixed queries
		if query.Source == mexamplequery.QuerySourceMixed && query.DeltaParentID != nil {
			replacedOrigins[*query.DeltaParentID] = true
		}
	}

	// Create result slice maintaining order from allQueries
	var rpcQueries []*requestv1.QueryDeltaListItem
	for _, query := range allQueries {
		// Skip origin queries that have been replaced by mixed queries
		if query.Source == mexamplequery.QuerySourceOrigin && replacedOrigins[query.ID] {
			continue
		}

		sourceKind := query.Source.ToSourceKind()
		var origin *requestv1.Query
		var key, value, description string
		var enabled bool

		if query.Source == mexamplequery.QuerySourceOrigin {
			// For origin items, put the data in origin field and leave main fields empty
			origin = tquery.SerializeQueryModelToRPC(query)
			key = ""
			value = ""
			description = ""
			enabled = false
		} else {
			// For delta/mixed items, use the current values and find the origin if it has a parent
			key = query.QueryKey
			value = query.Value
			description = query.Description
			enabled = query.Enable

			if query.DeltaParentID != nil {
				if originRPC, exists := originMap[*query.DeltaParentID]; exists {
					origin = originRPC
				}
			}
		}

		rpcQuery := &requestv1.QueryDeltaListItem{
			QueryId:     query.ID.Bytes(),
			Key:         key,
			Enabled:     enabled,
			Value:       value,
			Description: description,
			Origin:      origin,
			Source:      &sourceKind,
		}
		rpcQueries = append(rpcQueries, rpcQuery)
	}

	// Sort rpcQueries by ID, but if it has DeltaParentID use that ID instead
	sort.Slice(rpcQueries, func(i, j int) bool {
		idI, _ := idwrap.NewFromBytes(rpcQueries[i].QueryId)
		idJ, _ := idwrap.NewFromBytes(rpcQueries[j].QueryId)

		// Determine the ID to use for sorting for item i
		sortIDI := idI
		if rpcQueries[i].Origin != nil && len(rpcQueries[i].Origin.QueryId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcQueries[i].Origin.QueryId); err == nil {
				sortIDI = parentID
			}
		}

		// Determine the ID to use for sorting for item j
		sortIDJ := idJ
		if rpcQueries[j].Origin != nil && len(rpcQueries[j].Origin.QueryId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcQueries[j].Origin.QueryId); err == nil {
				sortIDJ = parentID
			}
		}

		return sortIDI.Compare(sortIDJ) < 0
	})

	resp := &requestv1.QueryDeltaListResponse{
		ExampleId: deltaExampleID.Bytes(),
		Items:     rpcQueries,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) QueryDeltaCreate(ctx context.Context, req *connect.Request[requestv1.QueryDeltaCreateRequest]) (*connect.Response[requestv1.QueryDeltaCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	reqQuery := requestv1.Query{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	query, err := tquery.SerlializeQueryRPCtoModelNoIDForDelta(&reqQuery, exID)
	if err != nil {
		return nil, err
	}

	queryID := idwrap.NewNow()
	query.ID = queryID

	// Always set source as delta since this is QueryDeltaCreate
	query.Source = mexamplequery.QuerySourceDelta

	// Check if query_id is provided in request
	if len(req.Msg.GetQueryId()) > 0 {
		// Query ID is provided, verify it exists and use as delta parent
		parentQueryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the parent query exists and belongs to the same example
		parentQuery, err := c.eqs.GetExampleQuery(ctx, parentQueryID)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		// Verify parent query belongs to the same example
		if parentQuery.ExampleID.Compare(exID) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("parent query does not belong to the specified example"))
		}

		query.DeltaParentID = &parentQueryID
	}
	// If no query_id provided, DeltaParentID remains nil (standalone delta)

	err = c.eqs.CreateExampleQuery(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.QueryDeltaCreateResponse{QueryId: queryID.Bytes()}), nil
}

func (c RequestRPC) QueryDeltaUpdate(ctx context.Context, req *connect.Request[requestv1.QueryDeltaUpdateRequest]) (*connect.Response[requestv1.QueryDeltaUpdateResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerQuery(ctx, c.eqs, c.iaes, c.cs, c.us, queryID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the existing query to check its source
	existingQuery, err := c.eqs.GetExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If this is an origin query, we need to create a mixed query instead of updating
	if existingQuery.Source == mexamplequery.QuerySourceOrigin {
		// Create a new mixed query with updated fields
		mixedQuery := mexamplequery.Query{
			ID:            idwrap.NewNow(),
			ExampleID:     existingQuery.ExampleID,
			DeltaParentID: &existingQuery.ID, // Point to the original query
			QueryKey:      req.Msg.GetKey(),
			Enable:        req.Msg.GetEnabled(),
			Description:   req.Msg.GetDescription(),
			Value:         req.Msg.GetValue(),
			Source:        mexamplequery.QuerySourceMixed, // Set as mixed
		}

		err = c.eqs.CreateExampleQuery(ctx, mixedQuery)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		// If it's already a delta or mixed query, just update it normally
		reqQuery := requestv1.Query{
			QueryId:     req.Msg.GetQueryId(),
			Key:         req.Msg.GetKey(),
			Enabled:     req.Msg.GetEnabled(),
			Value:       req.Msg.GetValue(),
			Description: req.Msg.GetDescription(),
		}
		query, err := tquery.SerlializeQueryRPCtoModel(&reqQuery, idwrap.IDWrap{})
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Preserve the existing source type
		query.Source = existingQuery.Source
		query.DeltaParentID = existingQuery.DeltaParentID

		err = c.eqs.UpdateExampleQuery(ctx, query)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&requestv1.QueryDeltaUpdateResponse{}), nil
}

func (c RequestRPC) QueryDeltaReset(ctx context.Context, req *connect.Request[requestv1.QueryDeltaResetRequest]) (*connect.Response[requestv1.QueryDeltaResetResponse], error) {
	queryID, err := idwrap.NewFromBytes(req.Msg.GetQueryId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerQuery(ctx, c.eqs, c.iaes, c.cs, c.us, queryID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the delta query
	deltaQuery, err := c.eqs.GetExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If this is a mixed query with a parent, delete the mixed query to revert to origin
	if deltaQuery.Source == mexamplequery.QuerySourceMixed && deltaQuery.DeltaParentID != nil {
		err = c.eqs.DeleteExampleQuery(ctx, queryID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else if deltaQuery.DeltaParentID != nil {
		// If it's a delta query with a parent, restore values from parent and set source to origin
		parentQuery, err := c.eqs.GetExampleQuery(ctx, *deltaQuery.DeltaParentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Restore delta query fields to match parent and set source to origin
		deltaQuery.QueryKey = parentQuery.QueryKey
		deltaQuery.Enable = parentQuery.Enable
		deltaQuery.Description = parentQuery.Description
		deltaQuery.Value = parentQuery.Value

		err = c.eqs.UpdateExampleQuery(ctx, deltaQuery)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		// If no parent, use the original reset behavior (clear fields)
		err = c.eqs.ResetExampleQueryDelta(ctx, queryID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&requestv1.QueryDeltaResetResponse{}), nil
}

func (c RequestRPC) HeaderList(ctx context.Context, req *connect.Request[requestv1.HeaderListRequest]) (*connect.Response[requestv1.HeaderListResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	allHeaders, err := c.ehs.GetHeaderByExampleID(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Filter to only include origin headers (source = 1)
	var originHeaders []mexampleheader.Header
	for _, header := range allHeaders {
		if header.Source == mexampleheader.HeaderSourceOrigin {
			originHeaders = append(originHeaders, header)
		}
	}

	rpcHeaders := tgeneric.MassConvert(originHeaders, theader.SerializeHeaderModelToRPCItem)
	resp := &requestv1.HeaderListResponse{
		ExampleId: exID.Bytes(),
		Items:     rpcHeaders,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) HeaderCreate(ctx context.Context, req *connect.Request[requestv1.HeaderCreateRequest]) (*connect.Response[requestv1.HeaderCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcHeader := requestv1.Header{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	headerID := idwrap.NewNow()
	var deltaParentIDPtr *idwrap.IDWrap
	header := theader.SerlializeHeaderRPCtoModelNoID(&rpcHeader, exID, deltaParentIDPtr)
	header.ID = headerID

	// Set source as origin for regular header creation
	header.Source = mexampleheader.HeaderSourceOrigin

	err = c.ehs.CreateHeader(ctx, header)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.HeaderCreateResponse{HeaderId: headerID.Bytes()}), nil
}

func (c RequestRPC) HeaderUpdate(ctx context.Context, req *connect.Request[requestv1.HeaderUpdateRequest]) (*connect.Response[requestv1.HeaderUpdateResponse], error) {
	rpcHeader := requestv1.Header{
		HeaderId:    req.Msg.GetHeaderId(),
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	header, err := theader.SerlializeHeaderRPCtoModel(&rpcHeader, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerHeader(ctx, c.ehs, c.iaes, c.cs, c.us, header.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Update the origin header
	header.Source = mexampleheader.HeaderSourceOrigin
	err = c.ehs.UpdateHeader(ctx, header)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Propagate changes to delta items with "origin" source that reference this header
	originHeader, err := c.ehs.GetHeaderByID(ctx, header.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get all headers from this example to find any that reference this origin header
	allHeaders, err := c.ehs.GetHeaderByExampleID(ctx, originHeader.ExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update any delta headers that reference this origin header with source="origin"
	for _, deltaHeader := range allHeaders {
		if deltaHeader.DeltaParentID != nil &&
			deltaHeader.DeltaParentID.Compare(header.ID) == 0 &&
			deltaHeader.Source == mexampleheader.HeaderSourceOrigin {
			// Update the delta header to match the origin
			deltaHeader.HeaderKey = header.HeaderKey
			deltaHeader.Enable = header.Enable
			deltaHeader.Description = header.Description
			deltaHeader.Value = header.Value

			err = c.ehs.UpdateHeader(ctx, deltaHeader)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	return connect.NewResponse(&requestv1.HeaderUpdateResponse{}), nil
}

func (c RequestRPC) HeaderDelete(ctx context.Context, req *connect.Request[requestv1.HeaderDeleteRequest]) (*connect.Response[requestv1.HeaderDeleteResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerHeader(ctx, c.ehs, c.iaes, c.cs, c.us, headerID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the header to check if it's an origin header and get its example ID
	originHeader, err := c.ehs.GetHeaderByID(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If this is an origin header, delete all delta items with "origin" or "mixed" source that reference it
	if originHeader.Source == mexampleheader.HeaderSourceOrigin {
		// Get all headers from this example to find any that reference this origin header
		allHeaders, err := c.ehs.GetHeaderByExampleID(ctx, originHeader.ExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Delete any delta headers that reference this origin header with source="origin" or "mixed"
		for _, deltaHeader := range allHeaders {
			if deltaHeader.DeltaParentID != nil &&
				deltaHeader.DeltaParentID.Compare(headerID) == 0 &&
				(deltaHeader.Source == mexampleheader.HeaderSourceOrigin || deltaHeader.Source == mexampleheader.HeaderSourceMixed) {
				err = c.ehs.DeleteHeader(ctx, deltaHeader.ID)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	// Delete the origin header itself
	err = c.ehs.DeleteHeader(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeleteResponse{}), nil
}

// HeaderDeltaExampleCopy copies all headers from an origin example to a delta example
// This implements the "Delta example create" functionality
func (c RequestRPC) HeaderDeltaExampleCopy(ctx context.Context, originExampleID, deltaExampleID idwrap.IDWrap) error {
	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return rpcErr
	}

	// Get all headers from the origin example
	originHeaders, err := c.ehs.GetHeaderByExampleID(ctx, originExampleID)
	if err != nil {
		return err
	}

	// Create corresponding headers in the delta example
	var deltaHeaders []mexampleheader.Header
	for _, originHeader := range originHeaders {
		// Only copy origin headers (not mixed or delta headers)
		if originHeader.Source == mexampleheader.HeaderSourceOrigin {
			deltaHeader := mexampleheader.Header{
				ID:            idwrap.NewNow(),
				ExampleID:     deltaExampleID,
				DeltaParentID: &originHeader.ID, // Reference the origin header
				HeaderKey:     originHeader.HeaderKey,
				Enable:        originHeader.Enable,
				Description:   originHeader.Description,
				Value:         originHeader.Value,
				Source:        mexampleheader.HeaderSourceOrigin, // Mark as "origin" in delta context
			}
			deltaHeaders = append(deltaHeaders, deltaHeader)
		}
	}

	// Bulk create all delta headers
	if len(deltaHeaders) > 0 {
		err = c.ehs.CreateBulkHeader(ctx, deltaHeaders)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c RequestRPC) HeaderDeltaList(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaListRequest]) (*connect.Response[requestv1.HeaderDeltaListResponse], error) {
	// Parse both example IDs
	deltaExampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	originExampleID, err := idwrap.NewFromBytes(req.Msg.GetOriginId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions for both examples
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, deltaExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcErr = permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, originExampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get headers from both origin and delta examples
	originHeaders, err := c.ehs.GetHeaderByExampleID(ctx, originExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	deltaHeaders, err := c.ehs.GetHeaderByExampleID(ctx, deltaExampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Combine all headers and build maps for lookup
	allHeaders := append(originHeaders, deltaHeaders...)
	headerMap := make(map[idwrap.IDWrap]mexampleheader.Header)
	originMap := make(map[idwrap.IDWrap]*requestv1.Header)
	replacedOrigins := make(map[idwrap.IDWrap]bool)

	for _, header := range allHeaders {
		headerMap[header.ID] = header
		originMap[header.ID] = theader.SerializeHeaderModelToRPC(header)

		// Mark origin headers that have been replaced by mixed headers
		if header.Source == mexampleheader.HeaderSourceMixed && header.DeltaParentID != nil {
			replacedOrigins[*header.DeltaParentID] = true
		}
	}

	// Create result slice maintaining order from allHeaders
	var rpcHeaders []*requestv1.HeaderDeltaListItem
	for _, header := range allHeaders {
		// Skip origin headers that have been replaced by mixed headers
		if header.Source == mexampleheader.HeaderSourceOrigin && replacedOrigins[header.ID] {
			continue
		}

		sourceKind := header.Source.ToSourceKind()
		var origin *requestv1.Header
		var key, value, description string
		var enabled bool

		if header.Source == mexampleheader.HeaderSourceOrigin {
			// For origin items, put the data in origin field and leave main fields empty
			origin = theader.SerializeHeaderModelToRPC(header)
			key = ""
			value = ""
			description = ""
			enabled = false
		} else {
			// For delta/mixed items, use the current values and find the origin if it has a parent
			key = header.HeaderKey
			value = header.Value
			description = header.Description
			enabled = header.Enable

			if header.DeltaParentID != nil {
				if originRPC, exists := originMap[*header.DeltaParentID]; exists {
					origin = originRPC
				}
			}
		}

		rpcHeader := &requestv1.HeaderDeltaListItem{
			HeaderId:    header.ID.Bytes(),
			Key:         key,
			Enabled:     enabled,
			Value:       value,
			Description: description,
			Origin:      origin,
			Source:      &sourceKind,
		}
		rpcHeaders = append(rpcHeaders, rpcHeader)
	}

	// Sort rpcHeaders by ID, but if it has DeltaParentID use that ID instead
	sort.Slice(rpcHeaders, func(i, j int) bool {
		idI, _ := idwrap.NewFromBytes(rpcHeaders[i].HeaderId)
		idJ, _ := idwrap.NewFromBytes(rpcHeaders[j].HeaderId)

		// Determine the ID to use for sorting for item i
		sortIDI := idI
		if rpcHeaders[i].Origin != nil && len(rpcHeaders[i].Origin.HeaderId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcHeaders[i].Origin.HeaderId); err == nil {
				sortIDI = parentID
			}
		}

		// Determine the ID to use for sorting for item j
		sortIDJ := idJ
		if rpcHeaders[j].Origin != nil && len(rpcHeaders[j].Origin.HeaderId) > 0 {
			if parentID, err := idwrap.NewFromBytes(rpcHeaders[j].Origin.HeaderId); err == nil {
				sortIDJ = parentID
			}
		}

		return sortIDI.Compare(sortIDJ) < 0
	})

	resp := &requestv1.HeaderDeltaListResponse{
		ExampleId: deltaExampleID.Bytes(),
		Items:     rpcHeaders,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) HeaderDeltaCreate(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaCreateRequest]) (*connect.Response[requestv1.HeaderDeltaCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcHeader := requestv1.Header{
		Key:         req.Msg.GetKey(),
		Enabled:     req.Msg.GetEnabled(),
		Value:       req.Msg.GetValue(),
		Description: req.Msg.GetDescription(),
	}
	header := theader.SerlializeHeaderRPCtoModelNoID(&rpcHeader, exID, nil)

	headerID := idwrap.NewNow()
	header.ID = headerID

	// Always set source as delta since this is HeaderDeltaCreate
	header.Source = mexampleheader.HeaderSourceDelta

	// Check if header_id is provided in request
	if len(req.Msg.GetHeaderId()) > 0 {
		// Header ID is provided, verify it exists and use as delta parent
		parentHeaderID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Verify the parent header exists and belongs to the same example
		parentHeader, err := c.ehs.GetHeaderByID(ctx, parentHeaderID)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		// Verify parent header belongs to the same example
		if parentHeader.ExampleID.Compare(exID) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("parent header does not belong to the specified example"))
		}

		header.DeltaParentID = &parentHeaderID
	}
	// If no header_id provided, DeltaParentID remains nil (standalone delta)

	err = c.ehs.CreateHeader(ctx, header)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeltaCreateResponse{HeaderId: headerID.Bytes()}), nil
}

func (c RequestRPC) HeaderDeltaUpdate(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaUpdateRequest]) (*connect.Response[requestv1.HeaderDeltaUpdateResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerHeader(ctx, c.ehs, c.iaes, c.cs, c.us, headerID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the existing header to check its source
	existingHeader, err := c.ehs.GetHeaderByID(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If this is an origin header, we need to create a mixed header instead of updating
	if existingHeader.Source == mexampleheader.HeaderSourceOrigin {
		// Create a new mixed header with updated fields
		mixedHeader := mexampleheader.Header{
			ID:            idwrap.NewNow(),
			ExampleID:     existingHeader.ExampleID,
			DeltaParentID: &existingHeader.ID, // Point to the original header
			HeaderKey:     req.Msg.GetKey(),
			Enable:        req.Msg.GetEnabled(),
			Description:   req.Msg.GetDescription(),
			Value:         req.Msg.GetValue(),
			Source:        mexampleheader.HeaderSourceMixed, // Set as mixed
		}

		err = c.ehs.CreateHeader(ctx, mixedHeader)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		// If it's already a delta or mixed header, just update it normally
		rpcHeader := requestv1.Header{
			HeaderId:    req.Msg.GetHeaderId(),
			Key:         req.Msg.GetKey(),
			Enabled:     req.Msg.GetEnabled(),
			Value:       req.Msg.GetValue(),
			Description: req.Msg.GetDescription(),
		}
		header, err := theader.SerlializeHeaderRPCtoModel(&rpcHeader, idwrap.IDWrap{})
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Preserve the existing source type
		header.Source = existingHeader.Source
		header.DeltaParentID = existingHeader.DeltaParentID

		err = c.ehs.UpdateHeader(ctx, header)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&requestv1.HeaderDeltaUpdateResponse{}), nil
}

func (c RequestRPC) HeaderDeltaDelete(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaDeleteRequest]) (*connect.Response[requestv1.HeaderDeltaDeleteResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerHeader(ctx, c.ehs, c.iaes, c.cs, c.us, headerID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.ehs.DeleteHeader(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeltaDeleteResponse{}), nil
}

func (c RequestRPC) HeaderDeltaReset(ctx context.Context, req *connect.Request[requestv1.HeaderDeltaResetRequest]) (*connect.Response[requestv1.HeaderDeltaResetResponse], error) {
	headerID, err := idwrap.NewFromBytes(req.Msg.GetHeaderId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerHeader(ctx, c.ehs, c.iaes, c.cs, c.us, headerID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get the delta header
	deltaHeader, err := c.ehs.GetHeaderByID(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// If this is a mixed header with a parent, delete the mixed header to revert to origin
	if deltaHeader.Source == mexampleheader.HeaderSourceMixed && deltaHeader.DeltaParentID != nil {
		err = c.ehs.DeleteHeader(ctx, headerID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else if deltaHeader.DeltaParentID != nil {
		// If it's a delta header with a parent, restore values from parent and set source to origin
		parentHeader, err := c.ehs.GetHeaderByID(ctx, *deltaHeader.DeltaParentID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Restore delta header fields to match parent and set source to origin
		deltaHeader.HeaderKey = parentHeader.HeaderKey
		deltaHeader.Enable = parentHeader.Enable
		deltaHeader.Description = parentHeader.Description
		deltaHeader.Value = parentHeader.Value

		err = c.ehs.UpdateHeader(ctx, deltaHeader)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		// If no parent, use the original reset behavior (clear fields)
		err = c.ehs.ResetHeaderDelta(ctx, headerID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&requestv1.HeaderDeltaResetResponse{}), nil
}

func (c RequestRPC) AssertList(ctx context.Context, req *connect.Request[requestv1.AssertListRequest]) (*connect.Response[requestv1.AssertListResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	asserts, err := c.as.GetAssertByExampleID(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var rpcAssserts []*requestv1.AssertListItem
	for _, a := range asserts {
		rpcAssert, err := tassert.SerializeAssertModelToRPCItem(a)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rpcAssserts = append(rpcAssserts, rpcAssert)
	}

	resp := &requestv1.AssertListResponse{
		ExampleId: exID.Bytes(),
		Items:     rpcAssserts,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) AssertCreate(ctx context.Context, req *connect.Request[requestv1.AssertCreateRequest]) (*connect.Response[requestv1.AssertCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcAssert := requestv1.Assert{
		Condition: req.Msg.GetCondition(),
	}

	var deltaParentIDPtr *idwrap.IDWrap
	assert := tassert.SerializeAssertRPCToModelWithoutID(&rpcAssert, exID, deltaParentIDPtr)
	assert.Enable = true
	assert.ID = idwrap.NewNow()
	err = c.as.CreateAssert(ctx, assert)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertCreateResponse{AssertId: assert.ID.Bytes()}), nil
}

func (c RequestRPC) AssertUpdate(ctx context.Context, req *connect.Request[requestv1.AssertUpdateRequest]) (*connect.Response[requestv1.AssertUpdateResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcAssert := requestv1.Assert{
		AssertId:  req.Msg.GetAssertId(),
		Condition: req.Msg.GetCondition(),
	}

	_, err = tassert.SerializeAssertRPCToModel(&rpcAssert, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	assertDB, err := c.as.GetAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	assertDB.Condition = tcondition.DeserializeConditionRPCToModel(req.Msg.GetCondition())

	err = c.as.UpdateAssert(ctx, *assertDB)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertUpdateResponse{}), nil
}

func (c RequestRPC) AssertDelete(ctx context.Context, req *connect.Request[requestv1.AssertDeleteRequest]) (*connect.Response[requestv1.AssertDeleteResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.as.DeleteAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertDeleteResponse{}), nil
}

func (c RequestRPC) AssertDeltaList(ctx context.Context, req *connect.Request[requestv1.AssertDeltaListRequest]) (*connect.Response[requestv1.AssertDeltaListResponse], error) {
	exampleID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exampleID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	asserts, err := c.as.GetAssertByExampleID(ctx, exampleID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var rpcAssserts []*requestv1.AssertDeltaListItem
	for _, a := range asserts {
		rpcAssert, err := tassert.SerializeAssertModelToRPCDeltaItem(a)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rpcAssserts = append(rpcAssserts, rpcAssert)
	}
	resp := &requestv1.AssertDeltaListResponse{
		ExampleId: req.Msg.GetExampleId(),
		Items:     rpcAssserts,
	}
	return connect.NewResponse(resp), nil
}

func (c RequestRPC) AssertDeltaCreate(ctx context.Context, req *connect.Request[requestv1.AssertDeltaCreateRequest]) (*connect.Response[requestv1.AssertDeltaCreateResponse], error) {
	exID, err := idwrap.NewFromBytes(req.Msg.GetExampleId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, exID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcAssert := requestv1.Assert{
		Condition: req.Msg.GetCondition(),
	}

	var deltaParentIDPtr *idwrap.IDWrap
	assert := tassert.SerializeAssertRPCToModelWithoutID(&rpcAssert, exID, deltaParentIDPtr)
	assert.Enable = true
	assert.ID = idwrap.NewNow()
	err = c.as.CreateAssert(ctx, assert)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.AssertDeltaCreateResponse{AssertId: assert.ID.Bytes()}), nil
}

func (c RequestRPC) AssertDeltaUpdate(ctx context.Context, req *connect.Request[requestv1.AssertDeltaUpdateRequest]) (*connect.Response[requestv1.AssertDeltaUpdateResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	rpcAssert := requestv1.Assert{
		AssertId:  req.Msg.GetAssertId(),
		Condition: req.Msg.GetCondition(),
	}

	assert, err := tassert.SerializeAssertRPCToModel(&rpcAssert, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	assertDB, err := c.as.GetAssert(ctx, assert.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	condition := tcondition.DeserializeConditionRPCToModel(req.Msg.Condition)
	expr := condition.Comparisons.Expression
	if expr != "" {
		assertDB.Condition = condition
	}
	err = c.as.UpdateAssert(ctx, *assertDB)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.AssertDeltaUpdateResponse{}), nil
}

func (c RequestRPC) AssertDeltaDelete(ctx context.Context, req *connect.Request[requestv1.AssertDeltaDeleteRequest]) (*connect.Response[requestv1.AssertDeltaDeleteResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.as.DeleteAssert(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.AssertDeltaDeleteResponse{}), nil
}

func (c RequestRPC) AssertDeltaReset(ctx context.Context, req *connect.Request[requestv1.AssertDeltaResetRequest]) (*connect.Response[requestv1.AssertDeltaResetResponse], error) {
	assertID, err := idwrap.NewFromBytes(req.Msg.GetAssertId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assertID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.as.ResetAssertDelta(ctx, assertID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.AssertDeltaResetResponse{}), nil
}
