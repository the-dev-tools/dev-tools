//nolint:revive // exported
package rhttp

import (
	"the-dev-tools/server/internal/converter"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/patch"
	"the-dev-tools/server/pkg/txutil"
)

// bodyRawWithWorkspace is a context carrier that pairs a body raw with its workspace ID.
type bodyRawWithWorkspace struct {
	bodyRaw     mhttp.HTTPBodyRaw
	workspaceID idwrap.IDWrap
}

// publishBulkBodyRawInsert publishes multiple body raw insert events in bulk.
// Since HttpBodyRaw is singleton (one per HTTP entry), this groups by workspace
// and publishes all inserts for that workspace in a single event.
func (h *HttpServiceRPC) publishBulkBodyRawInsert(
	topic HttpBodyRawTopic,
	items []bodyRawWithWorkspace,
) {
	events := make([]HttpBodyRawEvent, len(items))
	for i, item := range items {
		events[i] = HttpBodyRawEvent{
			Type:        eventTypeInsert,
			IsDelta:     item.bodyRaw.IsDelta,
			HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(item.bodyRaw),
		}
	}
	h.streamers.HttpBodyRaw.Publish(topic, events...)
}

// publishBulkBodyRawUpdate publishes multiple body raw update events in bulk.
// Preserves patch information for each update to enable efficient frontend sync.
func (h *HttpServiceRPC) publishBulkBodyRawUpdate(
	topic HttpBodyRawTopic,
	events []txutil.UpdateEvent[bodyRawWithWorkspace, patch.HTTPBodyRawPatch],
) {
	bodyRawEvents := make([]HttpBodyRawEvent, len(events))
	for i, evt := range events {
		bodyRawEvents[i] = HttpBodyRawEvent{
			Type:        eventTypeUpdate,
			IsDelta:     evt.Item.bodyRaw.IsDelta,
			HttpBodyRaw: converter.ToAPIHttpBodyRawFromMHttp(evt.Item.bodyRaw),
			Patch:       evt.Patch,
		}
	}
	h.streamers.HttpBodyRaw.Publish(topic, bodyRawEvents...)
}
