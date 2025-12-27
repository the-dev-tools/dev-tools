package testutil

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ConcurrencyTestConfig holds parameters for concurrency tests.
type ConcurrencyTestConfig struct {
	// NumGoroutines is the number of concurrent operations to run.
	// Default: 20
	NumGoroutines int

	// Timeout is the maximum duration for each operation.
	// If an operation takes longer, it's counted as a timeout (potential deadlock).
	// Default: 3 seconds
	Timeout time.Duration

	// ExpectSuccess indicates whether operations should succeed.
	// Default: true
	ExpectSuccess bool
}

// ConcurrencyTestResult captures the outcome of concurrency tests.
type ConcurrencyTestResult struct {
	// SuccessCount is the number of operations that completed successfully.
	SuccessCount int

	// ErrorCount is the number of operations that returned errors.
	ErrorCount int

	// TimeoutCount is the number of operations that exceeded the timeout.
	// This indicates potential deadlocks or blocking issues.
	TimeoutCount int

	// AverageDuration is the mean duration of all operations.
	AverageDuration time.Duration

	// MaxDuration is the longest operation duration.
	MaxDuration time.Duration

	// MinDuration is the shortest operation duration.
	MinDuration time.Duration
}

// RunConcurrentInserts executes multiple insert operations concurrently
// and detects timeouts/deadlocks. This is useful for testing that database
// operations handle concurrent requests without SQLite deadlocks.
//
// Parameters:
//   - ctx: Context to use for all operations (e.g., with auth)
//   - t: Testing context
//   - config: Configuration for the concurrency test
//   - setupData: Function to prepare test data for each goroutine (index i)
//   - executeInsert: Function to execute the insert operation
//
// Returns:
//   - ConcurrencyTestResult with success/error/timeout counts and timing stats
//
// Example:
//
//	result := testutil.RunConcurrentInserts(ctx, t, config,
//	    func(i int) *MyData {
//	        return &MyData{ID: i}
//	    },
//	    func(ctx context.Context, data *MyData) error {
//	        return service.Insert(ctx, data)
//	    },
//	)
//	assert.Equal(t, 0, result.TimeoutCount, "No deadlocks expected")
func RunConcurrentInserts[T any](
	ctx context.Context,
	t *testing.T,
	config ConcurrencyTestConfig,
	setupData func(i int) T,
	executeInsert func(ctx context.Context, data T) error,
) ConcurrencyTestResult {
	t.Helper()

	// Apply defaults
	if config.NumGoroutines == 0 {
		config.NumGoroutines = 20
	}
	if config.Timeout == 0 {
		config.Timeout = 3 * time.Second
	}

	// Result tracking
	type opResult struct {
		success  bool
		timeout  bool
		duration time.Duration
		err      error
	}

	resultChan := make(chan opResult, config.NumGoroutines)
	var wg sync.WaitGroup

	// Launch concurrent operations
	for i := 0; i < config.NumGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Setup data for this operation
			data := setupData(index)

			// Create context with timeout
			opCtx, cancel := context.WithTimeout(ctx, config.Timeout)
			defer cancel()

			// Track timing
			start := time.Now()

			// Execute operation
			done := make(chan error, 1)
			go func() {
				done <- executeInsert(opCtx, data)
			}()

			// Wait for completion or timeout
			select {
			case err := <-done:
				duration := time.Since(start)
				resultChan <- opResult{
					success:  err == nil,
					timeout:  false,
					duration: duration,
					err:      err,
				}
			case <-opCtx.Done():
				// Timeout - potential deadlock
				duration := time.Since(start)
				resultChan <- opResult{
					success:  false,
					timeout:  true,
					duration: duration,
					err:      opCtx.Err(),
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var (
		successCount int
		errorCount   int
		timeoutCount int
		totalDuration time.Duration
		maxDuration   time.Duration
		minDuration   = time.Hour // Start with a large value
	)

	for result := range resultChan {
		if result.timeout {
			timeoutCount++
			t.Logf("⚠️  Operation timed out after %v (potential deadlock)", result.duration)
		} else if result.success {
			successCount++
		} else {
			errorCount++
			t.Logf("❌ Operation failed: %v", result.err)
		}

		totalDuration += result.duration
		if result.duration > maxDuration {
			maxDuration = result.duration
		}
		if result.duration < minDuration {
			minDuration = result.duration
		}
	}

	// Calculate average
	var avgDuration time.Duration
	if config.NumGoroutines > 0 {
		avgDuration = totalDuration / time.Duration(config.NumGoroutines)
	}

	// Reset min if no operations completed
	if minDuration == time.Hour {
		minDuration = 0
	}

	return ConcurrencyTestResult{
		SuccessCount:    successCount,
		ErrorCount:      errorCount,
		TimeoutCount:    timeoutCount,
		AverageDuration: avgDuration,
		MaxDuration:     maxDuration,
		MinDuration:     minDuration,
	}
}

// RunConcurrentUpdates executes multiple update operations concurrently.
// See RunConcurrentInserts for detailed documentation.
func RunConcurrentUpdates[T any](
	ctx context.Context,
	t *testing.T,
	config ConcurrencyTestConfig,
	setupData func(i int) T,
	executeUpdate func(ctx context.Context, data T) error,
) ConcurrencyTestResult {
	t.Helper()
	return RunConcurrentInserts(ctx, t, config, setupData, executeUpdate)
}

// RunConcurrentDeletes executes multiple delete operations concurrently.
// See RunConcurrentInserts for detailed documentation.
func RunConcurrentDeletes[T any](
	ctx context.Context,
	t *testing.T,
	config ConcurrencyTestConfig,
	setupData func(i int) T,
	executeDelete func(ctx context.Context, data T) error,
) ConcurrencyTestResult {
	t.Helper()
	return RunConcurrentInserts(ctx, t, config, setupData, executeDelete)
}
