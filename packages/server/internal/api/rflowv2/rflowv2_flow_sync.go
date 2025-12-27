//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"
)

// flowWithWorkspace is a context carrier that pairs a flow with its workspace ID
// for topic-based event grouping.
type flowWithWorkspace struct {
	flow        mflow.Flow
	workspaceID idwrap.IDWrap
}

// flowNodePair tracks both a flow and its associated start node for bulk insert.
// FlowInsert creates both a Flow and a "Start" Node, so we need to publish events
// for both entities after transaction commits.
type flowNodePair struct {
	flow        mflow.Flow
	startNode   mflow.Node
	workspaceID idwrap.IDWrap
}

// publishBulkFlowInsert publishes multiple flow insert events in bulk.
// Also publishes start node events for flows that were created with a start node.
// Groups flows by workspace ID and publishes all inserts for that workspace.
func (s *FlowServiceV2RPC) publishBulkFlowInsert(
	topic FlowTopic,
	items []flowNodePair,
) {
	for _, item := range items {
		s.publishFlowEvent(flowEventInsert, item.flow)
		s.publishNodeEvent(nodeEventInsert, item.startNode)
	}
}

// publishBulkFlowUpdate publishes multiple flow update events in bulk.
// Preserves patch information for each update to enable efficient frontend sync.
// Also publishes FlowVersion events conditionally if the flow has a VersionParentID.
func (s *FlowServiceV2RPC) publishBulkFlowUpdate(
	topic FlowTopic,
	events []txutil.UpdateEvent[flowWithWorkspace, patch.FlowPatch],
) {
	for _, evt := range events {
		s.publishFlowEvent(flowEventUpdate, evt.Item.flow)
		if evt.Item.flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventUpdate, evt.Item.flow)
		}
	}
}

// publishBulkFlowDelete publishes multiple flow delete events in bulk.
// Groups flows by workspace ID and publishes all deletions for that workspace.
// Also publishes FlowVersion events conditionally if the flow has a VersionParentID.
func (s *FlowServiceV2RPC) publishBulkFlowDelete(
	topic FlowTopic,
	items []flowWithWorkspace,
) {
	for _, item := range items {
		s.publishFlowEvent(flowEventDelete, item.flow)
		if item.flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventDelete, item.flow)
		}
	}
}
