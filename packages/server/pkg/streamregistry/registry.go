// Package streamregistry provides a centralized registry for routing mutation events
// to their appropriate event streams. It implements mutation.Publisher to enable
// automatic event publishing after successful transaction commits.
package streamregistry

import "the-dev-tools/server/pkg/mutation"

// Handler publishes a mutation event to the appropriate stream.
// Each handler has closure access to the concrete streamer it needs.
type Handler func(evt mutation.Event)

// Registry maps entity types to stream handlers.
// It implements mutation.Publisher for automatic publishing on commit.
type Registry struct {
	handlers map[mutation.EntityType]Handler
}

// New creates an empty stream registry.
func New() *Registry {
	return &Registry{
		handlers: make(map[mutation.EntityType]Handler),
	}
}

// Register adds a handler for an entity type.
// Handlers should be registered at startup with closure access to streamers.
func (r *Registry) Register(entity mutation.EntityType, handler Handler) {
	r.handlers[entity] = handler
}

// PublishAll implements mutation.Publisher.
// Called automatically by mutation.Context.Commit() if configured.
func (r *Registry) PublishAll(events []mutation.Event) {
	for _, evt := range events {
		if handler, ok := r.handlers[evt.Entity]; ok {
			handler(evt)
		}
		// Silently skip unregistered entities - may be intentional
	}
}
