//nolint:revive // exported
package rhttp

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/model/mhttp"

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
		// Get base HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta HTTP entries (responses can be stored against delta IDs)
		deltaList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allHTTPs := make([]mhttp.HTTP, len(httpList), len(httpList)+len(deltaList))
		copy(allHTTPs, httpList)
		allHTTPs = append(allHTTPs, deltaList...)

		// Get responses for each HTTP entry
		for _, http := range allHTTPs {
			responses, err := h.httpResponseService.GetByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for _, response := range responses {
				apiResponse := converter.ToAPIHttpResponse(response)
				allResponses = append(allResponses, apiResponse)
			}
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
		// Get base HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta HTTP entries (response headers can be stored against delta IDs)
		deltaList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allHTTPs := make([]mhttp.HTTP, 0, len(httpList)+len(deltaList))
		allHTTPs = append(allHTTPs, httpList...)
		allHTTPs = append(allHTTPs, deltaList...)

		// Get response headers for each HTTP entry
		for _, http := range allHTTPs {
			headers, err := h.httpResponseService.GetHeadersByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for _, header := range headers {
				apiHeader := converter.ToAPIHttpResponseHeader(header)
				allHeaders = append(allHeaders, apiHeader)
			}
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
		// Get base HTTP entries for this workspace
		httpList, err := h.hs.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta HTTP entries (response asserts can be stored against delta IDs)
		deltaList, err := h.hs.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allHTTPs := make([]mhttp.HTTP, len(httpList), len(httpList)+len(deltaList))
		copy(allHTTPs, httpList)
		allHTTPs = append(allHTTPs, deltaList...)

		// Get response asserts for each HTTP entry
		for _, http := range allHTTPs {
			asserts, err := h.httpResponseService.GetAssertsByHttpID(ctx, http.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			for _, assert := range asserts {
				apiAssert := converter.ToAPIHttpResponseAssert(assert)
				allAsserts = append(allAsserts, apiAssert)
			}
		}
	}

	return connect.NewResponse(&apiv1.HttpResponseAssertCollectionResponse{Items: allAsserts}), nil
}
