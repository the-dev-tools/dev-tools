package mutation

// Publisher handles automatic event publishing after commit.
// Implementations route events to the appropriate streamers based on entity type.
type Publisher interface {
	// PublishAll publishes all events after a successful commit.
	// Called automatically by Context.Commit() if a publisher is configured.
	PublishAll(events []Event)
}
