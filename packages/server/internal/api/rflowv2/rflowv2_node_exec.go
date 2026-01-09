//nolint:revive // exported
package rflowv2

import (
	"context"
	"errors"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
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

	converter := func(events []ExecutionEvent) *flowv1.NodeExecutionSyncResponse {
		var items []*flowv1.NodeExecutionSync
		for _, evt := range events {
			resp, err := s.executionEventToSyncResponse(ctx, evt)
			if err != nil {
				s.logger.Error("failed to convert execution event", "error", err)
				continue
			}
			if resp != nil && len(resp.Items) > 0 {
				items = append(items, resp.Items...)
			}
		}
		if len(items) == 0 {
			return nil
		}
		return &flowv1.NodeExecutionSyncResponse{Items: items}
	}

	return eventstream.StreamToClient(
		ctx,
		s.executionStream,
		filter,
		converter,
		send,
		nil, // Use default batching options
	)
}

func (s *FlowServiceV2RPC) publishExecutionEvent(eventType string, execution mflow.NodeExecution, flowID idwrap.IDWrap) {
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
	case executionEventInsert, executionEventUpdate:
		// Use UPSERT for both insert and update events to ensure the frontend can handle out-of-order delivery
		// or missing initial state. This simplifies the client logic and makes it more robust.
		upsert := &flowv1.NodeExecutionSyncUpsert{
			NodeExecutionId: evt.Execution.NodeExecutionId,
			NodeId:          evt.Execution.NodeId,
			Name:            evt.Execution.Name,
			State:           evt.Execution.State,
		}

		if evt.Execution.Error != nil {
			upsert.Error = evt.Execution.Error
		}
		if evt.Execution.Input != nil {
			upsert.Input = evt.Execution.Input
		}
		if evt.Execution.Output != nil {
			upsert.Output = evt.Execution.Output
		}
		if evt.Execution.HttpResponseId != nil {
			upsert.HttpResponseId = evt.Execution.HttpResponseId
		}
		if evt.Execution.CompletedAt != nil {
			upsert.CompletedAt = evt.Execution.CompletedAt
		}

		syncEvent = &flowv1.NodeExecutionSync{
			Value: &flowv1.NodeExecutionSync_ValueUnion{
				Kind:   flowv1.NodeExecutionSync_ValueUnion_KIND_UPSERT,
				Upsert: upsert,
			},
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
