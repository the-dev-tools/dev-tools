//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"
)

// nodeWithFlow is a context carrier that pairs a node with its flow ID
// for topic-based event grouping.
type nodeWithFlow struct {
	node   mflow.Node
	flowID idwrap.IDWrap
}

// publishBulkNodeInsert publishes multiple node insert events in bulk.
// Groups nodes by flow ID and publishes all inserts for that flow.
func (s *FlowServiceV2RPC) publishBulkNodeInsert(
	topic NodeTopic,
	items []nodeWithFlow,
) {
	for _, item := range items {
		s.publishNodeEvent(nodeEventInsert, item.node)
	}
}

// publishBulkNodeUpdate publishes multiple node update events in bulk.
// Preserves patch information for each update to enable efficient frontend sync.
func (s *FlowServiceV2RPC) publishBulkNodeUpdate(
	topic NodeTopic,
	events []txutil.UpdateEvent[nodeWithFlow, patch.NodePatch],
) {
	for _, evt := range events {
		s.publishNodeEvent(nodeEventUpdate, evt.Item.node)
	}
}

// publishBulkNodeDelete publishes multiple node delete events in bulk.
// Groups nodes by flow ID and publishes all deletions for that flow.
func (s *FlowServiceV2RPC) publishBulkNodeDelete(
	topic NodeTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	for _, evt := range events {
		node := mflow.Node{
			ID:     evt.ID,
			FlowID: evt.WorkspaceID, // WorkspaceID field is reused for FlowID
		}
		s.publishNodeEvent(nodeEventDelete, node)
	}
}
