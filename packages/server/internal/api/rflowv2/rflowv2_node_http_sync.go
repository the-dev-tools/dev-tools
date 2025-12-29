//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/txutil"
)

// nodeHttpWithFlow is a context carrier struct that pairs a NodeRequest
// with its flow ID for topic-based event grouping.
type nodeHttpWithFlow struct {
	nodeRequest mflow.NodeRequest
	flowID      idwrap.IDWrap
	baseNode    *mflow.Node // Base node model for publishing events
}

// nodeHttpPatch is an empty patch type for NodeHttp updates.
// NodeHttp operations don't use complex patch types since they replace
// the entire sub-node config struct.
type nodeHttpPatch struct{}

// publishBulkNodeHttpInsert publishes base node events for bulk NodeHttp inserts.
func (s *FlowServiceV2RPC) publishBulkNodeHttpInsert(
	topic NodeTopic,
	items []nodeHttpWithFlow,
) {
	for _, item := range items {
		if item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *item.baseNode)
		}
	}
}

// publishBulkNodeHttpUpdate publishes base node events for bulk NodeHttp updates.
// NodeHttp sync relies on base node events (not NodeHttp-specific events).
func (s *FlowServiceV2RPC) publishBulkNodeHttpUpdate(
	topic NodeTopic,
	events []txutil.UpdateEvent[nodeHttpWithFlow, nodeHttpPatch],
) {
	for _, evt := range events {
		if evt.Item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *evt.Item.baseNode)
		}
	}
}
