//nolint:revive // exported
package rgraphql

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
)

func (s *GraphQLServiceRPC) GraphQLHeaderCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[graphqlv1.GraphQLHeaderCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wsReader.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var allItems []*graphqlv1.GraphQLHeader
	for _, ws := range workspaces {
		gqlList, err := s.graphqlService.GetByWorkspaceID(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, gql := range gqlList {
			headers, err := s.headerService.GetByGraphQLID(ctx, gql.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			for _, h := range headers {
				allItems = append(allItems, ToAPIGraphQLHeader(h))
			}
		}
	}

	return connect.NewResponse(&graphqlv1.GraphQLHeaderCollectionResponse{Items: allItems}), nil
}

func (s *GraphQLServiceRPC) GraphQLHeaderInsert(ctx context.Context, req *connect.Request[graphqlv1.GraphQLHeaderInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	for _, item := range req.Msg.Items {
		if len(item.GraphqlHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_header_id is required"))
		}
		if len(item.GraphqlId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.GraphqlHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		gqlID, err := idwrap.NewFromBytes(item.GraphqlId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.graphqlService.GetWorkspaceID(ctx, gqlID)
		if err != nil {
			if errors.Is(err, sgraphql.ErrNoGraphQLFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceWriteAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		header := &mgraphql.GraphQLHeader{
			ID:           headerID,
			GraphQLID:    gqlID,
			Key:          item.Key,
			Value:        item.Value,
			Enabled:      item.Enabled,
			Description:  item.Description,
			DisplayOrder: item.Order,
		}

		if err := s.headerService.Create(ctx, header); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if s.streamers.GraphQLHeader != nil {
			s.streamers.GraphQLHeader.Publish(GraphQLHeaderTopic{WorkspaceID: workspaceID}, GraphQLHeaderEvent{
				Type:          eventTypeInsert,
				GraphQLHeader: ToAPIGraphQLHeader(*header),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLHeaderUpdate(ctx context.Context, req *connect.Request[graphqlv1.GraphQLHeaderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	for _, item := range req.Msg.Items {
		if len(item.GraphqlHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.GraphqlHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existingHeaders, err := s.headerService.GetByIDs(ctx, []idwrap.IDWrap{headerID})
		if err != nil || len(existingHeaders) == 0 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("header not found"))
		}
		existing := existingHeaders[0]

		workspaceID, err := s.graphqlService.GetWorkspaceID(ctx, existing.GraphQLID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceWriteAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		if item.Key != nil {
			existing.Key = *item.Key
		}
		if item.Value != nil {
			existing.Value = *item.Value
		}
		if item.Enabled != nil {
			existing.Enabled = *item.Enabled
		}
		if item.Description != nil {
			existing.Description = *item.Description
		}
		if item.Order != nil {
			existing.DisplayOrder = *item.Order
		}

		if err := s.headerService.Update(ctx, &existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if s.streamers.GraphQLHeader != nil {
			s.streamers.GraphQLHeader.Publish(GraphQLHeaderTopic{WorkspaceID: workspaceID}, GraphQLHeaderEvent{
				Type:          eventTypeUpdate,
				GraphQLHeader: ToAPIGraphQLHeader(existing),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *GraphQLServiceRPC) GraphQLHeaderDelete(ctx context.Context, req *connect.Request[graphqlv1.GraphQLHeaderDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	for _, item := range req.Msg.Items {
		if len(item.GraphqlHeaderId) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("graphql_header_id is required"))
		}

		headerID, err := idwrap.NewFromBytes(item.GraphqlHeaderId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existingHeaders, err := s.headerService.GetByIDs(ctx, []idwrap.IDWrap{headerID})
		if err != nil || len(existingHeaders) == 0 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("header not found"))
		}
		existing := existingHeaders[0]

		workspaceID, err := s.graphqlService.GetWorkspaceID(ctx, existing.GraphQLID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.checkWorkspaceDeleteAccess(ctx, workspaceID); err != nil {
			return nil, err
		}

		if err := s.headerService.Delete(ctx, headerID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if s.streamers.GraphQLHeader != nil {
			s.streamers.GraphQLHeader.Publish(GraphQLHeaderTopic{WorkspaceID: workspaceID}, GraphQLHeaderEvent{
				Type:          eventTypeDelete,
				GraphQLHeader: &graphqlv1.GraphQLHeader{GraphqlHeaderId: headerID.Bytes(), GraphqlId: existing.GraphQLID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
