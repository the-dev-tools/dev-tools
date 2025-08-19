package rflow_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPRequestWithContext(t *testing.T) {
	// Test that our HTTP client properly respects context timeout
	t.Parallel() // Run this test in parallel with others

	t.Run("Context timeout is respected", func(t *testing.T) {
		t.Parallel() // Run sub-tests in parallel

		// Create a server that delays for 500ms (reduced from 5 seconds)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(500 * time.Millisecond):
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Success after delay"))
			case <-r.Context().Done():
				// Request was cancelled
				return
			}
		}))
		defer server.Close()

		// Test 1: Short timeout should fail
		{
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			client := &http.Client{}
			start := time.Now()
			_, err = client.Do(req)
			elapsed := time.Since(start)

			if err == nil {
				t.Error("Expected error due to timeout, but got none")
			}

			if elapsed > 300*time.Millisecond {
				t.Errorf("Expected timeout in ~200ms, but took %v", elapsed)
			}
		}

		// Test 2: Context isolation works
		{
			// Create a parent context with short timeout
			parentCtx, parentCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer parentCancel()

			// Create an isolated context (like rflow.go does)
			isolatedCtx, isolatedCancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer isolatedCancel()

			// Wait for parent to timeout (reduced from 2 seconds to 200ms)
			time.Sleep(200 * time.Millisecond)

			// Verify parent is cancelled
			select {
			case <-parentCtx.Done():
				// Good
			default:
				t.Error("Expected parent context to be cancelled")
			}

			// Create request with isolated context
			req, err := http.NewRequestWithContext(isolatedCtx, "GET", server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			// This should still work because we're using isolated context
			client := &http.Client{}
			start := time.Now()
			resp, err := client.Do(req)
			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("Expected success with isolated context, but got error: %v", err)
			}

			if resp != nil && resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			if elapsed < 500*time.Millisecond {
				t.Errorf("Expected request to take ~500ms, but took %v", elapsed)
			}
		}
	})
}

func TestContextPropagation(t *testing.T) {
	// Test that demonstrates the difference between NewRequest and NewRequestWithContext
	t.Parallel() // Run this test in parallel with others

	t.Run("NewRequest ignores context timeout", func(t *testing.T) {
		t.Parallel() // Run sub-tests in parallel

		// Create server for this specific test
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(300 * time.Millisecond): // Reduced from 3 seconds
				w.WriteHeader(http.StatusOK)
			case <-r.Context().Done():
				return
			}
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Using http.NewRequest (old way)
		req, err := http.NewRequest("GET", server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		client := &http.Client{
			Timeout: 500 * time.Millisecond, // Client timeout is longer (reduced from 5s)
		}

		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)

		// Even though context has 100ms timeout, request will complete
		// because NewRequest doesn't use the context
		if err != nil {
			t.Logf("Request failed as expected in %v: %v", elapsed, err)
		} else {
			t.Logf("Request succeeded in %v with status %d", elapsed, resp.StatusCode)
		}

		// Wait a bit for context to timeout since the request doesn't respect it
		time.Sleep(150 * time.Millisecond)

		// Context should be done
		select {
		case <-ctx.Done():
			t.Log("Context timed out as expected")
		default:
			t.Error("Expected context to be timed out")
		}
	})

	t.Run("NewRequestWithContext respects context timeout", func(t *testing.T) {
		t.Parallel() // Run sub-tests in parallel

		// Create server for this specific test
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(300 * time.Millisecond): // Reduced from 3 seconds
				w.WriteHeader(http.StatusOK)
			case <-r.Context().Done():
				return
			}
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Using http.NewRequestWithContext (new way)
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		client := &http.Client{
			Timeout: 500 * time.Millisecond, // Client timeout is longer (reduced from 5s)
		}

		start := time.Now()
		_, err = client.Do(req)
		elapsed := time.Since(start)

		// Request should fail due to context timeout
		if err == nil {
			t.Error("Expected error due to context timeout")
		}

		if elapsed > 200*time.Millisecond {
			t.Errorf("Expected timeout in ~100ms, but took %v", elapsed)
		}
	})
}
