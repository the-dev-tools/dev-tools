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
	
	t.Run("Context timeout is respected", func(t *testing.T) {
		// Create a server that delays for 5 seconds
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(5 * time.Second):
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Success after delay"))
			case <-r.Context().Done():
				// Request was cancelled
				return
			}
		}))
		defer server.Close()

		// Test 1: Short timeout should fail
		{
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

			if elapsed > 3*time.Second {
				t.Errorf("Expected timeout in ~2 seconds, but took %v", elapsed)
			}
		}

		// Test 2: Context isolation works
		{
			// Create a parent context with short timeout
			parentCtx, parentCancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer parentCancel()

			// Create an isolated context (like rflow.go does)
			isolatedCtx, isolatedCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer isolatedCancel()

			// Wait for parent to timeout
			time.Sleep(2 * time.Second)

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

			if elapsed < 5*time.Second {
				t.Errorf("Expected request to take ~5 seconds, but took %v", elapsed)
			}
		}
	})
}

func TestContextPropagation(t *testing.T) {
	// Test that demonstrates the difference between NewRequest and NewRequestWithContext
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			return
		}
	}))
	defer server.Close()

	t.Run("NewRequest ignores context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Using http.NewRequest (old way)
		req, err := http.NewRequest("GET", server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		client := &http.Client{
			Timeout: 5 * time.Second, // Client timeout is longer
		}
		
		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)

		// Even though context has 1 second timeout, request will complete
		// because NewRequest doesn't use the context
		if err != nil {
			t.Logf("Request failed as expected in %v: %v", elapsed, err)
		} else {
			t.Logf("Request succeeded in %v with status %d", elapsed, resp.StatusCode)
		}

		// Context should be done
		select {
		case <-ctx.Done():
			t.Log("Context timed out as expected")
		default:
			t.Error("Expected context to be timed out")
		}
	})

	t.Run("NewRequestWithContext respects context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Using http.NewRequestWithContext (new way)
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		client := &http.Client{
			Timeout: 5 * time.Second, // Client timeout is longer
		}
		
		start := time.Now()
		_, err = client.Do(req)
		elapsed := time.Since(start)

		// Request should fail due to context timeout
		if err == nil {
			t.Error("Expected error due to context timeout")
		}

		if elapsed > 2*time.Second {
			t.Errorf("Expected timeout in ~1 second, but took %v", elapsed)
		}
	})
}