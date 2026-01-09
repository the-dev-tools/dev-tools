// Package streamtest provides utilities for testing sync event publishing.
// It allows declarative specification of expected events and automatic verification.
package streamtest

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
)

// EventType represents the type of sync event (insert, update, delete).
type EventType string

const (
	Insert EventType = "insert"
	Update EventType = "update"
	Delete EventType = "delete"
)

// CountConstraint specifies how many events are expected.
type CountConstraint struct {
	min int
	max int // -1 means no upper limit
}

// Exactly expects exactly n events.
func Exactly(n int) CountConstraint {
	return CountConstraint{min: n, max: n}
}

// AtLeast expects at least n events.
func AtLeast(n int) CountConstraint {
	return CountConstraint{min: n, max: -1}
}

// AtMost expects at most n events.
func AtMost(n int) CountConstraint {
	return CountConstraint{min: 0, max: n}
}

// Between expects between min and max events (inclusive).
func Between(min, max int) CountConstraint {
	return CountConstraint{min: min, max: max}
}

// Any expects any number of events (including zero).
func Any() CountConstraint {
	return CountConstraint{min: 0, max: -1}
}

func (c CountConstraint) check(actual int) bool {
	if actual < c.min {
		return false
	}
	if c.max >= 0 && actual > c.max {
		return false
	}
	return true
}

func (c CountConstraint) String() string {
	if c.min == c.max {
		return fmt.Sprintf("exactly %d", c.min)
	}
	if c.max < 0 {
		return fmt.Sprintf("at least %d", c.min)
	}
	if c.min == 0 {
		return fmt.Sprintf("at most %d", c.max)
	}
	return fmt.Sprintf("between %d and %d", c.min, c.max)
}

// Expectation represents an expected event with optional matching criteria.
type Expectation[T any] struct {
	name      string
	eventType EventType
	count     CountConstraint
	matcher   func(T) bool
	getType   func(T) string // extracts the event type string from payload
	received  []T
	mu        sync.Mutex
}

// ExpectationResult contains the result of checking an expectation.
type ExpectationResult struct {
	Name     string
	Expected string
	Actual   int
	Passed   bool
	Details  string
}

// StreamSubscription holds a subscription that will be collected.
type StreamSubscription struct {
	name   string
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

// Verifier collects and verifies sync events against expectations.
type Verifier struct {
	t             *testing.T
	expectations  []expectationChecker
	subscriptions []StreamSubscription
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
}

// expectationChecker is an interface that allows us to store different typed expectations.
type expectationChecker interface {
	check() ExpectationResult
	getName() string
}

// New creates a new Verifier for testing sync events.
func New(t *testing.T) *Verifier {
	ctx, cancel := context.WithCancel(context.Background())
	return &Verifier{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Expect adds an expectation for events from a specific stream.
// The getType function extracts the event type string (e.g., "insert", "update", "delete") from the payload.
// The matcher function determines if a specific event matches this expectation.
func Expect[Topic any, Payload any](
	v *Verifier,
	name string,
	stream eventstream.SyncStreamer[Topic, Payload],
	eventType EventType,
	count CountConstraint,
	getType func(Payload) string,
	matcher func(Payload) bool,
) *Verifier {
	exp := &Expectation[Payload]{
		name:      name,
		eventType: eventType,
		count:     count,
		matcher:   matcher,
		getType:   getType,
		received:  make([]Payload, 0),
	}

	v.mu.Lock()
	v.expectations = append(v.expectations, exp)
	v.mu.Unlock()

	// Subscribe to the stream
	subscribe(v, name, stream, func(payload Payload) {
		// Check if this event matches our expectation
		if getType(payload) == string(eventType) && (matcher == nil || matcher(payload)) {
			exp.mu.Lock()
			exp.received = append(exp.received, payload)
			exp.mu.Unlock()
		}
	})

	return v
}

// subscribe sets up a subscription to a stream and collects events.
// This is a package-level function because Go methods can't have type parameters.
func subscribe[Topic any, Payload any](
	v *Verifier,
	name string,
	stream eventstream.SyncStreamer[Topic, Payload],
	handler func(Payload),
) {
	if stream == nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)

	subCtx, subCancel := context.WithCancel(v.ctx)

	// Use a channel to signal when subscription is ready
	ready := make(chan struct{})

	go func() {
		defer wg.Done()

		ch, err := stream.Subscribe(subCtx, func(topic Topic) bool {
			return true // Accept all topics
		})
		if err != nil {
			v.t.Logf("streamtest: failed to subscribe to %s: %v", name, err)
			close(ready)
			return
		}

		// Signal that we're ready to receive events
		close(ready)

		for {
			select {
			case evt, ok := <-ch:
				if !ok {
					return
				}
				handler(evt.Payload)
			case <-subCtx.Done():
				return
			}
		}
	}()

	// Wait for the subscription to be established
	<-ready

	v.mu.Lock()
	v.subscriptions = append(v.subscriptions, StreamSubscription{
		name:   name,
		cancel: subCancel,
		wg:     &wg,
	})
	v.mu.Unlock()
}

// WaitAndVerify waits for the specified duration, then verifies all expectations.
// Returns true if all expectations passed.
func (v *Verifier) WaitAndVerify(timeout time.Duration) bool {
	// Wait for events to arrive
	time.Sleep(timeout)

	// Cancel all subscriptions
	v.cancel()

	// Wait for all subscription goroutines to finish
	for _, sub := range v.subscriptions {
		sub.wg.Wait()
	}

	// Check all expectations
	allPassed := true
	var failures []string

	for _, exp := range v.expectations {
		result := exp.check()
		if !result.Passed {
			allPassed = false
			failures = append(failures, fmt.Sprintf("  - %s: expected %s, got %d%s",
				result.Name, result.Expected, result.Actual, result.Details))
		} else {
			v.t.Logf("streamtest: %s: OK (received %d)", result.Name, result.Actual)
		}
	}

	if !allPassed {
		v.t.Errorf("streamtest: expectations not met:\n%s", strings.Join(failures, "\n"))
	}

	return allPassed
}

// Verify immediately checks all expectations without waiting.
func (v *Verifier) Verify() bool {
	return v.WaitAndVerify(0)
}

// GetReceived returns the received events for a specific expectation.
// Useful for additional assertions after verification.
func GetReceived[T any](exp *Expectation[T]) []T {
	exp.mu.Lock()
	defer exp.mu.Unlock()
	result := make([]T, len(exp.received))
	copy(result, exp.received)
	return result
}

// check implements expectationChecker for Expectation.
func (e *Expectation[T]) check() ExpectationResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	actual := len(e.received)
	passed := e.count.check(actual)

	return ExpectationResult{
		Name:     e.name,
		Expected: e.count.String(),
		Actual:   actual,
		Passed:   passed,
	}
}

// getName implements expectationChecker for Expectation.
func (e *Expectation[T]) getName() string {
	return e.name
}
