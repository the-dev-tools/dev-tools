//nolint:revive // exported
package rflowv2

import (
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"
)

// variableWithFlow is a context carrier that pairs a flow variable with its flow ID.
type variableWithFlow struct {
	variable mflow.FlowVariable
	flowID   idwrap.IDWrap
}

// publishBulkFlowVariableInsert publishes multiple flow variable insert events in bulk.
// Groups variables by flow ID and publishes all inserts for that flow in a single event batch.
func (s *FlowServiceV2RPC) publishBulkFlowVariableInsert(
	topic FlowVariableTopic,
	items []variableWithFlow,
) {
	for _, item := range items {
		s.publishFlowVariableEvent(flowVarEventInsert, item.variable)
	}
}

// publishBulkFlowVariableUpdate publishes multiple flow variable update events in bulk.
// Preserves patch information for each update to enable efficient frontend sync.
func (s *FlowServiceV2RPC) publishBulkFlowVariableUpdate(
	topic FlowVariableTopic,
	events []txutil.UpdateEvent[variableWithFlow, patch.FlowVariablePatch],
) {
	for _, evt := range events {
		s.publishFlowVariableEvent(flowVarEventUpdate, evt.Item.variable)
	}
}

// publishBulkFlowVariableDelete publishes multiple flow variable delete events in bulk.
// Groups variables by flow ID and publishes all deletions for that flow.
func (s *FlowServiceV2RPC) publishBulkFlowVariableDelete(
	topic FlowVariableTopic,
	events []txutil.DeleteEvent[idwrap.IDWrap],
) {
	for _, evt := range events {
		variable := mflow.FlowVariable{
			ID:     evt.ID,
			FlowID: evt.WorkspaceID, // WorkspaceID field is reused for FlowID
		}
		s.publishFlowVariableEvent(flowVarEventDelete, variable)
	}
}