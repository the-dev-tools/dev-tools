//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) EdgeCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.EdgeCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var edgesPB []*flowv1.Edge

	for _, flow := range flows {
		edges, err := s.es.GetEdgesByFlowID(ctx, flow.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, e := range edges {
			edgesPB = append(edgesPB, serializeEdge(e))
		}
	}

	return connect.NewResponse(&flowv1.EdgeCollectionResponse{Items: edgesPB}), nil
}

func (s *FlowServiceV2RPC) EdgeInsert(ctx context.Context, req *connect.Request[flowv1.EdgeInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}
		if len(item.GetSourceId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source id is required"))
		}
		if len(item.GetTargetId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}
		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		sourceID, err := idwrap.NewFromBytes(item.GetSourceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid source id: %w", err))
		}
		// We don't strictly enforce node existence here to avoid race conditions with node creation.
		// The flow_edge table only has an FK to the flow table.

		targetID, err := idwrap.NewFromBytes(item.GetTargetId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target id: %w", err))
		}

		edgeID := idwrap.NewNow()
		if len(item.GetEdgeId()) != 0 {
			edgeID, err = idwrap.NewFromBytes(item.GetEdgeId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
			}
		}

		model := mflow.Edge{
			ID:            edgeID,
			FlowID:        flowID,
			SourceID:      sourceID,
			TargetID:      targetID,
			SourceHandler: convertHandle(item.GetSourceHandle()),
		}

		if err := s.es.CreateEdge(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishEdgeEvent(edgeEventInsert, model)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeUpdate(ctx context.Context, req *connect.Request[flowv1.EdgeUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		edgeID, err := idwrap.NewFromBytes(item.GetEdgeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
		}

		existing, err := s.ensureEdgeAccess(ctx, edgeID)
		if err != nil {
			return nil, err
		}

		if len(item.GetFlowId()) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow reassignment is not supported"))
		}

		if len(item.GetSourceId()) != 0 {
			sourceID, err := idwrap.NewFromBytes(item.GetSourceId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid source id: %w", err))
			}
			if _, err := s.ensureNodeAccess(ctx, sourceID); err != nil {
				return nil, err
			}
			existing.SourceID = sourceID
		}

		if len(item.GetTargetId()) != 0 {
			targetID, err := idwrap.NewFromBytes(item.GetTargetId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target id: %w", err))
			}
			if _, err := s.ensureNodeAccess(ctx, targetID); err != nil {
				return nil, err
			}
			existing.TargetID = targetID
		}

		if item.SourceHandle != nil {
			existing.SourceHandler = convertHandle(item.GetSourceHandle())
		}

		if err := s.es.UpdateEdge(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishEdgeEvent(edgeEventUpdate, *existing)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeDelete(ctx context.Context, req *connect.Request[flowv1.EdgeDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		edgeID, err := idwrap.NewFromBytes(item.GetEdgeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
		}

		existing, err := s.ensureEdgeAccess(ctx, edgeID)
		if err != nil {
			return nil, err
		}

		if err := s.es.DeleteEdge(ctx, edgeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if existing != nil {
			s.publishEdgeEvent(edgeEventDelete, *existing)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.EdgeSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamEdgeSync(ctx, func(resp *flowv1.EdgeSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamEdgeSync(
	ctx context.Context,
	send func(*flowv1.EdgeSyncResponse) error,
) error {
	if s.edgeStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("edge stream not configured"))
	}

	var flowSet sync.Map

	filter := func(topic EdgeTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.edgeStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := edgeEventToSyncResponse(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *FlowServiceV2RPC) publishEdgeEvent(eventType string, model mflow.Edge) {
	if s.edgeStream == nil {
		return
	}
	edgePB := serializeEdge(model)
	s.edgeStream.Publish(EdgeTopic{FlowID: model.FlowID}, EdgeEvent{
		Type:   eventType,
		FlowID: model.FlowID,
		Edge:   edgePB,
	})
}

func edgeEventToSyncResponse(evt EdgeEvent) *flowv1.EdgeSyncResponse {
	if evt.Edge == nil {
		return nil
	}

	edgePB := evt.Edge

	switch evt.Type {
	case edgeEventInsert:
		insert := &flowv1.EdgeSyncInsert{
			EdgeId:       edgePB.GetEdgeId(),
			FlowId:       edgePB.GetFlowId(),
			SourceId:     edgePB.GetSourceId(),
			TargetId:     edgePB.GetTargetId(),
			SourceHandle: edgePB.GetSourceHandle(),
		}
		return &flowv1.EdgeSyncResponse{
			Items: []*flowv1.EdgeSync{{
				Value: &flowv1.EdgeSync_ValueUnion{
					Kind:   flowv1.EdgeSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case edgeEventUpdate:
		update := &flowv1.EdgeSyncUpdate{
			EdgeId: edgePB.GetEdgeId(),
		}
		if flowID := edgePB.GetFlowId(); len(flowID) > 0 {
			update.FlowId = flowID
		}
		if sourceID := edgePB.GetSourceId(); len(sourceID) > 0 {
			update.SourceId = sourceID
		}
		if targetID := edgePB.GetTargetId(); len(targetID) > 0 {
			update.TargetId = targetID
		}
		if handle := edgePB.GetSourceHandle(); handle != flowv1.HandleKind_HANDLE_KIND_UNSPECIFIED {
			h := handle
			update.SourceHandle = &h
		}
		// Always include state to support resetting to UNSPECIFIED
		s := edgePB.GetState()
		update.State = &s
		return &flowv1.EdgeSyncResponse{
			Items: []*flowv1.EdgeSync{{
				Value: &flowv1.EdgeSync_ValueUnion{
					Kind:   flowv1.EdgeSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case edgeEventDelete:
		return &flowv1.EdgeSyncResponse{
			Items: []*flowv1.EdgeSync{{
				Value: &flowv1.EdgeSync_ValueUnion{
					Kind: flowv1.EdgeSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.EdgeSyncDelete{
						EdgeId: edgePB.GetEdgeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}
