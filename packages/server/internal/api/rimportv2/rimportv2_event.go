package rimportv2

import (
	"context"

	"the-dev-tools/server/internal/api/rfile"
	"the-dev-tools/server/internal/api/rflowv2"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/converter"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
)

// publishEvents publishes real-time sync events for imported entities
func (h *ImportV2RPC) publishEvents(ctx context.Context, results *ImportResults) {
	// Publish Flow event FIRST if present
	if results.Flow != nil {
		flowPB := &flowv1.Flow{
			FlowId: results.Flow.ID.Bytes(),
			Name:   results.Flow.Name,
		}
		if results.Flow.Duration != 0 {
			d := results.Flow.Duration
			flowPB.Duration = &d
		}

		h.FlowStream.Publish(rflowv2.FlowTopic{WorkspaceID: results.Flow.WorkspaceID}, rflowv2.FlowEvent{
			Type: "insert",
			Flow: flowPB,
		})

		// Publish Nodes events
		for _, node := range results.Nodes {
			nodePB := &flowv1.Node{
				NodeId: node.ID.Bytes(),
				FlowId: node.FlowID.Bytes(),
				Name:   node.Name,
				Kind:   converter.ToAPINodeKind(node.NodeKind),
				Position: &flowv1.Position{
					X: float32(node.PositionX),
					Y: float32(node.PositionY),
				},
			}
			h.NodeStream.Publish(rflowv2.NodeTopic{FlowID: node.FlowID}, rflowv2.NodeEvent{
				Type: "insert",
				Node: nodePB,
			})
		}

		// Publish Edges events
		for _, edge := range results.Edges {
			edgePB := &flowv1.Edge{
				EdgeId:       edge.ID.Bytes(),
				FlowId:       edge.FlowID.Bytes(),
				SourceId:     edge.SourceID.Bytes(),
				TargetId:     edge.TargetID.Bytes(),
				SourceHandle: flowv1.HandleKind(edge.SourceHandler),
				Kind:         flowv1.EdgeKind(edge.Kind),
			}
			h.EdgeStream.Publish(rflowv2.EdgeTopic{FlowID: edge.FlowID}, rflowv2.EdgeEvent{
				Type: "insert",
				Edge: edgePB,
			})
		}

		// Publish NoOpNodes events
		for _, noOpNode := range results.NoOpNodes {
			noOpPB := &flowv1.NodeNoOp{
				NodeId: noOpNode.FlowNodeID.Bytes(),
				Kind:   converter.ToAPINodeNoOpKind(noOpNode.Type),
			}

			h.NoopStream.Publish(rflowv2.NoOpTopic{FlowID: results.Flow.ID}, rflowv2.NoOpEvent{
				Type:   "insert",
				FlowID: results.Flow.ID,
				Node:   noOpPB,
			})
		}
	}
	// Publish HTTP events
	for _, httpReq := range results.HTTPReqs {
		h.HttpStream.Publish(rhttp.HttpTopic{WorkspaceID: httpReq.WorkspaceID}, rhttp.HttpEvent{
			Type:    "insert",
			IsDelta: httpReq.IsDelta,
			Http:    converter.ToAPIHttp(*httpReq),
		})
	}

	// Publish File events
	for _, file := range results.Files {
		// No longer skipping Flow files since we publish Flow event first now
		h.FileStream.Publish(rfile.FileTopic{WorkspaceID: file.WorkspaceID}, rfile.FileEvent{
			Type: "create",
			File: converter.ToAPIFile(*file),
			Name: file.Name,
		})
	}

	// Publish Header events
	for _, header := range results.HTTPHeaders {
		h.HttpHeaderStream.Publish(rhttp.HttpHeaderTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpHeaderEvent{
			Type:       "insert",
			IsDelta:    header.IsDelta,
			HttpHeader: converter.ToAPIHttpHeader(*header),
		})
	}

	// Publish SearchParam events
	for _, param := range results.HTTPSearchParams {
		h.HttpSearchParamStream.Publish(rhttp.HttpSearchParamTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpSearchParamEvent{
			Type:            "insert",
			IsDelta:         param.IsDelta,
			HttpSearchParam: converter.ToAPIHttpSearchParamFromMHttp(*param),
		})
	}

	// Publish BodyForm events
	for _, form := range results.HTTPBodyForms {
		h.HttpBodyFormStream.Publish(rhttp.HttpBodyFormTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyFormEvent{
			Type:         "insert",
			IsDelta:      form.IsDelta,
			HttpBodyForm: converter.ToAPIHttpBodyFormDataFromMHttp(*form),
		})
	}

	// Publish BodyUrlEncoded events
	for _, encoded := range results.HTTPBodyUrlEncoded {
		h.HttpBodyUrlEncodedStream.Publish(rhttp.HttpBodyUrlEncodedTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyUrlEncodedEvent{
			Type:               "insert",
			IsDelta:            encoded.IsDelta,
			HttpBodyUrlEncoded: converter.ToAPIHttpBodyUrlEncodedFromMHttp(*encoded),
		})
	}

	// Publish BodyRaw events
	for _, raw := range results.HTTPBodyRaws {
		h.HttpBodyRawStream.Publish(rhttp.HttpBodyRawTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpBodyRawEvent{
			Type:        "insert",
			IsDelta:     raw.IsDelta,
			HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(*raw),
		})
	}

	// Publish Assert events
	for _, assert := range results.HTTPAsserts {
		h.HttpAssertStream.Publish(rhttp.HttpAssertTopic{WorkspaceID: results.WorkspaceID}, rhttp.HttpAssertEvent{
			Type:       "insert",
			IsDelta:    assert.IsDelta,
			HttpAssert: converter.ToAPIHttpAssert(*assert),
		})
	}
}
