package rwebsocket

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"

	devtoolsdb "github.com/the-dev-tools/dev-tools/packages/db"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mwebsocket"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/permcheck"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/swebsocket"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/web_socket/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/web_socket/v1/web_socketv1connect"
)

const (
	eventTypeInsert = "insert"
	eventTypeUpdate = "update"
	eventTypeDelete = "delete"
)

type WebSocketTopic struct {
	WorkspaceID idwrap.IDWrap
}

type WebSocketEvent struct {
	Type      string
	WebSocket *apiv1.WebSocket
}

type WebSocketHeaderTopic struct {
	WorkspaceID idwrap.IDWrap
}

type WebSocketHeaderEvent struct {
	Type            string
	WebSocketHeader *apiv1.WebSocketHeader
}

// WebSocketRPC handles WebSocket CRUD operations and real-time sync.
type WebSocketRPC struct {
	web_socketv1connect.UnimplementedWebSocketServiceHandler

	DB        *sql.DB
	ws        swebsocket.WebSocketService
	wsh       swebsocket.WebSocketHeaderService
	us        suser.UserService
	wk        sworkspace.WorkspaceService
	wsStream  eventstream.SyncStreamer[WebSocketTopic, WebSocketEvent]
	wshStream eventstream.SyncStreamer[WebSocketHeaderTopic, WebSocketHeaderEvent]
}

type Deps struct {
	DB        *sql.DB
	WS        swebsocket.WebSocketService
	WSH       swebsocket.WebSocketHeaderService
	US        suser.UserService
	Workspace sworkspace.WorkspaceService
	WSStream  eventstream.SyncStreamer[WebSocketTopic, WebSocketEvent]
	WSHStream eventstream.SyncStreamer[WebSocketHeaderTopic, WebSocketHeaderEvent]
}

func New(deps Deps) WebSocketRPC {
	return WebSocketRPC{
		DB:        deps.DB,
		ws:        deps.WS,
		wsh:       deps.WSH,
		us:        deps.US,
		wk:        deps.Workspace,
		wsStream:  deps.WSStream,
		wshStream: deps.WSHStream,
	}
}

func CreateService(srv WebSocketRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := web_socketv1connect.NewWebSocketServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (s *WebSocketRPC) WebSocketCollection(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.WebSocketCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wk.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*apiv1.WebSocket
	for _, workspace := range workspaces {
		wsList, err := s.ws.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, ws := range wsList {
			items = append(items, &apiv1.WebSocket{
				WebsocketId: ws.ID.Bytes(),
				Name:        ws.Name,
				Url:         ws.Url,
			})
		}
	}

	return connect.NewResponse(&apiv1.WebSocketCollectionResponse{Items: items}), nil
}

func (s *WebSocketRPC) WebSocketSync(ctx context.Context, _ *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.WebSocketSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	var workspaceSet sync.Map
	filter := func(topic WebSocketTopic) bool {
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

	events, err := s.wsStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := webSocketSyncResponseFrom(evt.Payload)
			if resp == nil {
				continue
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *WebSocketRPC) WebSocketHeaderCollection(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.WebSocketHeaderCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wk.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*apiv1.WebSocketHeader
	for _, workspace := range workspaces {
		wsList, err := s.ws.GetByWorkspaceID(ctx, workspace.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, ws := range wsList {
			headers, err := s.wsh.GetByWebSocketID(ctx, ws.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			for _, h := range headers {
				items = append(items, toAPIWebSocketHeader(h))
			}
		}
	}

	return connect.NewResponse(&apiv1.WebSocketHeaderCollectionResponse{Items: items}), nil
}

func (s *WebSocketRPC) WebSocketHeaderSync(ctx context.Context, _ *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.WebSocketHeaderSyncResponse]) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	var workspaceSet sync.Map
	filter := func(topic WebSocketHeaderTopic) bool {
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

	events, err := s.wshStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := webSocketHeaderSyncResponseFrom(evt.Payload)
			if resp == nil {
				continue
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *WebSocketRPC) WebSocketInsert(ctx context.Context, req *connect.Request[apiv1.WebSocketInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	// FETCH
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.wk.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(workspaces) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user has no workspaces"))
	}
	defaultWorkspaceID := workspaces[0].ID

	// CHECK
	rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, s.us, defaultWorkspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Parse items
	now := time.Now().Unix()
	items := make([]mwebsocket.WebSocket, 0, len(req.Msg.Items))
	for _, item := range req.Msg.Items {
		if len(item.GetWebsocketId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("websocket_id is required"))
		}
		wsID, err := idwrap.NewFromBytes(item.GetWebsocketId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		items = append(items, mwebsocket.WebSocket{
			ID:          wsID,
			WorkspaceID: defaultWorkspaceID,
			Name:        item.GetName(),
			Url:         item.GetUrl(),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// ACT
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsTx := s.ws.TX(tx)
	for i := range items {
		if err := wsTx.Create(ctx, &items[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range items {
		s.wsStream.Publish(WebSocketTopic{WorkspaceID: item.WorkspaceID}, WebSocketEvent{
			Type: eventTypeInsert,
			WebSocket: &apiv1.WebSocket{
				WebsocketId: item.ID.Bytes(),
				Name:        item.Name,
				Url:         item.Url,
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *WebSocketRPC) WebSocketUpdate(ctx context.Context, req *connect.Request[apiv1.WebSocketUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	// FETCH + CHECK
	updates := make([]mwebsocket.WebSocket, 0, len(req.Msg.Items))
	for _, item := range req.Msg.Items {
		if len(item.GetWebsocketId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("websocket_id is required"))
		}
		wsID, err := idwrap.NewFromBytes(item.GetWebsocketId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existing, err := s.ws.Get(ctx, wsID)
		if err != nil {
			if errors.Is(err, swebsocket.ErrNoWebSocketFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, s.us, existing.WorkspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Apply partial updates
		if item.Name != nil {
			existing.Name = *item.Name
		}
		if item.Url != nil {
			existing.Url = *item.Url
		}
		existing.UpdatedAt = time.Now().Unix()

		updates = append(updates, *existing)
	}

	// ACT
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsTx := s.ws.TX(tx)
	for i := range updates {
		if err := wsTx.Update(ctx, &updates[i]); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range updates {
		s.wsStream.Publish(WebSocketTopic{WorkspaceID: item.WorkspaceID}, WebSocketEvent{
			Type: eventTypeUpdate,
			WebSocket: &apiv1.WebSocket{
				WebsocketId: item.ID.Bytes(),
				Name:        item.Name,
				Url:         item.Url,
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *WebSocketRPC) WebSocketDelete(ctx context.Context, req *connect.Request[apiv1.WebSocketDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	// FETCH + CHECK
	type deleteItem struct {
		ID          idwrap.IDWrap
		WorkspaceID idwrap.IDWrap
	}
	deleteItems := make([]deleteItem, 0, len(req.Msg.Items))
	for _, item := range req.Msg.Items {
		if len(item.GetWebsocketId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("websocket_id is required"))
		}
		wsID, err := idwrap.NewFromBytes(item.GetWebsocketId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.ws.GetWorkspaceID(ctx, wsID)
		if err != nil {
			if errors.Is(err, swebsocket.ErrNoWebSocketFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, s.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		deleteItems = append(deleteItems, deleteItem{ID: wsID, WorkspaceID: workspaceID})
	}

	// ACT
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wsTx := s.ws.TX(tx)
	for _, item := range deleteItems {
		if err := wsTx.Delete(ctx, item.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range deleteItems {
		s.wsStream.Publish(WebSocketTopic{WorkspaceID: item.WorkspaceID}, WebSocketEvent{
			Type: eventTypeDelete,
			WebSocket: &apiv1.WebSocket{
				WebsocketId: item.ID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *WebSocketRPC) WebSocketHeaderInsert(ctx context.Context, req *connect.Request[apiv1.WebSocketHeaderInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	// FETCH + CHECK
	items := make([]mwebsocket.WebSocketHeader, 0, len(req.Msg.Items))
	workspaceIDs := make([]idwrap.IDWrap, 0, len(req.Msg.Items))
	now := time.Now().Unix()
	for _, item := range req.Msg.Items {
		if len(item.GetWebsocketHeaderId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("websocket_header_id is required"))
		}
		headerID, err := idwrap.NewFromBytes(item.GetWebsocketHeaderId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		if len(item.GetWebsocketId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("websocket_id is required"))
		}
		wsID, err := idwrap.NewFromBytes(item.GetWebsocketId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := s.ws.GetWorkspaceID(ctx, wsID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, s.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		items = append(items, mwebsocket.WebSocketHeader{
			ID:           headerID,
			WebSocketID:  wsID,
			Key:          item.GetKey(),
			Value:        item.GetValue(),
			Enabled:      item.GetEnabled(),
			Description:  item.GetDescription(),
			DisplayOrder: item.GetOrder(),
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		workspaceIDs = append(workspaceIDs, workspaceID)
	}

	// ACT
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wshTx := s.wsh.TX(tx)
	for _, h := range items {
		if err := wshTx.Create(ctx, h); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for i, h := range items {
		s.wshStream.Publish(WebSocketHeaderTopic{WorkspaceID: workspaceIDs[i]}, WebSocketHeaderEvent{
			Type:            eventTypeInsert,
			WebSocketHeader: toAPIWebSocketHeader(h),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *WebSocketRPC) WebSocketHeaderUpdate(ctx context.Context, req *connect.Request[apiv1.WebSocketHeaderUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	// FETCH + CHECK
	updates := make([]mwebsocket.WebSocketHeader, 0, len(req.Msg.Items))
	updateWorkspaceIDs := make([]idwrap.IDWrap, 0, len(req.Msg.Items))
	for _, item := range req.Msg.Items {
		if len(item.GetWebsocketHeaderId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("websocket_header_id is required"))
		}
		headerID, err := idwrap.NewFromBytes(item.GetWebsocketHeaderId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existing, err := s.wsh.GetByID(ctx, headerID)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		workspaceID, err := s.ws.GetWorkspaceID(ctx, existing.WebSocketID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, s.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
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
		existing.UpdatedAt = time.Now().Unix()

		updates = append(updates, existing)
		updateWorkspaceIDs = append(updateWorkspaceIDs, workspaceID)
	}

	// ACT
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wshTx := s.wsh.TX(tx)
	for _, h := range updates {
		if err := wshTx.Update(ctx, h); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for i, h := range updates {
		s.wshStream.Publish(WebSocketHeaderTopic{WorkspaceID: updateWorkspaceIDs[i]}, WebSocketHeaderEvent{
			Type:            eventTypeUpdate,
			WebSocketHeader: toAPIWebSocketHeader(h),
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *WebSocketRPC) WebSocketHeaderDelete(ctx context.Context, req *connect.Request[apiv1.WebSocketHeaderDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one item must be provided"))
	}

	// FETCH + CHECK
	type headerDeleteItem struct {
		ID          idwrap.IDWrap
		WorkspaceID idwrap.IDWrap
	}
	headerDeleteItems := make([]headerDeleteItem, 0, len(req.Msg.Items))
	for _, item := range req.Msg.Items {
		if len(item.GetWebsocketHeaderId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("websocket_header_id is required"))
		}
		headerID, err := idwrap.NewFromBytes(item.GetWebsocketHeaderId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		existing, err := s.wsh.GetByID(ctx, headerID)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		workspaceID, err := s.ws.GetWorkspaceID(ctx, existing.WebSocketID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rpcErr := permcheck.CheckPerm(mwauth.CheckOwnerWorkspace(ctx, s.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		headerDeleteItems = append(headerDeleteItems, headerDeleteItem{ID: headerID, WorkspaceID: workspaceID})
	}

	// ACT
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)

	wshTx := s.wsh.TX(tx)
	for _, item := range headerDeleteItems {
		if err := wshTx.Delete(ctx, item.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range headerDeleteItems {
		s.wshStream.Publish(WebSocketHeaderTopic{WorkspaceID: item.WorkspaceID}, WebSocketHeaderEvent{
			Type: eventTypeDelete,
			WebSocketHeader: &apiv1.WebSocketHeader{
				WebsocketHeaderId: item.ID.Bytes(),
			},
		})
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func toAPIWebSocketHeader(h mwebsocket.WebSocketHeader) *apiv1.WebSocketHeader {
	return &apiv1.WebSocketHeader{
		WebsocketHeaderId: h.ID.Bytes(),
		WebsocketId:       h.WebSocketID.Bytes(),
		Key:               h.Key,
		Value:             h.Value,
		Enabled:           h.Enabled,
		Description:       h.Description,
		Order:             h.DisplayOrder,
	}
}

func stringPtr(s string) *string   { return &s }
func boolPtr(b bool) *bool         { return &b }
func float32Ptr(f float32) *float32 { return &f }

func webSocketSyncResponseFrom(evt WebSocketEvent) *apiv1.WebSocketSyncResponse {
	if evt.WebSocket == nil {
		return nil
	}

	switch evt.Type {
	case eventTypeInsert:
		msg := &apiv1.WebSocketSync{
			Value: &apiv1.WebSocketSync_ValueUnion{
				Kind: apiv1.WebSocketSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.WebSocketSyncInsert{
					WebsocketId: evt.WebSocket.WebsocketId,
					Name:        evt.WebSocket.Name,
					Url:         evt.WebSocket.Url,
				},
			},
		}
		return &apiv1.WebSocketSyncResponse{Items: []*apiv1.WebSocketSync{msg}}
	case eventTypeUpdate:
		msg := &apiv1.WebSocketSync{
			Value: &apiv1.WebSocketSync_ValueUnion{
				Kind: apiv1.WebSocketSync_ValueUnion_KIND_UPDATE,
				Update: &apiv1.WebSocketSyncUpdate{
					WebsocketId: evt.WebSocket.WebsocketId,
					Name:        stringPtr(evt.WebSocket.Name),
					Url:         stringPtr(evt.WebSocket.Url),
				},
			},
		}
		return &apiv1.WebSocketSyncResponse{Items: []*apiv1.WebSocketSync{msg}}
	case eventTypeDelete:
		msg := &apiv1.WebSocketSync{
			Value: &apiv1.WebSocketSync_ValueUnion{
				Kind: apiv1.WebSocketSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.WebSocketSyncDelete{
					WebsocketId: evt.WebSocket.WebsocketId,
				},
			},
		}
		return &apiv1.WebSocketSyncResponse{Items: []*apiv1.WebSocketSync{msg}}
	default:
		return nil
	}
}

func webSocketHeaderSyncResponseFrom(evt WebSocketHeaderEvent) *apiv1.WebSocketHeaderSyncResponse {
	if evt.WebSocketHeader == nil {
		return nil
	}

	switch evt.Type {
	case eventTypeInsert:
		msg := &apiv1.WebSocketHeaderSync{
			Value: &apiv1.WebSocketHeaderSync_ValueUnion{
				Kind: apiv1.WebSocketHeaderSync_ValueUnion_KIND_INSERT,
				Insert: &apiv1.WebSocketHeaderSyncInsert{
					WebsocketHeaderId: evt.WebSocketHeader.WebsocketHeaderId,
					WebsocketId:       evt.WebSocketHeader.WebsocketId,
					Key:               evt.WebSocketHeader.Key,
					Value:             evt.WebSocketHeader.Value,
					Enabled:           evt.WebSocketHeader.Enabled,
					Description:       evt.WebSocketHeader.Description,
					Order:             evt.WebSocketHeader.Order,
				},
			},
		}
		return &apiv1.WebSocketHeaderSyncResponse{Items: []*apiv1.WebSocketHeaderSync{msg}}
	case eventTypeUpdate:
		msg := &apiv1.WebSocketHeaderSync{
			Value: &apiv1.WebSocketHeaderSync_ValueUnion{
				Kind: apiv1.WebSocketHeaderSync_ValueUnion_KIND_UPDATE,
				Update: &apiv1.WebSocketHeaderSyncUpdate{
					WebsocketHeaderId: evt.WebSocketHeader.WebsocketHeaderId,
					WebsocketId:       evt.WebSocketHeader.WebsocketId,
					Key:               stringPtr(evt.WebSocketHeader.Key),
					Value:             stringPtr(evt.WebSocketHeader.Value),
					Enabled:           boolPtr(evt.WebSocketHeader.Enabled),
					Description:       stringPtr(evt.WebSocketHeader.Description),
					Order:             float32Ptr(evt.WebSocketHeader.Order),
				},
			},
		}
		return &apiv1.WebSocketHeaderSyncResponse{Items: []*apiv1.WebSocketHeaderSync{msg}}
	case eventTypeDelete:
		msg := &apiv1.WebSocketHeaderSync{
			Value: &apiv1.WebSocketHeaderSync_ValueUnion{
				Kind: apiv1.WebSocketHeaderSync_ValueUnion_KIND_DELETE,
				Delete: &apiv1.WebSocketHeaderSyncDelete{
					WebsocketHeaderId: evt.WebSocketHeader.WebsocketHeaderId,
				},
			},
		}
		return &apiv1.WebSocketHeaderSyncResponse{Items: []*apiv1.WebSocketHeaderSync{msg}}
	default:
		return nil
	}
}
