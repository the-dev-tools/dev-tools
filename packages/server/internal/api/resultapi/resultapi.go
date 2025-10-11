package resultapi

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tassert"
	"the-dev-tools/server/pkg/translate/texampleresp"
	responsev1 "the-dev-tools/spec/dist/buf/go/collection/item/response/v1"
	"the-dev-tools/spec/dist/buf/go/collection/item/response/v1/responsev1connect"
	"time"

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

const (
	responseLookupAttempts = 12
	responseLookupDelay    = 60 * time.Millisecond
)

func (c *ResultService) ResponseGet(ctx context.Context, req *connect.Request[responsev1.ResponseGetRequest]) (*connect.Response[responsev1.ResponseGetResponse], error) {
	ResponseID, err := idwrap.NewFromBytes(req.Msg.ResponseId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	result, rpcErr := fetchExampleRespWithRetry(ctx, c.ers, ResponseID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	permErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, result.ExampleID))
	if permErr != nil {
		return nil, permErr
	}

	rpcResp, err := texampleresp.SeralizeModelToRPC(*result)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	size := int32(len(rpcResp.Body))

	resp := &responsev1.ResponseGetResponse{
		ResponseId: rpcResp.ResponseId,
		Status:     rpcResp.Status,
		Body:       rpcResp.Body,
		Time:       rpcResp.Time,
		Duration:   rpcResp.Duration,
		Size:       size,
	}

	return connect.NewResponse(resp), nil
}

func waitOnContext(ctx context.Context, d time.Duration) *connect.Error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return connect.NewError(connect.CodeDeadlineExceeded, ctx.Err())
		}
		return connect.NewError(connect.CodeCanceled, ctx.Err())
	case <-timer.C:
		return nil
	}
}

func fetchExampleRespWithRetry(ctx context.Context, ers sexampleresp.ExampleRespService, respID idwrap.IDWrap) (*mexampleresp.ExampleResp, *connect.Error) {
	var lastErr error
	for attempt := 0; attempt < responseLookupAttempts; attempt++ {
		resp, err := ers.GetExampleResp(ctx, respID)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if !errors.Is(err, sexampleresp.ErrNoRespFound) && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if attempt == responseLookupAttempts-1 {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		if waitErr := waitOnContext(ctx, responseLookupDelay); waitErr != nil {
			return nil, waitErr
		}
	}

	return nil, connect.NewError(connect.CodeNotFound, lastErr)
}

func (c *ResultService) ResponseHeaderList(ctx context.Context, req *connect.Request[responsev1.ResponseHeaderListRequest]) (*connect.Response[responsev1.ResponseHeaderListResponse], error) {
	ResponseID, err := idwrap.NewFromBytes(req.Msg.ResponseId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	respModel, rpcErr := fetchExampleRespWithRetry(ctx, c.ers, ResponseID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	permErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, respModel.ExampleID))
	if permErr != nil {
		return nil, permErr
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

	respModel, rpcErr := fetchExampleRespWithRetry(ctx, c.ers, ResponseID)
	if rpcErr != nil {
		return nil, rpcErr
	}

	permErr := permcheck.CheckPerm(ritemapiexample.CheckOwnerExample(ctx, c.iaes, c.cs, c.us, respModel.ExampleID))
	if permErr != nil {
		return nil, permErr
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
	resp, rpcErr := fetchExampleRespWithRetry(ctx, ers, respID)
	if rpcErr != nil {
		return false, rpcErr
	}
	return ritemapiexample.CheckOwnerExample(ctx, iaes, cs, us, resp.ExampleID)
}
