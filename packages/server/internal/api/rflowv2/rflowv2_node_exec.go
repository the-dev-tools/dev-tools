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

	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) NodeExecutionCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeExecutionCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeExecution, 0)

	for _, flow := range flows {
		// Get all nodes for this flow
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// For each node, get its executions
		for _, node := range nodes {
			executions, err := s.nes.ListNodeExecutionsByNodeID(ctx, node.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Serialize each execution
			for _, execution := range executions {
				items = append(items, serializeNodeExecution(execution))
			}
		}
	}

	return connect.NewResponse(&flowv1.NodeExecutionCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeExecutionSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeExecutionSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeExecutionSync(ctx, func(resp *flowv1.NodeExecutionSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeExecutionSync(
	ctx context.Context,
	send func(*flowv1.NodeExecutionSyncResponse) error,
) error {
	if s.executionStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("execution stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[ExecutionTopic, ExecutionEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[ExecutionTopic, ExecutionEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			// Get all nodes for this flow
			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			// For each node, get its executions
			for _, node := range nodes {
				executions, err := s.nes.ListNodeExecutionsByNodeID(ctx, node.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}

				// Create events for each execution
				for _, execution := range executions {
					serializedExecution := serializeNodeExecution(execution)
					events = append(events, eventstream.Event[ExecutionTopic, ExecutionEvent]{
						Topic: ExecutionTopic{FlowID: flow.ID},
						Payload: ExecutionEvent{
							Type:      executionEventInsert,
							FlowID:    flow.ID,
							Execution: serializedExecution,
						},
					})
				}
			}
		}

		return events, nil
	}

	filter := func(topic ExecutionTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.executionStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp, err := s.executionEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert execution event: %w", err))
			}
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

func (s *FlowServiceV2RPC) publishExecutionEvent(eventType string, execution mnodeexecution.NodeExecution, flowID idwrap.IDWrap) {
	if s.executionStream == nil {
		return
	}

	executionPB := serializeNodeExecution(execution)
	s.executionStream.Publish(ExecutionTopic{FlowID: flowID}, ExecutionEvent{
		Type:      eventType,
		FlowID:    flowID,
		Execution: executionPB,
	})
}

func (s *FlowServiceV2RPC) publishHttpResponseEvent(eventType string, response mhttp.HTTPResponse, workspaceID idwrap.IDWrap) {
	if s.httpResponseStream == nil {
		return
	}
	responsePB := converter.ToAPIHttpResponse(response)
	s.httpResponseStream.Publish(rhttp.HttpResponseTopic{WorkspaceID: workspaceID}, rhttp.HttpResponseEvent{
		Type:         eventType,
		HttpResponse: responsePB,
	})
}

func (s *FlowServiceV2RPC) publishHttpResponseHeaderEvent(eventType string, header mhttp.HTTPResponseHeader, workspaceID idwrap.IDWrap) {
	if s.httpResponseHeaderStream == nil {
		return
	}
	headerPB := converter.ToAPIHttpResponseHeader(header)
	s.httpResponseHeaderStream.Publish(rhttp.HttpResponseHeaderTopic{WorkspaceID: workspaceID}, rhttp.HttpResponseHeaderEvent{
		Type:               eventType,
		HttpResponseHeader: headerPB,
	})
}

func (s *FlowServiceV2RPC) publishHttpResponseAssertEvent(eventType string, assert mhttp.HTTPResponseAssert, workspaceID idwrap.IDWrap) {
	if s.httpResponseAssertStream == nil {
		return
	}
	assertPB := converter.ToAPIHttpResponseAssert(assert)
	s.httpResponseAssertStream.Publish(rhttp.HttpResponseAssertTopic{WorkspaceID: workspaceID}, rhttp.HttpResponseAssertEvent{
		Type:               eventType,
		HttpResponseAssert: assertPB,
	})
}

func (s *FlowServiceV2RPC) executionEventToSyncResponse(
	ctx context.Context,
	evt ExecutionEvent,
) (*flowv1.NodeExecutionSyncResponse, error) {
	if evt.Execution == nil {
		return nil, nil
	}

	var syncEvent *flowv1.NodeExecutionSync
	switch evt.Type {
	case executionEventInsert:
		syncEvent = &flowv1.NodeExecutionSync{
			Value: &flowv1.NodeExecutionSync_ValueUnion{
				Kind: flowv1.NodeExecutionSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeExecutionSyncInsert{
					NodeExecutionId: evt.Execution.NodeExecutionId,
					NodeId:          evt.Execution.NodeId,
					Name:            evt.Execution.Name,
					State:           evt.Execution.State,
				},
			},
		}

		// Add optional fields to INSERT event
		if evt.Execution.Error != nil {
			syncEvent.Value.GetInsert().Error = evt.Execution.Error
		}
		if evt.Execution.Input != nil {
			syncEvent.Value.GetInsert().Input = evt.Execution.Input
		}
		if evt.Execution.Output != nil {
			syncEvent.Value.GetInsert().Output = evt.Execution.Output
		}
		if evt.Execution.HttpResponseId != nil {
			syncEvent.Value.GetInsert().HttpResponseId = evt.Execution.HttpResponseId
		}
		if evt.Execution.CompletedAt != nil {
			syncEvent.Value.GetInsert().CompletedAt = evt.Execution.CompletedAt
		}

	case executionEventUpdate:
		syncEvent = &flowv1.NodeExecutionSync{
			Value: &flowv1.NodeExecutionSync_ValueUnion{
				Kind: flowv1.NodeExecutionSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeExecutionSyncUpdate{
					NodeExecutionId: evt.Execution.NodeExecutionId,
				},
			},
		}

		// Add optional fields to UPDATE event
		update := syncEvent.Value.GetUpdate()

		// Only include NodeId if it's being updated
		if evt.Execution.NodeId != nil {
			update.NodeId = evt.Execution.NodeId
		}

		// Only include Name if it's being updated
		if evt.Execution.Name != "" {
			update.Name = &evt.Execution.Name
		}

		// Only include State if it's being updated
		if evt.Execution.State != flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED {
			update.State = &evt.Execution.State
		}

		// Handle Error union
		if evt.Execution.Error != nil {
			update.Error = &flowv1.NodeExecutionSyncUpdate_ErrorUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_ErrorUnion_KIND_VALUE,
				Value: evt.Execution.Error,
			}
		}

		// Handle Input union
		if evt.Execution.Input != nil {
			update.Input = &flowv1.NodeExecutionSyncUpdate_InputUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_InputUnion_KIND_VALUE,
				Value: evt.Execution.Input,
			}
		}

		// Handle Output union
		if evt.Execution.Output != nil {
			update.Output = &flowv1.NodeExecutionSyncUpdate_OutputUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_OutputUnion_KIND_VALUE,
				Value: evt.Execution.Output,
			}
		}

		// Handle HttpResponseId union
		if evt.Execution.HttpResponseId != nil {
			update.HttpResponseId = &flowv1.NodeExecutionSyncUpdate_HttpResponseIdUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_HttpResponseIdUnion_KIND_VALUE,
				Value: evt.Execution.HttpResponseId,
			}
		}

		// Handle CompletedAt union
		if evt.Execution.CompletedAt != nil {
			update.CompletedAt = &flowv1.NodeExecutionSyncUpdate_CompletedAtUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_CompletedAtUnion_KIND_VALUE,
				Value: evt.Execution.CompletedAt,
			}
		}

	case executionEventDelete:
		syncEvent = &flowv1.NodeExecutionSync{
			Value: &flowv1.NodeExecutionSync_ValueUnion{
				Kind: flowv1.NodeExecutionSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeExecutionSyncDelete{
					NodeExecutionId: evt.Execution.NodeExecutionId,
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeExecutionSyncResponse{
		Items: []*flowv1.NodeExecutionSync{syncEvent},
	}, nil
}
