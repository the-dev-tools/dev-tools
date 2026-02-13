//nolint:revive // exported
package rgraphql

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

func (s *GraphQLServiceRPC) GraphQLResponseCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLResponseCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allItems []*graphqlv1.GraphQLResponse
	for _, ws := range workspaces {
		responses, err := s.responseService.GetByWorkspaceID(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, r := range responses {
			allItems = append(allItems, ToAPIGraphQLResponse(r))
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLResponseCollectionResponse{Items: allItems}), nil
}

func (s *GraphQLServiceRPC) GraphQLResponseHeaderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLResponseHeaderCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allItems []*graphqlv1.GraphQLResponseHeader
	for _, ws := range workspaces {
		headers, err := s.responseService.GetHeadersByWorkspaceID(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, h := range headers {
			allItems = append(allItems, ToAPIGraphQLResponseHeader(h))
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLResponseHeaderCollectionResponse{Items: allItems}), nil
}
