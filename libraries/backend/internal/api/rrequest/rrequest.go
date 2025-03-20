package rrequest

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/ritemapiexample"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massert"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sassert"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/translate/tassert"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/backend/pkg/translate/theader"
	"the-dev-tools/backend/pkg/translate/tquery"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/request/v1/requestv1connect"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"

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
	queries, err := c.eqs.GetExampleQueriesByExampleID(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcQueries := tgeneric.MassConvert(queries, tquery.SerializeQueryModelToRPCItem)
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
		Key:           req.Msg.GetKey(),
		Enabled:       req.Msg.GetEnabled(),
		Value:         req.Msg.GetValue(),
		Description:   req.Msg.GetDescription(),
		ParentQueryId: req.Msg.GetParentQueryId(),
	}
	query, err := tquery.SerlializeQueryRPCtoModelNoID(&reqQuery, exID)
	if err != nil {
		return nil, err
	}
	queryID := idwrap.NewNow()
	query.ID = queryID

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

	err = c.eqs.UpdateExampleQuery(ctx, query)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

	err = c.eqs.DeleteExampleQuery(ctx, queryID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&requestv1.QueryDeleteResponse{}), nil
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

	headers, err := c.ehs.GetHeaderByExampleID(ctx, exID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcHeaders := tgeneric.MassConvert(headers, theader.SerializeHeaderModelToRPCItem)
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
		Key:            req.Msg.GetKey(),
		Enabled:        req.Msg.GetEnabled(),
		Value:          req.Msg.GetValue(),
		Description:    req.Msg.GetDescription(),
		ParentHeaderId: req.Msg.GetParentHeaderId(),
	}
	headerID := idwrap.NewNow()
	var deltaParentIDPtr *idwrap.IDWrap
	if len(rpcHeader.ParentHeaderId) > 0 {
		deltaParentID, err := idwrap.NewFromBytes(rpcHeader.GetParentHeaderId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		deltaParentIDPtr = &deltaParentID
	}

	header := theader.SerlializeHeaderRPCtoModelNoID(&rpcHeader, exID, deltaParentIDPtr)
	header.ID = headerID
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
	err = c.ehs.UpdateHeader(ctx, header)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	err = c.ehs.DeleteHeader(ctx, headerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&requestv1.HeaderDeleteResponse{}), nil
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
	if len(rpcAssert.ParentAssertId) > 0 {
		deltaParentID, err := idwrap.NewFromBytes(rpcAssert.GetParentAssertId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		deltaParentIDPtr = &deltaParentID
	}

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
	rpcAssert := requestv1.Assert{
		AssertId:  req.Msg.GetAssertId(),
		Condition: req.Msg.GetCondition(),
	}
	assert, err := tassert.SerializeAssertRPCToModel(&rpcAssert, idwrap.IDWrap{})
	assert.Enable = true
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	assertDB, err := c.as.GetAssert(ctx, assert.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if assert.Type == massert.AssertType(referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED) {
		assert.Type = assertDB.Type
	}

	comp := rpcAssert.Condition.Comparison
	for i, pathKey := range comp.Path {
		if pathKey.GetKind() == referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_UNSPECIFIED {
			comp.Path[i].Kind = referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_INDEX
		}
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerAssert(ctx, c.as, c.iaes, c.cs, c.us, assert.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.as.UpdateAssert(ctx, assert)
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
