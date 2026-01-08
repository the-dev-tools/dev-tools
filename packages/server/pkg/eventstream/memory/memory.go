//nolint:revive // exported
package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
)

// defaultSubscriberBuffer is set to 4096 to handle bulk operations (like HAR import)
// where thousands of events might be published in a short burst.
// A small buffer (e.g., 32) causes events to be dropped in non-blocking Publish calls.
const defaultSubscriberBuffer = 4096

type subscriber[Topic any, Payload any] struct {
	ctx    context.Context
	filter eventstream.TopicFilter[Topic]
	ch     chan eventstream.Event[Topic, Payload]
	closed atomic.Bool
}

type inMemorySyncStreamer[Topic any, Payload any] struct {
	mu          sync.RWMutex
	subscribers map[*subscriber[Topic, Payload]]struct{}
	closed      atomic.Bool
}

// NewInMemorySyncStreamer creates a new in-memory streamer that supports topic
// filtering.
func NewInMemorySyncStreamer[Topic any, Payload any]() eventstream.SyncStreamer[Topic, Payload] {
	return &inMemorySyncStreamer[Topic, Payload]{
		subscribers: make(map[*subscriber[Topic, Payload]]struct{}),
	}
}

func (s *inMemorySyncStreamer[Topic, Payload]) Publish(topic Topic, payloads ...Payload) {
	if s.closed.Load() {
		return
	}
	if len(payloads) == 0 {
		return
	}

	s.mu.RLock()
	subs := make([]*subscriber[Topic, Payload], 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.mu.RUnlock()

	for _, sub := range subs {
		if sub.closed.Load() {
			continue
		}
		if sub.filter != nil && !sub.filter(topic) {
			continue
		}

		// Try to send all payloads
		for _, payload := range payloads {
			event := eventstream.Event[Topic, Payload]{Topic: topic, Payload: payload}
			s.trySend(sub, event)
		}
	}
}

func (s *inMemorySyncStreamer[Topic, Payload]) Subscribe(
	ctx context.Context,
	filter eventstream.TopicFilter[Topic],
) (<-chan eventstream.Event[Topic, Payload], error) {
	if s.closed.Load() {
		return nil, errStreamerClosed
	}

	if filter == nil {
		filter = func(Topic) bool { return true }
	}

	sub := &subscriber[Topic, Payload]{
		ctx:    ctx,
		filter: filter,
		ch:     make(chan eventstream.Event[Topic, Payload], defaultSubscriberBuffer),
	}

	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		return nil, errStreamerClosed
	}
	s.subscribers[sub] = struct{}{}
	s.mu.Unlock()

	go s.monitorContext(sub)

	return sub.ch, nil
}

func (s *inMemorySyncStreamer[Topic, Payload]) Shutdown() {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for sub := range s.subscribers {
		if sub.closed.CompareAndSwap(false, true) {
			close(sub.ch)
		}
	}
	s.subscribers = nil
}

func (s *inMemorySyncStreamer[Topic, Payload]) monitorContext(sub *subscriber[Topic, Payload]) {
	<-sub.ctx.Done()
	s.removeSubscriber(sub)
}

func (s *inMemorySyncStreamer[Topic, Payload]) removeSubscriber(sub *subscriber[Topic, Payload]) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.subscribers == nil {
		return
	}

	if _, ok := s.subscribers[sub]; !ok {
		return
	}
	delete(s.subscribers, sub)
	if sub.closed.CompareAndSwap(false, true) {
		close(sub.ch)
	}
}

func (s *inMemorySyncStreamer[Topic, Payload]) trySend(sub *subscriber[Topic, Payload], evt eventstream.Event[Topic, Payload]) {
	defer func() {
		if r := recover(); r != nil {
			sub.closed.Store(true)
		}
	}()

	select {
	case sub.ch <- evt:
	default:
	}
}

var errStreamerClosed = errors.New("eventstream: streamer closed")
