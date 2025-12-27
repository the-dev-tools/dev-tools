//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/txutil"
)

// nodeForEachWithFlow is a context carrier struct that pairs a NodeForEach
// with its flow ID for topic-based event grouping.
type nodeForEachWithFlow struct {
	nodeForEach mflow.NodeForEach
	flowID      idwrap.IDWrap
	baseNode    *mflow.Node
}

// nodeForEachPatch is an empty patch type for NodeForEach updates.
// NodeForEach operations don't use complex patch types since they replace
// the entire sub-node config struct.
type nodeForEachPatch struct{}

// publishBulkNodeForEachInsert publishes node events for bulk NodeForEach inserts.
func (s *FlowServiceV2RPC) publishBulkNodeForEachInsert(
	topic NodeTopic,
	items []nodeForEachWithFlow,
) {
	for _, item := range items {
		if item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *item.baseNode)
		}
	}
}

// publishBulkNodeForEachUpdate publishes node events for bulk NodeForEach updates.
func (s *FlowServiceV2RPC) publishBulkNodeForEachUpdate(
	topic NodeTopic,
	events []txutil.UpdateEvent[nodeForEachWithFlow, nodeForEachPatch],
) {
	for _, evt := range events {
		if evt.Item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *evt.Item.baseNode)
		}
	}
}
