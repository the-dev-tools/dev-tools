//nolint:revive // exported
package rgraphql

import (
	"context"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

// GraphQLResponseAssert operations

func (s *GraphQLServiceRPC) GraphQLResponseAssertCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLResponseAssertCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Collect all response asserts across user's workspaces
	var allAsserts []*graphqlv1.GraphQLResponseAssert
	for _, workspace := range workspaces {
		asserts, err := s.responseService.GetAssertsByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			continue
		}
		for _, assert := range asserts {
			allAsserts = append(allAsserts, ToAPIGraphQLResponseAssert(assert))
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLResponseAssertCollectionResponse{Items: allAsserts}), nil
}

func (s *GraphQLServiceRPC) GraphQLResponseAssertSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLResponseAssertSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return s.streamGraphQLResponseAssertSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLResponseAssertSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLResponseAssertSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLResponseAssertTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, topic.WorkspaceID)
		if err != nil || !belongs {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	converter := func(events []GraphQLResponseAssertEvent) *graphqlv1.GraphQLResponseAssertSyncResponse {
		var items []*graphqlv1.GraphQLResponseAssertSync
		for _, event := range events {
			if resp := graphqlResponseAssertSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLResponseAssertSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		s.streamers.GraphQLResponseAssert,
		filter,
		converter,
		send,
		nil,
	)
}
