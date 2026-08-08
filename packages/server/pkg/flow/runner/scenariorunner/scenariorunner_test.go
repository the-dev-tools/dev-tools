package scenariorunner_test

import (
	"context"
	"errors"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/scenariorunner"
)

// concurrencyProbe records the high-water mark of simultaneously running
// iterations so tests can prove the VU ceiling is actually enforced.
type concurrencyProbe struct {
	current atomic.Int64
	highest atomic.Int64
}

func (p *concurrencyProbe) enter() {
	now := p.current.Add(1)
	for {
		high := p.highest.Load()
		if now <= high || p.highest.CompareAndSwap(high, now) {
			return
		}
	}
}

func (p *concurrencyProbe) leave() { p.current.Add(-1) }

func (p *concurrencyProbe) highWater() int64 { return p.highest.Load() }

func TestRunEnforcesVUCeiling(t *testing.T) {
	var probe concurrencyProbe

	summary, err := scenariorunner.Run(t.Context(),
		scenariorunner.RunProfile{VUs: 5, MaxIterations: 50},
		func(context.Context, int, int64) error {
			probe.enter()
			defer probe.leave()
			time.Sleep(20 * time.Millisecond)
			return nil
		})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if got := probe.highWater(); got > 5 {
		t.Errorf("high-water concurrency = %d, want <= 5", got)
	}
	if got := probe.highWater(); got < 2 {
		t.Errorf("high-water concurrency = %d, want >= 2 (VUs never ran in parallel)", got)
	}
	if summary.Iterations != 50 {
		t.Errorf("Iterations = %d, want 50", summary.Iterations)
	}
	if summary.Errors != 0 {
		t.Errorf("Errors = %d, want 0", summary.Errors)
	}
	if summary.Elapsed <= 0 {
		t.Errorf("Elapsed = %v, want > 0", summary.Elapsed)
	}
}

func TestRunStopsIssuingAtDuration(t *testing.T) {
	const (
		duration  = 150 * time.Millisecond
		iterCost  = 20 * time.Millisecond
		tolerance = 100 * time.Millisecond // generous: covers scheduler jitter on loaded CI
	)

	var (
		mu     sync.Mutex
		starts []time.Time
	)

	begin := time.Now()
	summary, err := scenariorunner.Run(t.Context(),
		scenariorunner.RunProfile{VUs: 4, Duration: duration},
		func(context.Context, int, int64) error {
			mu.Lock()
			starts = append(starts, time.Now())
			mu.Unlock()
			time.Sleep(iterCost)
			return nil
		})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	deadline := begin.Add(duration)

	mu.Lock()
	defer mu.Unlock()

	if len(starts) == 0 {
		t.Fatal("no iterations ran within the duration window")
	}
	for i, start := range starts {
		if start.After(deadline.Add(tolerance)) {
			t.Errorf("iteration %d started %v after the deadline, want no starts past it",
				i, start.Sub(deadline))
		}
	}
	if summary.Iterations != int64(len(starts)) {
		t.Errorf("Iterations = %d, want %d (one per observed start)", summary.Iterations, len(starts))
	}
	if summary.Elapsed < duration {
		t.Errorf("Elapsed = %v, want >= %v", summary.Elapsed, duration)
	}
}

func TestRunCountsErrorsWithoutAborting(t *testing.T) {
	sentinel := errors.New("iteration blew up")

	summary, err := scenariorunner.Run(t.Context(),
		scenariorunner.RunProfile{VUs: 4, MaxIterations: 30},
		func(_ context.Context, _ int, seq int64) error {
			if seq%3 == 0 {
				return sentinel
			}
			return nil
		})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil (iteration errors must not abort the scenario)", err)
	}

	if summary.Iterations != 30 {
		t.Errorf("Iterations = %d, want 30 (errors must not stop later iterations)", summary.Iterations)
	}
	if summary.Errors != 10 {
		t.Errorf("Errors = %d, want 10", summary.Errors)
	}
}

func TestRunHandsOutEachSequenceExactlyOnce(t *testing.T) {
	const (
		vus   = 4
		total = 200
	)

	var (
		mu   sync.Mutex
		seqs []int64
	)

	summary, err := scenariorunner.Run(t.Context(),
		scenariorunner.RunProfile{VUs: vus, MaxIterations: total},
		func(_ context.Context, vu int, seq int64) error {
			if vu < 0 || vu >= vus {
				return errors.New("vu index out of range")
			}
			mu.Lock()
			seqs = append(seqs, seq)
			mu.Unlock()
			return nil
		})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if summary.Errors != 0 {
		t.Fatalf("Errors = %d, want 0 (vu index out of range)", summary.Errors)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(seqs) != total {
		t.Fatalf("collected %d sequence numbers, want %d", len(seqs), total)
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	for i, seq := range seqs {
		if seq != int64(i) {
			t.Fatalf("sequence numbers are not 0..%d contiguous: index %d = %d", total-1, i, seq)
		}
	}
}

func TestRunStopsAtWhicheverBoundComesFirst(t *testing.T) {
	summary, err := scenariorunner.Run(t.Context(),
		scenariorunner.RunProfile{VUs: 2, MaxIterations: 5, Duration: time.Hour},
		func(context.Context, int, int64) error { return nil })
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if summary.Iterations != 5 {
		t.Errorf("Iterations = %d, want 5 (iteration bound must win over the long duration)", summary.Iterations)
	}
}

func TestRunReturnsContextErrorAndDrains(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var started, finished atomic.Int64

	go func() {
		time.Sleep(60 * time.Millisecond)
		cancel()
	}()

	summary, err := scenariorunner.Run(ctx,
		scenariorunner.RunProfile{VUs: 3, Duration: time.Hour},
		func(context.Context, int, int64) error {
			started.Add(1)
			// Deliberately ignores ctx: a runner that abandons in-flight work
			// instead of draining would return while this is still sleeping.
			time.Sleep(50 * time.Millisecond)
			finished.Add(1)
			return nil
		})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if s, f := started.Load(), finished.Load(); s != f {
		t.Errorf("started = %d, finished = %d: in-flight iterations were not drained", s, f)
	}
	if summary.Iterations != finished.Load() {
		t.Errorf("Iterations = %d, want %d (partial summary must report completed work)",
			summary.Iterations, finished.Load())
	}
	if summary.Elapsed <= 0 {
		t.Errorf("Elapsed = %v, want > 0", summary.Elapsed)
	}
}

func TestRunRejectsInvalidProfiles(t *testing.T) {
	okIter := func(context.Context, int, int64) error { return nil }

	tests := []struct {
		name    string
		profile scenariorunner.RunProfile
		iter    func(ctx context.Context, vu int, seq int64) error
		wantErr error
	}{
		{
			name:    "zero VUs",
			profile: scenariorunner.RunProfile{VUs: 0, MaxIterations: 10},
			iter:    okIter,
			wantErr: scenariorunner.ErrInvalidVUs,
		},
		{
			name:    "negative VUs",
			profile: scenariorunner.RunProfile{VUs: -3, Duration: time.Second},
			iter:    okIter,
			wantErr: scenariorunner.ErrInvalidVUs,
		},
		{
			name:    "no stop condition",
			profile: scenariorunner.RunProfile{VUs: 2},
			iter:    okIter,
			wantErr: scenariorunner.ErrNoStopCondition,
		},
		{
			name:    "negative bounds are not bounds",
			profile: scenariorunner.RunProfile{VUs: 2, Duration: -time.Second, MaxIterations: -5},
			iter:    okIter,
			wantErr: scenariorunner.ErrNoStopCondition,
		},
		{
			name:    "nil iteration function",
			profile: scenariorunner.RunProfile{VUs: 2, MaxIterations: 10},
			iter:    nil,
			wantErr: scenariorunner.ErrNilIteration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int64
			iter := tt.iter
			if iter != nil {
				iter = func(ctx context.Context, vu int, seq int64) error {
					calls.Add(1)
					return tt.iter(ctx, vu, seq)
				}
			}

			summary, err := scenariorunner.Run(t.Context(), tt.profile, iter)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, tt.wantErr)
			}
			if calls.Load() != 0 {
				t.Errorf("iteration function called %d times, want 0 (no work on config error)", calls.Load())
			}
			if summary != (scenariorunner.Summary{}) {
				t.Errorf("Summary = %+v, want zero value", summary)
			}
		})
	}
}

func TestRunDoesNotLeakGoroutines(t *testing.T) {
	// Warm up so one-shot runtime goroutines are not attributed to Run.
	if _, err := scenariorunner.Run(t.Context(),
		scenariorunner.RunProfile{VUs: 2, MaxIterations: 2},
		func(context.Context, int, int64) error { return nil }); err != nil {
		t.Fatalf("warm-up Run() error = %v", err)
	}

	before := runtime.NumGoroutine()

	if _, err := scenariorunner.Run(t.Context(),
		scenariorunner.RunProfile{VUs: 16, MaxIterations: 200},
		func(context.Context, int, int64) error {
			time.Sleep(time.Millisecond)
			return nil
		}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := scenariorunner.Run(ctx,
		scenariorunner.RunProfile{VUs: 16, Duration: time.Hour},
		func(context.Context, int, int64) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Run() error = %v, want context.Canceled", err)
	}

	// Goroutines exit asynchronously; give the scheduler a bounded window.
	var after int
	for range 100 {
		after = runtime.NumGoroutine()
		if after <= before+2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("goroutines before = %d, after = %d: scheduler leaked workers", before, after)
}
