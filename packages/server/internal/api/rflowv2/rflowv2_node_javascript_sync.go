//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/txutil"
)

// nodeJsWithFlow is a context carrier struct that pairs a NodeJS
// with its flow ID for topic-based event grouping.
type nodeJsWithFlow struct {
	nodeJS   mflow.NodeJS
	flowID   idwrap.IDWrap
	baseNode *mflow.Node
}

// nodeJsPatch is an empty patch type for NodeJS updates.
// NodeJS operations don't use complex patch types since they replace
// the entire sub-node config struct.
type nodeJsPatch struct{}

// publishBulkNodeJsInsert publishes node events for bulk NodeJS inserts.
func (s *FlowServiceV2RPC) publishBulkNodeJsInsert(
	topic NodeTopic,
	items []nodeJsWithFlow,
) {
	for _, item := range items {
		if item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *item.baseNode)
		}
	}
}

// publishBulkNodeJsUpdate publishes node events for bulk NodeJS updates.
func (s *FlowServiceV2RPC) publishBulkNodeJsUpdate(
	topic NodeTopic,
	events []txutil.UpdateEvent[nodeJsWithFlow, nodeJsPatch],
) {
	for _, evt := range events {
		if evt.Item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *evt.Item.baseNode)
		}
	}
}
