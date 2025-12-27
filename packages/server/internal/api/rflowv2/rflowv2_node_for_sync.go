//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/txutil"
)

// nodeForWithFlow is a context carrier struct that pairs a NodeFor
// with its flow ID for topic-based event grouping.
type nodeForWithFlow struct {
	nodeFor mflow.NodeFor
	flowID  idwrap.IDWrap
}

// nodeForPatch is an empty patch type for NodeFor updates.
// NodeFor operations don't use complex patch types since they replace
// the entire sub-node config struct.
type nodeForPatch struct{}

// publishBulkNodeForInsert publishes ForEvent events for bulk NodeFor inserts.
func (s *FlowServiceV2RPC) publishBulkNodeForInsert(
	topic ForTopic,
	items []nodeForWithFlow,
) {
	for _, item := range items {
		s.publishForEvent(forEventInsert, item.flowID, item.nodeFor)
	}
}

// publishBulkNodeForUpdate publishes ForEvent events for bulk NodeFor updates.
func (s *FlowServiceV2RPC) publishBulkNodeForUpdate(
	topic ForTopic,
	events []txutil.UpdateEvent[nodeForWithFlow, nodeForPatch],
) {
	for _, evt := range events {
		s.publishForEvent(forEventUpdate, evt.Item.flowID, evt.Item.nodeFor)
	}
}
