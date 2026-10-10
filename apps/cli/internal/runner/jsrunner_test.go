package runner

import (
	"context"
	"testing"
	"time"
)

// The worker is ready in a few hundred ms; Start must return as soon as it answers instead
// of sleeping through a fixed start-up delay.
func TestJSRunnerStartReturnsWhenWorkerIsReady(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a Node.js process")
	}
	jsRunner, err := NewJSRunner()
	if err != nil {
		t.Skipf("Node.js not available: %v", err)
	}
	defer jsRunner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	started := time.Now()
	if err := jsRunner.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	elapsed := time.Since(started)
	t.Logf("JS worker ready in %v", elapsed)
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("JS worker start took %v, want well under the old fixed 3 s", elapsed)
	}
}
