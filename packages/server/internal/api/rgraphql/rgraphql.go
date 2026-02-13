//nolint:revive // exported
package rgraphql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	graphqlv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/graph_q_l/v1/graph_q_lv1connect"
)

const (
	eventTypeInsert = "insert"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

// Topic/Event types for each entity

type GraphQLTopic struct {
	WorkspaceID idwrap.IDWrap
}

type GraphQLEvent struct {
	Type    string
	GraphQL *graphqlv1.GraphQL
}

type GraphQLHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

type GraphQLHeaderEvent struct {
	Type          string
	GraphQLHeader *graphqlv1.GraphQLHeader
}

type GraphQLResponseTopic struct {
	WorkspaceID idwrap.IDWrap
}

type GraphQLResponseEvent struct {
	Type            string
	GraphQLResponse *graphqlv1.GraphQLResponse
}

type GraphQLResponseHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

type GraphQLResponseHeaderEvent struct {
	Type                  string
	GraphQLResponseHeader *graphqlv1.GraphQLResponseHeader
}

// GraphQLStreamers groups all event streams
type GraphQLStreamers struct {
	GraphQL               eventstream.SyncStreamer[GraphQLTopic, GraphQLEvent]
	GraphQLHeader         eventstream.SyncStreamer[GraphQLHeaderTopic, GraphQLHeaderEvent]
	GraphQLResponse       eventstream.SyncStreamer[GraphQLResponseTopic, GraphQLResponseEvent]
	GraphQLResponseHeader eventstream.SyncStreamer[GraphQLResponseHeaderTopic, GraphQLResponseHeaderEvent]
	File                  eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
}

// GraphQLServiceRPC handles GraphQL RPC operations
type GraphQLServiceRPC struct {
	DB *sql.DB

	graphqlService  sgraphql.GraphQLService
	headerService   sgraphql.GraphQLHeaderService
	responseService sgraphql.GraphQLResponseService

	us         suser.UserService
	ws         sworkspace.WorkspaceService
	wus        sworkspace.UserService
	userReader *sworkspace.UserReader
	wsReader   *sworkspace.WorkspaceReader

	es senv.EnvService
	vs senv.VariableService

	fileService *sfile.FileService
	streamers   *GraphQLStreamers
}

type GraphQLServiceRPCDeps struct {
	DB        *sql.DB
	Services  GraphQLServiceRPCServices
	Readers   GraphQLServiceRPCReaders
	Streamers *GraphQLStreamers
}

type GraphQLServiceRPCServices struct {
	GraphQL       sgraphql.GraphQLService
	Header        sgraphql.GraphQLHeaderService
	Response      sgraphql.GraphQLResponseService
	User          suser.UserService
	Workspace     sworkspace.WorkspaceService
	WorkspaceUser sworkspace.UserService
	Env           senv.EnvService
	Variable      senv.VariableService
	File          *sfile.FileService
}

type GraphQLServiceRPCReaders struct {
	User      *sworkspace.UserReader
	Workspace *sworkspace.WorkspaceReader
}

func (d *GraphQLServiceRPCDeps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if d.Streamers == nil {
		return fmt.Errorf("streamers is required")
	}
	return nil
}

func New(deps GraphQLServiceRPCDeps) GraphQLServiceRPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("GraphQLServiceRPC Deps validation failed: %v", err))
	}

	return GraphQLServiceRPC{
		DB:              deps.DB,
		graphqlService:  deps.Services.GraphQL,
		headerService:   deps.Services.Header,
		responseService: deps.Services.Response,
		us:              deps.Services.User,
		ws:              deps.Services.Workspace,
		wus:             deps.Services.WorkspaceUser,
		userReader:      deps.Readers.User,
		wsReader:        deps.Readers.Workspace,
		es:              deps.Services.Env,
		vs:              deps.Services.Variable,
		fileService:     deps.Services.File,
		streamers:       deps.Streamers,
	}
}

func CreateService(srv GraphQLServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := graph_q_lv1connect.NewGraphQLServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Access control helpers

func (s *GraphQLServiceRPC) checkWorkspaceReadAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := s.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	if wsUser.Role < mworkspace.RoleUser {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	return nil
}

func (s *GraphQLServiceRPC) checkWorkspaceWriteAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := s.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	if wsUser.Role < mworkspace.RoleAdmin {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	return nil
}

func (s *GraphQLServiceRPC) checkWorkspaceDeleteAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	wsUser, err := s.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return connect.NewError(connect.CodeNotFound, errors.New("workspace not found or access denied"))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	if wsUser.Role != mworkspace.RoleOwner {
		return connect.NewError(connect.CodePermissionDenied, errors.New("permission denied"))
	}
	return nil
}

// Mutation publisher for auto-publish on commit

func (s *GraphQLServiceRPC) mutationPublisher() mutation.Publisher {
	return &rgraphqlPublisher{streamers: s.streamers}
}

type rgraphqlPublisher struct {
	streamers *GraphQLStreamers
}

func (p *rgraphqlPublisher) PublishAll(events []mutation.Event) {
	for _, evt := range events {
		//nolint:exhaustive
		switch evt.Entity {
		case mutation.EntityGraphQL:
			p.publishGraphQL(evt)
		case mutation.EntityGraphQLHeader:
			p.publishGraphQLHeader(evt)
		}
	}
}

func (p *rgraphqlPublisher) publishGraphQL(evt mutation.Event) {
	if p.streamers.GraphQL == nil {
		return
	}
	var model *graphqlv1.GraphQL
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if evt.Op == mutation.OpInsert {
			eventType = eventTypeInsert
		} else {
			eventType = eventTypeUpdate
		}
		if g, ok := evt.Payload.(mgraphql.GraphQL); ok {
			model = ToAPIGraphQL(g)
		} else if gp, ok := evt.Payload.(*mgraphql.GraphQL); ok {
			model = ToAPIGraphQL(*gp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		model = &graphqlv1.GraphQL{GraphqlId: evt.ID.Bytes()}
	}

	if model != nil {
		p.streamers.GraphQL.Publish(GraphQLTopic{WorkspaceID: evt.WorkspaceID}, GraphQLEvent{
			Type:    eventType,
			GraphQL: model,
		})
	}
}

func (p *rgraphqlPublisher) publishGraphQLHeader(evt mutation.Event) {
	if p.streamers.GraphQLHeader == nil {
		return
	}
	var model *graphqlv1.GraphQLHeader
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if evt.Op == mutation.OpInsert {
			eventType = eventTypeInsert
		} else {
			eventType = eventTypeUpdate
		}
		if h, ok := evt.Payload.(mgraphql.GraphQLHeader); ok {
			model = ToAPIGraphQLHeader(h)
		} else if hp, ok := evt.Payload.(*mgraphql.GraphQLHeader); ok {
			model = ToAPIGraphQLHeader(*hp)
		}
	case mutation.OpDelete:
		eventType = eventTypeDelete
		model = &graphqlv1.GraphQLHeader{GraphqlHeaderId: evt.ID.Bytes(), GraphqlId: evt.ParentID.Bytes()}
	}

	if model != nil {
		p.streamers.GraphQLHeader.Publish(GraphQLHeaderTopic{WorkspaceID: evt.WorkspaceID}, GraphQLHeaderEvent{
			Type:          eventType,
			GraphQLHeader: model,
		})
	}
}

// Sync stream handlers

func (s *GraphQLServiceRPC) GraphQLSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamGraphQLSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLTopic) bool {
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

	converter := func(events []GraphQLEvent) *graphqlv1.GraphQLSyncResponse {
		var items []*graphqlv1.GraphQLSync
		for _, event := range events {
			if resp := graphqlSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(ctx, s.streamers.GraphQL, filter, converter, send, nil)
}

func (s *GraphQLServiceRPC) GraphQLHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLHeaderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamGraphQLHeaderSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLHeaderSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLHeaderSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLHeaderTopic) bool {
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

	converter := func(events []GraphQLHeaderEvent) *graphqlv1.GraphQLHeaderSyncResponse {
		var items []*graphqlv1.GraphQLHeaderSync
		for _, event := range events {
			if resp := graphqlHeaderSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLHeaderSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(ctx, s.streamers.GraphQLHeader, filter, converter, send, nil)
}

func (s *GraphQLServiceRPC) GraphQLResponseSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLResponseSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamGraphQLResponseSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLResponseSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLResponseSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLResponseTopic) bool {
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

	converter := func(events []GraphQLResponseEvent) *graphqlv1.GraphQLResponseSyncResponse {
		var items []*graphqlv1.GraphQLResponseSync
		for _, event := range events {
			if resp := graphqlResponseSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLResponseSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(ctx, s.streamers.GraphQLResponse, filter, converter, send, nil)
}

func (s *GraphQLServiceRPC) GraphQLResponseHeaderSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[graphqlv1.GraphQLResponseHeaderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}
	return s.streamGraphQLResponseHeaderSync(ctx, userID, stream.Send)
}

func (s *GraphQLServiceRPC) streamGraphQLResponseHeaderSync(ctx context.Context, userID idwrap.IDWrap, send func(*graphqlv1.GraphQLResponseHeaderSyncResponse) error) error {
	var workspaceSet sync.Map

	filter := func(topic GraphQLResponseHeaderTopic) bool {
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

	converter := func(events []GraphQLResponseHeaderEvent) *graphqlv1.GraphQLResponseHeaderSyncResponse {
		var items []*graphqlv1.GraphQLResponseHeaderSync
		for _, event := range events {
			if resp := graphqlResponseHeaderSyncResponseFrom(event); resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &graphqlv1.GraphQLResponseHeaderSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(ctx, s.streamers.GraphQLResponseHeader, filter, converter, send, nil)
}
