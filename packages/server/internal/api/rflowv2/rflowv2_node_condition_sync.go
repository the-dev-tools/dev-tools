//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/txutil"
)

// nodeConditionWithFlow is a context carrier struct that pairs a NodeIf
// with its flow ID for topic-based event grouping.
type nodeConditionWithFlow struct {
	nodeIf   mflow.NodeIf
	flowID   idwrap.IDWrap
	baseNode *mflow.Node
}

// nodeConditionPatch is an empty patch type for NodeCondition updates.
// NodeCondition operations don't use complex patch types since they replace
// the entire sub-node config struct.
type nodeConditionPatch struct{}

// publishBulkNodeConditionInsert publishes node events for bulk NodeCondition inserts.
func (s *FlowServiceV2RPC) publishBulkNodeConditionInsert(
	topic NodeTopic,
	items []nodeConditionWithFlow,
) {
	for _, item := range items {
		if item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *item.baseNode)
		}
	}
}

// publishBulkNodeConditionUpdate publishes node events for bulk NodeCondition updates.
func (s *FlowServiceV2RPC) publishBulkNodeConditionUpdate(
	topic NodeTopic,
	events []txutil.UpdateEvent[nodeConditionWithFlow, nodeConditionPatch],
) {
	for _, evt := range events {
		if evt.Item.baseNode != nil {
			s.publishNodeEvent(nodeEventUpdate, *evt.Item.baseNode)
		}
	}
}
