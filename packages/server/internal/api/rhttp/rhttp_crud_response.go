//nolint:revive // exported
package rhttp

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"

	apiv1 "the-dev-tools/spec/dist/buf/go/api/http/v1"
)

func (h *HttpServiceRPC) HttpResponseCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allResponses []*apiv1.HttpResponse
	for _, workspace := range workspaces {
		// Get all responses for this workspace directly via JOIN query
		// This is more efficient than iterating through HTTP records
		responses, err := h.httpResponseService.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, response := range responses {
			apiResponse := converter.ToAPIHttpResponse(response)
			allResponses = append(allResponses, apiResponse)
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseCollectionResponse{Items: allResponses}), nil
}

func (h *HttpServiceRPC) HttpResponseHeaderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseHeaderCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allHeaders []*apiv1.HttpResponseHeader
	for _, workspace := range workspaces {
		// Get all response headers for this workspace directly via JOIN query
		// This is more efficient than iterating through HTTP records
		headers, err := h.httpResponseService.GetHeadersByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, header := range headers {
			apiHeader := converter.ToAPIHttpResponseHeader(header)
			allHeaders = append(allHeaders, apiHeader)
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseHeaderCollectionResponse{Items: allHeaders}), nil
}

func (h *HttpServiceRPC) HttpResponseAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.HttpResponseAssertCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := h.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allAsserts []*apiv1.HttpResponseAssert
	for _, workspace := range workspaces {
		// Get all response asserts for this workspace directly via JOIN query
		// This is more efficient than iterating through HTTP records
		asserts, err := h.httpResponseService.GetAssertsByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, assert := range asserts {
			apiAssert := converter.ToAPIHttpResponseAssert(assert)
			allAsserts = append(allAsserts, apiAssert)
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseAssertCollectionResponse{Items: allAsserts}), nil
}
