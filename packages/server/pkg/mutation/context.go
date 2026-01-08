package mutation

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

// Context manages a mutation transaction with automatic cascade event collection.
// Events are collected during mutations and can be retrieved after commit for publishing.
type Context struct {
	db        *sql.DB
	tx        *sql.Tx
	q         *gen.Queries
	events    []Event
	recorder  Recorder
	publisher Publisher
}

// Option configures a Context.
type Option func(*Context)

// WithPublisher sets the publisher for auto-publishing events after commit.
func WithPublisher(p Publisher) Option {
	return func(c *Context) {
		c.publisher = p
	}
}

// New creates a new mutation context.
func New(db *sql.DB, opts ...Option) *Context {
	c := &Context{
		db:       db,
		events:   make([]Event, 0, 64),
		recorder: newRecorder(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Begin starts a new transaction.
func (c *Context) Begin(ctx context.Context) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	c.tx = tx
	c.q = gen.New(tx)
	return nil
}

// Rollback aborts the transaction.
func (c *Context) Rollback() {
	if c.tx != nil {
		_ = c.tx.Rollback()
		c.tx = nil
	}
}

// Commit commits the transaction and records events for replay (dev only).
// If a publisher is configured, events are auto-published after successful commit.
// Otherwise, call Events() to get collected events for manual publishing.
func (c *Context) Commit(ctx context.Context) error {
	// Record for replay (noop in prod)
	if c.recorder != nil {
		_ = c.recorder.Record(c.events)
	}

	if err := c.tx.Commit(); err != nil {
		return err
	}
	c.tx = nil

	// Auto-publish all events after successful commit
	if c.publisher != nil {
		c.publisher.PublishAll(c.events)
	}

	return nil
}

// Queries returns the sqlc queries bound to the transaction.
func (c *Context) Queries() *gen.Queries {
	return c.q
}

// TX returns the underlying transaction.
func (c *Context) TX() *sql.Tx {
	return c.tx
}

// Events returns all collected events for publishing.
// Call this after Commit() to get events to publish.
func (c *Context) Events() []Event {
	return c.events
}

// track adds an event to the collection (internal use).
func (c *Context) track(evt Event) {
	c.events = append(c.events, evt)
}

// Track adds an event to the collection (public API for leaf entities).
// Use this for entities without cascade children (headers, params, etc.).
func (c *Context) Track(evt Event) {
	c.events = append(c.events, evt)
}

// Reset clears collected events (useful for reuse).
func (c *Context) Reset() {
	c.events = c.events[:0]
	c.tx = nil
	c.q = nil
}

// UpdateLastEventPayload updates the payload of the most recently tracked event.
func (c *Context) UpdateLastEventPayload(payload any) {
	if len(c.events) > 0 {
		c.events[len(c.events)-1].Payload = payload
	}
}
