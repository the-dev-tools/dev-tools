package mutation

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
)

// Context manages a mutation transaction with automatic cascade event collection.
// Events are collected during mutations and can be retrieved after commit for publishing.
// Use WithoutTX() option to skip transaction creation for publish-only mode.
type Context struct {
	db        *sql.DB
	tx        *sql.Tx
	q         *gen.Queries
	events    []Event
	recorder  Recorder
	publisher Publisher
	skipTx    bool // When true, no transaction is created - events are just collected and published
}

// Option configures a Context.
type Option func(*Context)

// WithPublisher sets the publisher for auto-publishing events after commit.
func WithPublisher(p Publisher) Option {
	return func(c *Context) {
		c.publisher = p
	}
}

// WithoutTX configures the context to skip transaction creation.
// In this mode, Begin() is a no-op and Commit() just publishes events without DB commit.
// Use this for high-frequency operations where TX overhead is too expensive.
func WithoutTX() Option {
	return func(c *Context) {
		c.skipTx = true
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
// If WithoutTX() was used, this is a no-op.
func (c *Context) Begin(ctx context.Context) error {
	if c.skipTx {
		return nil // No-op in TX-free mode
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	c.tx = tx
	c.q = gen.New(tx)
	return nil
}

// Rollback aborts the transaction.
// If WithoutTX() was used, this just clears collected events.
func (c *Context) Rollback() {
	if c.tx != nil {
		_ = c.tx.Rollback()
		c.tx = nil
	}
	// In TX-free mode, just clear events on rollback
	if c.skipTx {
		c.events = c.events[:0]
	}
}

// Commit commits the transaction and records events for replay (dev only).
// If a publisher is configured, events are auto-published after successful commit.
// If WithoutTX() was used, this just publishes events without DB commit.
// Otherwise, call Events() to get collected events for manual publishing.
func (c *Context) Commit(ctx context.Context) error {
	// Record for replay (noop in prod)
	if c.recorder != nil {
		_ = c.recorder.Record(c.events)
	}

	// Only commit TX if we have one (not in TX-free mode)
	if c.tx != nil {
		if err := c.tx.Commit(); err != nil {
			return err
		}
		c.tx = nil
	}

	// Auto-publish all events after successful commit (or immediately in TX-free mode)
	if c.publisher != nil {
		c.publisher.PublishAll(c.events)
	}

	return nil
}

// Queries returns the sqlc queries bound to the transaction.
// Returns nil in TX-free mode.
func (c *Context) Queries() *gen.Queries {
	return c.q
}

// TX returns the underlying transaction.
// Returns nil in TX-free mode.
func (c *Context) TX() *sql.Tx {
	return c.tx
}

// Events returns all collected events for publishing.
// Call this after Commit() to get events to publish.
func (c *Context) Events() []Event {
	return c.events
}

// IsTxFree returns true if the context is in TX-free mode.
func (c *Context) IsTxFree() bool {
	return c.skipTx
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

// Publish immediately publishes all tracked events without commit.
// Useful in TX-free mode for explicit publish timing.
func (c *Context) Publish() {
	if c.publisher != nil {
		c.publisher.PublishAll(c.events)
	}
}
