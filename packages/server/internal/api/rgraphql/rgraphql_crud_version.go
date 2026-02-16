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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

// GraphQLVersion operations

func (s *GraphQLServiceRPC) GraphQLVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLVersionCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// Get user's workspaces
	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allVersions []*graphqlv1.GraphQLVersion
	for _, workspace := range workspaces {
		// Get base GraphQL entries for this workspace
		graphqlList, err := s.graphqlReader.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Also get delta GraphQL entries (versions can be stored against delta IDs)
		deltaList, err := s.graphqlReader.GetDeltasByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Combine base and delta entries
		allGraphQLs := make([]mgraphql.GraphQL, 0, len(graphqlList)+len(deltaList))
		allGraphQLs = append(allGraphQLs, graphqlList...)
		allGraphQLs = append(allGraphQLs, deltaList...)

		// Get versions for each GraphQL entry
		for _, graphql := range allGraphQLs {
			versions, err := s.graphqlReader.GetGraphQLVersionsByGraphQLID(ctx, graphql.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Convert to API format
			for _, version := range versions {
				apiVersion := ToAPIGraphQLVersion(version)
				allVersions = append(allVersions, apiVersion)
			}
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLVersionCollectionResponse{Items: allVersions}), nil
}

func (s *GraphQLServiceRPC) GraphQLVersionSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLVersionSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	return s.streamGraphQLVersionSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLVersionSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLVersionSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLVersionTopic) bool {
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

	converter := func(events []GraphQLVersionEvent) *graphqlv1.GraphQLVersionSyncResponse {
		var items []*graphqlv1.GraphQLVersionSync
		for _, event := range events {
			if resp := graphqlVersionSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLVersionSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		s.streamers.GraphQLVersion,
		filter,
		converter,
		send,
		nil,
	)
}

// ToAPIGraphQLVersion converts model to API type
func ToAPIGraphQLVersion(version mgraphql.GraphQLVersion) *graphqlv1.GraphQLVersion{
	return &graphqlv1.GraphQLVersion{
		GraphqlVersionId: version.ID.Bytes(),
		GraphqlId:        version.GraphQLID.Bytes(),
		Name:             version.VersionName,
		Description:      version.VersionDescription,
		CreatedAt:        version.CreatedAt,
	}
}

// graphqlVersionSyncResponseFrom converts GraphQL version events to sync responses
func graphqlVersionSyncResponseFrom(event GraphQLVersionEvent) *graphqlv1.GraphQLVersionSyncResponse {
	var value *graphqlv1.GraphQLVersionSync_ValueUnion

	switch event.Type {
	case eventTypeInsert:
		value = &graphqlv1.GraphQLVersionSync_ValueUnion{
			Kind: graphqlv1.GraphQLVersionSync_ValueUnion_KIND_INSERT,
			Insert: &graphqlv1.GraphQLVersionSyncInsert{
				GraphqlVersionId: event.GraphQLVersion.GetGraphqlVersionId(),
				GraphqlId:        event.GraphQLVersion.GetGraphqlId(),
				Name:             event.GraphQLVersion.GetName(),
				Description:      event.GraphQLVersion.GetDescription(),
				CreatedAt:        event.GraphQLVersion.GetCreatedAt(),
			},
		}
	case eventTypeUpdate:
		name := event.GraphQLVersion.GetName()
		description := event.GraphQLVersion.GetDescription()
		createdAt := event.GraphQLVersion.GetCreatedAt()
		value = &graphqlv1.GraphQLVersionSync_ValueUnion{
			Kind: graphqlv1.GraphQLVersionSync_ValueUnion_KIND_UPDATE,
			Update: &graphqlv1.GraphQLVersionSyncUpdate{
				GraphqlVersionId: event.GraphQLVersion.GetGraphqlVersionId(),
				Name:             &name,
				Description:      &description,
				CreatedAt:        &createdAt,
			},
		}
	case eventTypeDelete:
		value = &graphqlv1.GraphQLVersionSync_ValueUnion{
			Kind: graphqlv1.GraphQLVersionSync_ValueUnion_KIND_DELETE,
			Delete: &graphqlv1.GraphQLVersionSyncDelete{
				GraphqlVersionId: event.GraphQLVersion.GetGraphqlVersionId(),
			},
		}
	default:
		return nil
	}

	return &graphqlv1.GraphQLVersionSyncResponse{
		Items: []*graphqlv1.GraphQLVersionSync{{Value: value}},
	}
}
