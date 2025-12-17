package streamtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/eventstream/memory"
)

// TestEvent is a simple event type for testing.
type TestEvent struct {
	Type  string
	Value string
}

// TestTopic is a simple topic type for testing.
type TestTopic struct {
	ID string
}

func TestCountConstraint(t *testing.T) {
	tests := []struct {
		name       string
		constraint CountConstraint
		actual     int
		expected   bool
	}{
		{"Exactly(3) with 3", Exactly(3), 3, true},
		{"Exactly(3) with 2", Exactly(3), 2, false},
		{"Exactly(3) with 4", Exactly(3), 4, false},
		{"AtLeast(2) with 2", AtLeast(2), 2, true},
		{"AtLeast(2) with 3", AtLeast(2), 3, true},
		{"AtLeast(2) with 1", AtLeast(2), 1, false},
		{"AtMost(3) with 2", AtMost(3), 2, true},
		{"AtMost(3) with 3", AtMost(3), 3, true},
		{"AtMost(3) with 4", AtMost(3), 4, false},
		{"Between(2,4) with 2", Between(2, 4), 2, true},
		{"Between(2,4) with 3", Between(2, 4), 3, true},
		{"Between(2,4) with 4", Between(2, 4), 4, true},
		{"Between(2,4) with 1", Between(2, 4), 1, false},
		{"Between(2,4) with 5", Between(2, 4), 5, false},
		{"Any() with 0", Any(), 0, true},
		{"Any() with 100", Any(), 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.constraint.check(tt.actual)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountConstraintString(t *testing.T) {
	tests := []struct {
		constraint CountConstraint
		expected   string
	}{
		{Exactly(3), "exactly 3"},
		{AtLeast(2), "at least 2"},
		{AtMost(5), "at most 5"},
		{Between(2, 4), "between 2 and 4"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constraint.String())
		})
	}
}

func TestVerifier_BasicExpectation(t *testing.T) {
	stream := memory.NewInMemorySyncStreamer[TestTopic, TestEvent]()

	// Create verifier with expectation
	verifier := New(t)
	Expect(verifier, "TestEvent", stream, Insert, Exactly(2),
		func(e TestEvent) string { return e.Type },
		func(e TestEvent) bool { return true },
	)

	// Publish events
	stream.Publish(TestTopic{ID: "1"}, TestEvent{Type: "insert", Value: "a"})
	stream.Publish(TestTopic{ID: "2"}, TestEvent{Type: "insert", Value: "b"})

	// Verify
	passed := verifier.WaitAndVerify(100 * time.Millisecond)
	assert.True(t, passed)
}

func TestVerifier_ExpectationWithMatcher(t *testing.T) {
	stream := memory.NewInMemorySyncStreamer[TestTopic, TestEvent]()

	verifier := New(t)
	Expect(verifier, "TestEvent", stream, Insert, Exactly(1),
		func(e TestEvent) string { return e.Type },
		func(e TestEvent) bool { return e.Value == "special" },
	)

	// Publish events - only one matches
	stream.Publish(TestTopic{ID: "1"}, TestEvent{Type: "insert", Value: "normal"})
	stream.Publish(TestTopic{ID: "2"}, TestEvent{Type: "insert", Value: "special"})
	stream.Publish(TestTopic{ID: "3"}, TestEvent{Type: "insert", Value: "other"})

	passed := verifier.WaitAndVerify(100 * time.Millisecond)
	assert.True(t, passed)
}

func TestVerifier_MultipleExpectations(t *testing.T) {
	insertStream := memory.NewInMemorySyncStreamer[TestTopic, TestEvent]()
	updateStream := memory.NewInMemorySyncStreamer[TestTopic, TestEvent]()

	verifier := New(t)
	Expect(verifier, "InsertEvents", insertStream, Insert, AtLeast(2),
		func(e TestEvent) string { return e.Type },
		nil,
	)
	Expect(verifier, "UpdateEvents", updateStream, Update, Exactly(1),
		func(e TestEvent) string { return e.Type },
		nil,
	)

	// Publish events
	insertStream.Publish(TestTopic{ID: "1"}, TestEvent{Type: "insert", Value: "a"})
	insertStream.Publish(TestTopic{ID: "2"}, TestEvent{Type: "insert", Value: "b"})
	insertStream.Publish(TestTopic{ID: "3"}, TestEvent{Type: "insert", Value: "c"})
	updateStream.Publish(TestTopic{ID: "1"}, TestEvent{Type: "update", Value: "x"})

	passed := verifier.WaitAndVerify(100 * time.Millisecond)
	assert.True(t, passed)
}

func TestVerifier_FiltersByEventType(t *testing.T) {
	stream := memory.NewInMemorySyncStreamer[TestTopic, TestEvent]()

	verifier := New(t)
	Expect(verifier, "InsertOnly", stream, Insert, Exactly(2),
		func(e TestEvent) string { return e.Type },
		nil,
	)

	// Publish mixed events
	stream.Publish(TestTopic{ID: "1"}, TestEvent{Type: "insert", Value: "a"})
	stream.Publish(TestTopic{ID: "2"}, TestEvent{Type: "update", Value: "b"}) // Should be ignored
	stream.Publish(TestTopic{ID: "3"}, TestEvent{Type: "insert", Value: "c"})
	stream.Publish(TestTopic{ID: "4"}, TestEvent{Type: "delete", Value: "d"}) // Should be ignored

	passed := verifier.WaitAndVerify(100 * time.Millisecond)
	assert.True(t, passed)
}

func TestVerifier_FailsWhenExpectationNotMet(t *testing.T) {
	stream := memory.NewInMemorySyncStreamer[TestTopic, TestEvent]()

	// Use a mock testing.T to capture the error
	mockT := &testing.T{}

	verifier := New(mockT)
	Expect(verifier, "TestEvent", stream, Insert, Exactly(3),
		func(e TestEvent) string { return e.Type },
		nil,
	)

	// Only publish 2 events (expecting 3)
	stream.Publish(TestTopic{ID: "1"}, TestEvent{Type: "insert", Value: "a"})
	stream.Publish(TestTopic{ID: "2"}, TestEvent{Type: "insert", Value: "b"})

	passed := verifier.WaitAndVerify(100 * time.Millisecond)
	assert.False(t, passed)
}

func TestVerifier_NilStream(t *testing.T) {
	// Should not panic when stream is nil
	verifier := New(t)
	Expect[TestTopic, TestEvent](verifier, "NilStream", nil, Insert, Any(),
		func(e TestEvent) string { return e.Type },
		nil,
	)

	// Should pass since Any() accepts 0 events
	passed := verifier.WaitAndVerify(50 * time.Millisecond)
	assert.True(t, passed)
}

func TestGetReceived(t *testing.T) {
	stream := memory.NewInMemorySyncStreamer[TestTopic, TestEvent]()

	verifier := New(t)
	exp := &Expectation[TestEvent]{
		name:      "TestExp",
		eventType: Insert,
		count:     AtLeast(1),
		getType:   func(e TestEvent) string { return e.Type },
		matcher:   func(e TestEvent) bool { return true },
		received:  make([]TestEvent, 0),
	}

	verifier.expectations = append(verifier.expectations, exp)
	subscribe(verifier, "test", stream, func(payload TestEvent) {
		if payload.Type == "insert" {
			exp.mu.Lock()
			exp.received = append(exp.received, payload)
			exp.mu.Unlock()
		}
	})

	// Publish events
	stream.Publish(TestTopic{ID: "1"}, TestEvent{Type: "insert", Value: "first"})
	stream.Publish(TestTopic{ID: "2"}, TestEvent{Type: "insert", Value: "second"})

	verifier.WaitAndVerify(100 * time.Millisecond)

	received := GetReceived(exp)
	require.Len(t, received, 2)
	assert.Equal(t, "first", received[0].Value)
	assert.Equal(t, "second", received[1].Value)
}
