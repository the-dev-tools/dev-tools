package rimportv2

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	apiv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/import/v1"

	"connectrpc.com/connect"
)

// TestConcurrencyStress_DeadlockDetection simulates high concurrency to detect deadlocks
// specifically focusing on the StoreDomainVariables path which caused deadlocks previously.
func TestConcurrencyStress_DeadlockDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	fixture := newIntegrationTestFixture(t)

	// 1. Setup: Create multiple environments to maximize StoreDomainVariables work
	// This increases the chance of lock contention during the Read-Write cycle
	numEnvs := 10
	for i := 0; i < numEnvs; i++ {
		err := fixture.rpc.EnvService.Create(fixture.ctx, menv.Env{
			ID:          idwrap.NewNow(),
			WorkspaceID: fixture.workspaceID,
			Name:        fmt.Sprintf("Env %d", i),
		})
		require.NoError(t, err)
	}

	// 2. Prepare payload that triggers both StoreUnifiedResults and StoreDomainVariables
	// Using HAR with multiple domains to trigger variable creation
	harData := createMultiDomainHAR(t)

	// 3. Run concurrent imports
	// High concurrency to force contention
	concurrency := 20
	iterations := 5 // Each goroutine does this many imports

	var wg sync.WaitGroup
	start := time.Now()

	// Use a timeout to detect deadlocks (stuck tests)
	// If the test hangs here, it's likely a deadlock
	ctx, cancel := context.WithTimeout(fixture.ctx, 30*time.Second)
	defer cancel()

	errCh := make(chan error, concurrency*iterations)

	t.Logf("Starting stress test with %d workers, %d iterations each", concurrency, iterations)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				select {
				case <-ctx.Done():
					errCh <- fmt.Errorf("worker %d timed out", workerID)
					return
				default:
				}

				// Create a unique request
				req := connect.NewRequest(&apiv1.ImportRequest{
					WorkspaceId: fixture.workspaceID.Bytes(),
					Name:        fmt.Sprintf("Stress Import %d-%d", workerID, j),
					Data:        harData,
					// Provide domain data to force StoreDomainVariables logic to run
					DomainData: []*apiv1.ImportDomainData{
						{Enabled: true, Domain: "api.example.com", Variable: "API_VAR"},
						{Enabled: true, Domain: "cdn.example.com", Variable: "CDN_VAR"},
					},
				})

				_, err := fixture.rpc.Import(ctx, req)
				if err != nil {
					errCh <- fmt.Errorf("worker %d iter %d failed: %w", workerID, j, err)
				}
			}
		}(i)
	}

	// Wait for all to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Logf("Stress test finished in %v", time.Since(start))
	case <-ctx.Done():
		t.Fatal("Stress test timed out - likely deadlock!")
	}
	close(errCh)

	// Check for errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Errorf("Encountered %d errors during stress test", len(errs))
		for i, e := range errs {
			if i < 5 { // Log first 5 errors
				t.Logf("Error %d: %v", i, e)
			}
		}
		// Fail if failure rate is high (some might be transient)
		if len(errs) > (concurrency * iterations / 5) {
			t.FailNow()
		}
	}
}
