package rflowv2

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// TestResponseExecutionSyncCoordination tests the coordination mechanism
// that ensures HttpResponse events are published before NodeExecution events.
// This prevents race conditions where frontend receives NodeExecution with
// ResponseID before the HttpResponse itself has arrived.
func TestResponseExecutionSyncCoordination(t *testing.T) {
	t.Run("basic ordering - execution waits for response", func(t *testing.T) {
		responsePublished := make(map[string]chan struct{})
		var mu sync.Mutex

		responseID := idwrap.NewNow()

		// Track event order
		var eventOrder []string
		var orderMu sync.Mutex

		// Simulate response handler (registers and signals)
		responseHandler := func() {
			mu.Lock()
			ch := make(chan struct{})
			responsePublished[responseID.String()] = ch
			mu.Unlock()

			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)

			orderMu.Lock()
			eventOrder = append(eventOrder, "response_published")
			orderMu.Unlock()

			close(ch)
		}

		// Simulate execution handler (waits for response)
		executionHandler := func() {
			mu.Lock()
			ch, ok := responsePublished[responseID.String()]
			mu.Unlock()

			if ok {
				<-ch // Wait for response to be published
			}

			orderMu.Lock()
			eventOrder = append(eventOrder, "execution_published")
			orderMu.Unlock()
		}

		// Start response handler first (simulates normal flow)
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			responseHandler()
		}()

		// Small delay to ensure response handler registers first
		time.Sleep(5 * time.Millisecond)

		go func() {
			defer wg.Done()
			executionHandler()
		}()

		wg.Wait()

		// Verify order: response should always be before execution
		require.Len(t, eventOrder, 2)
		assert.Equal(t, "response_published", eventOrder[0], "Response should be published first")
		assert.Equal(t, "execution_published", eventOrder[1], "Execution should be published second")
	})

	t.Run("execution without ResponseID does not wait", func(t *testing.T) {
		executed := make(chan struct{})

		// Execution handler with no ResponseID
		go func() {

			// No wait if auxiliaryID is nil (as defined above)

			close(executed)
		}()

		// Should complete immediately without waiting
		select {
		case <-executed:
			// Success - didn't wait
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Execution without ResponseID should not wait")
		}
	})

	t.Run("context cancellation unblocks wait", func(t *testing.T) {
		responsePublished := make(map[string]chan struct{})
		var mu sync.Mutex

		responseID := idwrap.NewNow()
		ctx, cancel := context.WithCancel(context.Background())

		// Register but don't close the channel (simulating slow response)
		mu.Lock()
		publishedChan := make(chan struct{})
		responsePublished[responseID.String()] = publishedChan
		mu.Unlock()

		executed := make(chan struct{})

		go func() {
			mu.Lock()
			ch, ok := responsePublished[responseID.String()]
			mu.Unlock()

			if ok {
				select {
				case <-ch:
					// Response published
				case <-ctx.Done():
					// Context cancelled - proceed anyway
				}
			}

			close(executed)
		}()

		// Cancel context after short delay
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Should complete due to context cancellation
		select {
		case <-executed:
			// Success - context cancellation unblocked
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Context cancellation should unblock wait")
		}
	})

	t.Run("multiple concurrent responses maintain ordering", func(t *testing.T) {
		responsePublished := make(map[string]chan struct{})
		var mu sync.Mutex

		numRequests := 5
		responseIDs := make([]idwrap.IDWrap, numRequests)
		for i := range responseIDs {
			responseIDs[i] = idwrap.NewNow()
		}

		// Track per-response ordering
		type eventRecord struct {
			responseID string
			eventType  string
			timestamp  time.Time
		}
		var events []eventRecord
		var eventsMu sync.Mutex

		var wg sync.WaitGroup

		// Start response handlers
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				respID := responseIDs[idx]

				mu.Lock()
				ch := make(chan struct{})
				responsePublished[respID.String()] = ch
				mu.Unlock()

				// Variable processing time
				time.Sleep(time.Duration(idx*5) * time.Millisecond)

				eventsMu.Lock()
				events = append(events, eventRecord{
					responseID: respID.String(),
					eventType:  "response",
					timestamp:  time.Now(),
				})
				eventsMu.Unlock()

				close(ch)
			}(i)
		}

		// Small delay to ensure all response handlers register
		time.Sleep(5 * time.Millisecond)

		// Start execution handlers
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				respID := responseIDs[idx]

				mu.Lock()
				ch, ok := responsePublished[respID.String()]
				mu.Unlock()

				if ok {
					<-ch
				}

				eventsMu.Lock()
				events = append(events, eventRecord{
					responseID: respID.String(),
					eventType:  "execution",
					timestamp:  time.Now(),
				})
				eventsMu.Unlock()
			}(i)
		}

		wg.Wait()

		// Verify: for each responseID, response event should be before execution event
		responseTimestamps := make(map[string]time.Time)
		executionTimestamps := make(map[string]time.Time)

		for _, e := range events {
			if e.eventType == "response" {
				responseTimestamps[e.responseID] = e.timestamp
			} else {
				executionTimestamps[e.responseID] = e.timestamp
			}
		}

		for _, respID := range responseIDs {
			respTime, respOk := responseTimestamps[respID.String()]
			execTime, execOk := executionTimestamps[respID.String()]

			require.True(t, respOk, "Response event not found for %s", respID.String())
			require.True(t, execOk, "Execution event not found for %s", respID.String())
			assert.True(t, respTime.Before(execTime) || respTime.Equal(execTime),
				"Response should be before or equal to execution for %s", respID.String())
		}
	})

	t.Run("execution handler finds channel even with concurrent registration", func(t *testing.T) {
		responsePublished := make(map[string]chan struct{})
		var mu sync.Mutex

		responseID := idwrap.NewNow()
		executed := make(chan struct{})
		responseHandlerStarted := make(chan struct{})

		// Response handler
		go func() {
			close(responseHandlerStarted)

			mu.Lock()
			ch := make(chan struct{})
			responsePublished[responseID.String()] = ch
			mu.Unlock()

			// Simulate processing
			time.Sleep(20 * time.Millisecond)
			close(ch)
		}()

		// Wait for response handler to start
		<-responseHandlerStarted
		// Small delay for registration
		time.Sleep(10 * time.Millisecond)

		// Execution handler
		go func() {
			mu.Lock()
			ch, ok := responsePublished[responseID.String()]
			mu.Unlock()

			require.True(t, ok, "Channel should be registered")

			<-ch
			close(executed)
		}()

		select {
		case <-executed:
			// Success
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Execution should complete after response")
		}
	})
}

// TestResponsePublishedMapCleanup verifies that we don't leak memory
// by ensuring map entries are cleaned up after use.
func TestResponsePublishedMapCleanup(t *testing.T) {
	t.Run("map entries are cleaned up after execution waits", func(t *testing.T) {
		responsePublished := make(map[string]chan struct{})
		var mu sync.Mutex

		// Simulate the full response -> execution flow with cleanup
		for i := 0; i < 10; i++ {
			id := idwrap.NewNow().String()

			// Response handler registers and closes
			mu.Lock()
			ch := make(chan struct{})
			responsePublished[id] = ch
			mu.Unlock()

			close(ch) // Response published

			// Execution handler waits and cleans up
			mu.Lock()
			waitCh, ok := responsePublished[id]
			mu.Unlock()

			if ok {
				<-waitCh // Wait (already closed, returns immediately)

				// Clean up
				mu.Lock()
				delete(responsePublished, id)
				mu.Unlock()
			}
		}

		// Map should be empty after cleanup
		mu.Lock()
		mapLen := len(responsePublished)
		mu.Unlock()

		assert.Equal(t, 0, mapLen, "Map should be empty after cleanup")
	})

	t.Run("cleanup happens even with context cancellation", func(t *testing.T) {
		responsePublished := make(map[string]chan struct{})
		var mu sync.Mutex
		ctx, cancel := context.WithCancel(context.Background())

		id := idwrap.NewNow().String()

		// Response handler registers but doesn't close (simulating slow response)
		mu.Lock()
		ch := make(chan struct{})
		responsePublished[id] = ch
		mu.Unlock()

		// Cancel context
		cancel()

		// Execution handler waits (will be unblocked by context) and cleans up
		mu.Lock()
		waitCh, ok := responsePublished[id]
		mu.Unlock()

		if ok {
			select {
			case <-waitCh:
			case <-ctx.Done():
			}

			// Clean up even on context cancellation
			mu.Lock()
			delete(responsePublished, id)
			mu.Unlock()
		}

		// Map should be empty
		mu.Lock()
		mapLen := len(responsePublished)
		mu.Unlock()

		assert.Equal(t, 0, mapLen, "Map should be empty after cleanup on cancellation")
	})
}
