//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"
)

// edgeWithFlow is a context carrier that pairs an edge with its flow ID.
type edgeWithFlow struct {
	edge   mflow.Edge
	flowID idwrap.IDWrap
}

// publishBulkEdgeInsert publishes multiple edge insert events in bulk.
// Groups edges by flow ID and publishes all inserts for that flow in a single event batch.
func (s *FlowServiceV2RPC) publishBulkEdgeInsert(
	topic EdgeTopic,
	items []edgeWithFlow,
) {
	for _, item := range items {
		s.publishEdgeEvent(edgeEventInsert, item.edge)
	}
}

// publishBulkEdgeUpdate publishes multiple edge update events in bulk.
// Preserves patch information for each update to enable efficient frontend sync.
func (s *FlowServiceV2RPC) publishBulkEdgeUpdate(
	topic EdgeTopic,
	events []txutil.UpdateEvent[edgeWithFlow, patch.EdgePatch],
) {
	for _, evt := range events {
		s.publishEdgeEvent(edgeEventUpdate, evt.Item.edge)
	}
}