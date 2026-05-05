package mutation

// Publisher handles automatic event publishing after commit.
// Implementations route events to the appropriate streamers based on entity type.
type Publisher interface {
	// PublishAll publishes all events after a successful commit.
	// Called automatically by Context.Commit() if a publisher is configured.
	PublishAll(events []Event)
}

// MultiPublisher fans an event slice out to several publishers in order.
// Each underlying publisher's switch typically ignores entity types it
// doesn't handle, so combining domain-specific publishers (flow + http +
// graphql, …) just works without a central dispatch table.
type MultiPublisher []Publisher

// PublishAll forwards events to every publisher in the slice.
func (m MultiPublisher) PublishAll(events []Event) {
	for _, p := range m {
		if p == nil {
			continue
		}
		p.PublishAll(events)
	}
}
