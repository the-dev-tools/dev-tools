package rcloudlink

import (
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rflowv2"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
)

// publishImport tells sync subscribers about the rows an Upsert or SetLocalDir created, after the
// commit, the way the workspace import does: files and flows on their streams, nodes, edges and
// HTTP requests through the flow and HTTP services' publishers. Deleted rows were published by
// the mutation context on commit.
func (h *CloudLinkRPC) publishImport(workspaceID idwrap.IDWrap, folders []mfile.File, bundle *ioworkspace.WorkspaceBundle) {
	files := folders
	if bundle != nil {
		files = append(append([]mfile.File{}, folders...), bundle.Files...)
	}
	if h.fileStream != nil {
		for _, f := range files {
			h.fileStream.Publish(rfile.FileTopic{WorkspaceID: workspaceID}, rfile.FileEvent{
				Type: "create",
				File: converter.ToAPIFile(f),
				Name: f.Name,
			})
		}
	}
	if bundle == nil {
		return
	}

	if h.flowStream != nil {
		for _, f := range bundle.Flows {
			h.flowStream.Publish(rflowv2.FlowTopic{WorkspaceID: workspaceID}, rflowv2.FlowEvent{
				Type: "insert",
				Flow: &flowv1.Flow{FlowId: f.ID.Bytes(), WorkspaceId: workspaceID.Bytes(), Name: f.Name},
			})
		}
	}

	if h.publisher != nil {
		h.publisher.PublishAll(importEvents(workspaceID, bundle))
	}
}

// importEvents lists insert events for an imported bundle's nodes, edges, variables, node
// settings and HTTP requests.
func importEvents(wsID idwrap.IDWrap, b *ioworkspace.WorkspaceBundle) []mutation.Event {
	nodeFlow := make(map[idwrap.IDWrap]idwrap.IDWrap, len(b.FlowNodes))
	for _, n := range b.FlowNodes {
		nodeFlow[n.ID] = n.FlowID
	}

	events := make([]mutation.Event, 0, len(b.FlowNodes)*2+len(b.FlowEdges)+len(b.HTTPRequests)*4)
	add := func(entity mutation.EntityType, id, parent idwrap.IDWrap, payload any) {
		events = append(events, mutation.Event{
			Entity: entity, Op: mutation.OpInsert, ID: id, WorkspaceID: wsID, ParentID: parent, Payload: payload,
		})
	}

	for _, n := range b.FlowNodes {
		add(mutation.EntityFlowNode, n.ID, n.FlowID, n)
	}
	for _, e := range b.FlowEdges {
		add(mutation.EntityFlowEdge, e.ID, e.FlowID, e)
	}
	for _, v := range b.FlowVariables {
		add(mutation.EntityFlowVariable, v.ID, v.FlowID, v)
	}
	for _, n := range b.FlowForNodes {
		add(mutation.EntityFlowNodeFor, n.FlowNodeID, nodeFlow[n.FlowNodeID], n)
	}
	for _, n := range b.FlowForEachNodes {
		add(mutation.EntityFlowNodeForEach, n.FlowNodeID, nodeFlow[n.FlowNodeID], n)
	}
	for _, n := range b.FlowConditionNodes {
		add(mutation.EntityFlowNodeCondition, n.FlowNodeID, nodeFlow[n.FlowNodeID], n)
	}
	for _, n := range b.FlowJSNodes {
		add(mutation.EntityFlowNodeJS, n.FlowNodeID, nodeFlow[n.FlowNodeID], n)
	}
	for _, n := range b.FlowAINodes {
		add(mutation.EntityFlowNodeAI, n.FlowNodeID, nodeFlow[n.FlowNodeID], n)
	}
	for _, n := range b.FlowAIProviderNodes {
		add(mutation.EntityFlowNodeAiProvider, n.FlowNodeID, nodeFlow[n.FlowNodeID], n)
	}
	for _, n := range b.FlowAIMemoryNodes {
		add(mutation.EntityFlowNodeMemory, n.FlowNodeID, nodeFlow[n.FlowNodeID], n)
	}

	for _, r := range b.HTTPRequests {
		events = append(events, mutation.Event{
			Entity: mutation.EntityHTTP, Op: mutation.OpInsert, ID: r.ID, WorkspaceID: wsID, IsDelta: r.IsDelta, Payload: r,
		})
	}
	for _, x := range b.HTTPHeaders {
		add(mutation.EntityHTTPHeader, x.ID, x.HttpID, x)
	}
	for _, x := range b.HTTPSearchParams {
		add(mutation.EntityHTTPParam, x.ID, x.HttpID, x)
	}
	for _, x := range b.HTTPBodyForms {
		add(mutation.EntityHTTPBodyForm, x.ID, x.HttpID, x)
	}
	for _, x := range b.HTTPBodyUrlencoded {
		add(mutation.EntityHTTPBodyURL, x.ID, x.HttpID, x)
	}
	for _, x := range b.HTTPBodyRaw {
		add(mutation.EntityHTTPBodyRaw, x.ID, x.HttpID, x)
	}
	for _, x := range b.HTTPAsserts {
		add(mutation.EntityHTTPAssert, x.ID, x.HttpID, x)
	}
	return events
}
