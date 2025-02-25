package resultapi

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/ritemapiexample"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sassert"
	"the-dev-tools/backend/pkg/service/sassertres"
	"the-dev-tools/backend/pkg/service/scollection"
	"the-dev-tools/backend/pkg/service/sexampleresp"
	"the-dev-tools/backend/pkg/service/sexamplerespheader"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/translate/tassert"
	"the-dev-tools/backend/pkg/translate/texampleresp"
	responsev1 "the-dev-tools/spec/dist/buf/go/collection/item/response/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/response/v1/responsev1connect"

	"connectrpc.com/connect"
)

type ResultService struct {
	DB   *sql.DB
	us   suser.UserService
	cs   scollection.CollectionService
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	ws   sworkspace.WorkspaceService

	// Response
	ers  sexampleresp.ExampleRespService
	erhs sexamplerespheader.ExampleRespHeaderService

	// Assert
	as   sassert.AssertService
	asrs sassertres.AssertResultService
}

func New(db *sql.DB, us suser.UserService, cs scollection.CollectionService, ias sitemapi.ItemApiService,
	iaes sitemapiexample.ItemApiExampleService, ws sworkspace.WorkspaceService,
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService,
	as sassert.AssertService, asrs sassertres.AssertResultService,
) ResultService {
	return ResultService{
		DB:   db,
		us:   us,
		cs:   cs,
		ias:  ias,
		iaes: iaes,
		ws:   ws,
		ers:  ers,
		erhs: erhs,
		as:   as,
		asrs: asrs,
	}
}

func CreateService(srv ResultService, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := responsev1connect.NewResponseServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *ResultService) ResponseGet(ctx context.Context, req *connect.Request[responsev1.ResponseGetRequest]) (*connect.Response[responsev1.ResponseGetResponse], error) {
	ResponseID, err := idwrap.NewFromBytes(req.Msg.ResponseId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerResp(ctx, ResponseID, c.ers, c.iaes, c.cs, c.us))
	if rpcErr != nil {
		return nil, rpcErr
	}

	result, err := c.ers.GetExampleResp(ctx, ResponseID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rpcResp, err := texampleresp.SeralizeModelToRPC(*result)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &responsev1.ResponseGetResponse{
		ResponseId: rpcResp.ResponseId,
		Status:     rpcResp.Status,
		Body:       rpcResp.Body,
		Time:       rpcResp.Time,
		Duration:   rpcResp.Duration,
	}

	return connect.NewResponse(resp), nil
}

func (c *ResultService) ResponseHeaderList(ctx context.Context, req *connect.Request[responsev1.ResponseHeaderListRequest]) (*connect.Response[responsev1.ResponseHeaderListResponse], error) {
	ResponseID, err := idwrap.NewFromBytes(req.Msg.ResponseId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerResp(ctx, ResponseID, c.ers, c.iaes, c.cs, c.us))
	if rpcErr != nil {
		return nil, rpcErr
	}

	headers, err := c.erhs.GetHeaderByRespID(ctx, ResponseID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: move to translate package
	var rpcHeaders []*responsev1.ResponseHeaderListItem
	for _, header := range headers {
		rpcHeader := &responsev1.ResponseHeaderListItem{
			ResponseHeaderId: header.ID.Bytes(),
			Key:              header.HeaderKey,
			Value:            header.Value,
		}
		rpcHeaders = append(rpcHeaders, rpcHeader)
	}

	resp := &responsev1.ResponseHeaderListResponse{
		ResponseId: req.Msg.ResponseId,
		Items:      rpcHeaders,
	}

	return connect.NewResponse(resp), nil
}

func (c *ResultService) ResponseAssertList(ctx context.Context, req *connect.Request[responsev1.ResponseAssertListRequest]) (*connect.Response[responsev1.ResponseAssertListResponse], error) {
	ResponseID, err := idwrap.NewFromBytes(req.Msg.ResponseId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerResp(ctx, ResponseID, c.ers, c.iaes, c.cs, c.us))
	if rpcErr != nil {
		return nil, rpcErr
	}

	assertResponse, err := c.asrs.GetAssertResultsByResponseID(ctx, ResponseID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// TODO: move to translate package
	var rpcAssertResponses []*responsev1.ResponseAssertListItem
	for _, assertResp := range assertResponse {
		assert, err := c.as.GetAssert(ctx, assertResp.AssertID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		a, err := tassert.SerializeAssertModelToRPC(*assert)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcAssertResp := &responsev1.ResponseAssertListItem{
			Assert: a,
			Result: assertResp.Result,
		}
		rpcAssertResponses = append(rpcAssertResponses, rpcAssertResp)
	}

	resp := &responsev1.ResponseAssertListResponse{
		Items:      rpcAssertResponses,
		ResponseId: req.Msg.ResponseId,
	}

	return connect.NewResponse(resp), nil
}

func CheckOwnerResp(ctx context.Context, respID idwrap.IDWrap, ers sexampleresp.ExampleRespService,
	iaes sitemapiexample.ItemApiExampleService, cs scollection.CollectionService, us suser.UserService,
) (bool, error) {
	resp, err := ers.GetExampleResp(ctx, respID)
	if err != nil {
		return false, err
	}

	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, resp.ExampleID)
}
