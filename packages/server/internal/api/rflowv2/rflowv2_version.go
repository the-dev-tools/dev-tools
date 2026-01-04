//nolint:revive // exported
package rflowv2

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"sync"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

func (s *FlowServiceV2RPC) FlowVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVersionCollectionResponse], error) {
	// Get all accessible flows
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all versions from all flows
	var allVersions []*flowv1.FlowVersion
	for _, flow := range flows {
		versions, err := s.fs.GetFlowsByVersionParentID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, version := range versions {
			allVersions = append(allVersions, &flowv1.FlowVersion{
				FlowVersionId: version.ID.Bytes(),
				FlowId:        flow.ID.Bytes(),
			})
		}
	}

	// Sort by flow version ID for consistent ordering
	sort.Slice(allVersions, func(i, j int) bool {
		return bytes.Compare(allVersions[i].GetFlowVersionId(), allVersions[j].GetFlowVersionId()) < 0
	})

	return connect.NewResponse(&flowv1.FlowVersionCollectionResponse{Items: allVersions}), nil
}

func (s *FlowServiceV2RPC) FlowVersionSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowVersionSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamFlowVersionSync(ctx, func(resp *flowv1.FlowVersionSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamFlowVersionSync(
	ctx context.Context,
	send func(*flowv1.FlowVersionSyncResponse) error,
) error {
	if s.versionStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("flow version stream not configured"))
	}

	var flowSet sync.Map

	filter := func(topic FlowVersionTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.versionStream.Subscribe(ctx, filter)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := flowVersionEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishFlowVersionEvent(eventType string, flow mflow.Flow) {
	if s.versionStream == nil {
		return
	}
	if flow.VersionParentID == nil {
		return
	}
	parent := *flow.VersionParentID
	s.versionStream.Publish(FlowVersionTopic{FlowID: parent}, FlowVersionEvent{
		Type:      eventType,
		FlowID:    parent,
		VersionID: flow.ID,
	})
}

func flowVersionEventToSyncResponse(evt FlowVersionEvent) *flowv1.FlowVersionSyncResponse {
	if evt.VersionID == (idwrap.IDWrap{}) {
		return nil
	}

	switch evt.Type {
	case flowVersionEventInsert:
		insert := &flowv1.FlowVersionSyncInsert{
			FlowVersionId: evt.VersionID.Bytes(),
			FlowId:        evt.FlowID.Bytes(),
		}
		return &flowv1.FlowVersionSyncResponse{
			Items: []*flowv1.FlowVersionSync{
				{
					Value: &flowv1.FlowVersionSync_ValueUnion{
						Kind:   flowv1.FlowVersionSync_ValueUnion_KIND_INSERT,
						Insert: insert,
					},
				},
			},
		}
	case flowVersionEventUpdate:
		update := &flowv1.FlowVersionSyncUpdate{
			FlowVersionId: evt.VersionID.Bytes(),
		}
		if evt.FlowID != (idwrap.IDWrap{}) {
			update.FlowId = evt.FlowID.Bytes()
		}
		return &flowv1.FlowVersionSyncResponse{
			Items: []*flowv1.FlowVersionSync{
				{
					Value: &flowv1.FlowVersionSync_ValueUnion{
						Kind:   flowv1.FlowVersionSync_ValueUnion_KIND_UPDATE,
						Update: update,
					},
				},
			},
		}
	case flowVersionEventDelete:
		return &flowv1.FlowVersionSyncResponse{
			Items: []*flowv1.FlowVersionSync{
				{
					Value: &flowv1.FlowVersionSync_ValueUnion{
						Kind: flowv1.FlowVersionSync_ValueUnion_KIND_DELETE,
						Delete: &flowv1.FlowVersionSyncDelete{
							FlowVersionId: evt.VersionID.Bytes(),
						},
					},
				},
			},
		}
	default:
		return nil
	}
}
