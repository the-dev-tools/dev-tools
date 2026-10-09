package scenariorunner_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/scenariorunner"
)

// waitOrDone sleeps for d unless ctx ends first, and reports ctx's error in
// that case - the shape of a flow iteration that honours cancellation.
func waitOrDone(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestRunRampingVUsFollowsStages(t *testing.T) {
	var (
		probe  concurrencyProbe
		maxVU  atomic.Int64
		early  atomic.Int64 // high-water concurrency during the first 60ms
		begins = time.Now()
	)

	summary, err := scenariorunner.RunRampingVUs(t.Context(), scenariorunner.RampingVUsProfile{
		StartVUs: 0,
		Stages: []scenariorunner.Stage{
			{Duration: 200 * time.Millisecond, Target: 4},
			{Duration: 200 * time.Millisecond, Target: 4},
			{Duration: 100 * time.Millisecond, Target: 0},
		},
	}, func(ctx context.Context, vu int, _ int64) error {
		probe.enter()
		defer probe.leave()
		if int64(vu) > maxVU.Load() {
			maxVU.Store(int64(vu))
		}
		if time.Since(begins) < 60*time.Millisecond {
			if c := probe.current.Load(); c > early.Load() {
				early.Store(c)
			}
		}
		return waitOrDone(ctx, 10*time.Millisecond)
	})
	if err != nil {
		t.Fatalf("RunRampingVUs() error = %v", err)
	}

	if got := probe.highWater(); got != 4 {
		t.Errorf("high-water concurrency = %d, want 4", got)
	}
	if got := maxVU.Load(); got > 3 {
		t.Errorf("highest VU index = %d, want <= 3", got)
	}
	if got := early.Load(); got > 2 {
		t.Errorf("concurrency during the first 60ms of a 0->4 ramp over 200ms = %d, want <= 2", got)
	}
	if summary.Iterations == 0 {
		t.Error("no iterations ran")
	}
	if summary.Elapsed < 450*time.Millisecond || summary.Elapsed > 900*time.Millisecond {
		t.Errorf("Elapsed = %v, want about 500ms", summary.Elapsed)
	}
}

func TestRunRampingVUsGracefulRampDownInterrupts(t *testing.T) {
	summary, err := scenariorunner.RunRampingVUs(t.Context(), scenariorunner.RampingVUsProfile{
		StartVUs: 2,
		Stages: []scenariorunner.Stage{
			{Duration: 50 * time.Millisecond, Target: 2},
			{Duration: 10 * time.Millisecond, Target: 0},
			{Duration: 300 * time.Millisecond, Target: 0},
		},
		GracefulRampDown: 40 * time.Millisecond,
	}, func(ctx context.Context, _ int, _ int64) error {
		return waitOrDone(ctx, 5*time.Second)
	})
	if err != nil {
		t.Fatalf("RunRampingVUs() error = %v", err)
	}
	if summary.Interrupted != 2 {
		t.Errorf("Interrupted = %d, want 2", summary.Interrupted)
	}
	if summary.Iterations != 0 || summary.Errors != 0 {
		t.Errorf("interrupted iterations must not count as completed or failed, got %+v", summary)
	}
	if summary.Elapsed > time.Second {
		t.Errorf("Elapsed = %v: graceful ramp-down did not cut the iterations off", summary.Elapsed)
	}
}

func TestRunRampingVUsGracefulRampDownLetsShortIterationsFinish(t *testing.T) {
	summary, err := scenariorunner.RunRampingVUs(t.Context(), scenariorunner.RampingVUsProfile{
		StartVUs: 2,
		Stages: []scenariorunner.Stage{
			{Duration: 30 * time.Millisecond, Target: 2},
			{Duration: 10 * time.Millisecond, Target: 0},
			{Duration: 200 * time.Millisecond, Target: 0},
		},
		GracefulRampDown: time.Second,
	}, func(ctx context.Context, _ int, _ int64) error {
		return waitOrDone(ctx, 80*time.Millisecond)
	})
	if err != nil {
		t.Fatalf("RunRampingVUs() error = %v", err)
	}
	if summary.Interrupted != 0 {
		t.Errorf("Interrupted = %d, want 0", summary.Interrupted)
	}
	if summary.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2 (one per VU, then ramped away)", summary.Iterations)
	}
}

func TestRunRampingVUsGracefulStopAtEnd(t *testing.T) {
	summary, err := scenariorunner.RunRampingVUs(t.Context(), scenariorunner.RampingVUsProfile{
		StartVUs:     1,
		Stages:       []scenariorunner.Stage{{Duration: 30 * time.Millisecond, Target: 1}},
		GracefulStop: 30 * time.Millisecond,
	}, func(ctx context.Context, _ int, _ int64) error {
		return waitOrDone(ctx, 5*time.Second)
	})
	if err != nil {
		t.Fatalf("RunRampingVUs() error = %v", err)
	}
	if summary.Interrupted != 1 {
		t.Errorf("Interrupted = %d, want 1", summary.Interrupted)
	}
	if summary.Elapsed > time.Second {
		t.Errorf("Elapsed = %v, want the graceful stop to cut the run short", summary.Elapsed)
	}
}

func TestRunArrivalRateStartsOnScheduleRegardlessOfLatency(t *testing.T) {
	var (
		probe  concurrencyProbe
		mu     sync.Mutex
		starts []time.Duration
		begin  = time.Now()
	)

	summary, err := scenariorunner.RunArrivalRate(t.Context(), scenariorunner.ArrivalRateProfile{
		StartRate:       50,
		Stages:          []scenariorunner.Stage{{Duration: 200 * time.Millisecond, Target: 50}},
		TimeUnit:        time.Second,
		PreAllocatedVUs: 2,
		MaxVUs:          10,
	}, func(ctx context.Context, _ int, _ int64) error {
		mu.Lock()
		starts = append(starts, time.Since(begin))
		mu.Unlock()
		probe.enter()
		defer probe.leave()
		// Each iteration takes five arrival intervals; a closed model would
		// only manage a fraction of the schedule.
		return waitOrDone(ctx, 100*time.Millisecond)
	})
	if err != nil {
		t.Fatalf("RunArrivalRate() error = %v", err)
	}

	if summary.Iterations != 10 {
		t.Errorf("Iterations = %d, want 10 (50/s for 200ms)", summary.Iterations)
	}
	if summary.Dropped != 0 {
		t.Errorf("Dropped = %d, want 0", summary.Dropped)
	}
	if got := probe.highWater(); got < 4 {
		t.Errorf("high-water concurrency = %d: iterations did not overlap", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if last := starts[len(starts)-1]; last > 300*time.Millisecond {
		t.Errorf("last iteration started at %v, want within the 200ms schedule", last)
	}
}

func TestRunArrivalRateDropsWhenMaxVUsExhausted(t *testing.T) {
	var vus sync.Map

	summary, err := scenariorunner.RunArrivalRate(t.Context(), scenariorunner.ArrivalRateProfile{
		StartRate:       100,
		Stages:          []scenariorunner.Stage{{Duration: 200 * time.Millisecond, Target: 100}},
		PreAllocatedVUs: 1,
		MaxVUs:          2,
	}, func(ctx context.Context, vu int, _ int64) error {
		vus.Store(vu, true)
		return waitOrDone(ctx, 150*time.Millisecond)
	})
	if err != nil {
		t.Fatalf("RunArrivalRate() error = %v", err)
	}

	if total := summary.Iterations + summary.Dropped; total != 20 {
		t.Errorf("Iterations + Dropped = %d + %d, want 20 scheduled starts", summary.Iterations, summary.Dropped)
	}
	if summary.Dropped < 15 {
		t.Errorf("Dropped = %d, want most of the 20 starts dropped with only 2 VUs", summary.Dropped)
	}
	vus.Range(func(key, _ any) bool {
		if vu, _ := key.(int); vu < 0 || vu >= 2 {
			t.Errorf("iteration ran on VU %d, outside MaxVUs", vu)
		}
		return true
	})
}

func TestRunArrivalRateLiveGauges(t *testing.T) {
	live := &scenariorunner.Live{}
	var sawActive atomic.Int64

	summary, err := scenariorunner.RunArrivalRate(t.Context(), scenariorunner.ArrivalRateProfile{
		StartRate:       100,
		Stages:          []scenariorunner.Stage{{Duration: 100 * time.Millisecond, Target: 100}},
		PreAllocatedVUs: 1,
		MaxVUs:          1,
		Live:            live,
	}, func(ctx context.Context, _ int, _ int64) error {
		sawActive.Store(live.ActiveVUs.Load())
		return waitOrDone(ctx, 50*time.Millisecond)
	})
	if err != nil {
		t.Fatalf("RunArrivalRate() error = %v", err)
	}
	if sawActive.Load() != 1 {
		t.Errorf("ActiveVUs during an iteration = %d, want 1", sawActive.Load())
	}
	if live.ActiveVUs.Load() != 0 {
		t.Errorf("ActiveVUs after the run = %d, want 0", live.ActiveVUs.Load())
	}
	if live.Dropped.Load() != summary.Dropped || summary.Dropped == 0 {
		t.Errorf("Live.Dropped = %d, Summary.Dropped = %d; want equal and non-zero", live.Dropped.Load(), summary.Dropped)
	}
}

func TestStopEndsEveryExecutorEarly(t *testing.T) {
	iter := func(ctx context.Context, _ int, _ int64) error {
		return waitOrDone(ctx, 5*time.Millisecond)
	}

	cases := map[string]func(stop <-chan struct{}) (scenariorunner.Summary, error){
		"constant-vus": func(stop <-chan struct{}) (scenariorunner.Summary, error) {
			return scenariorunner.Run(t.Context(), scenariorunner.RunProfile{VUs: 2, Duration: 10 * time.Second, Stop: stop}, iter)
		},
		"ramping-vus": func(stop <-chan struct{}) (scenariorunner.Summary, error) {
			return scenariorunner.RunRampingVUs(t.Context(), scenariorunner.RampingVUsProfile{
				StartVUs: 2, Stages: []scenariorunner.Stage{{Duration: 10 * time.Second, Target: 2}}, Stop: stop,
			}, iter)
		},
		"arrival-rate": func(stop <-chan struct{}) (scenariorunner.Summary, error) {
			return scenariorunner.RunArrivalRate(t.Context(), scenariorunner.ArrivalRateProfile{
				StartRate: 100, Stages: []scenariorunner.Stage{{Duration: 10 * time.Second, Target: 100}},
				PreAllocatedVUs: 2, Stop: stop,
			}, iter)
		},
	}

	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			stop := make(chan struct{})
			time.AfterFunc(100*time.Millisecond, func() { close(stop) })

			summary, err := run(stop)
			if err != nil {
				t.Fatalf("error = %v, want nil: a stop is not a failure", err)
			}
			if summary.Elapsed > 2*time.Second {
				t.Errorf("Elapsed = %v: Stop did not end the run", summary.Elapsed)
			}
			if summary.Iterations == 0 {
				t.Error("no iterations ran before the stop")
			}
		})
	}
}

func TestExecutorsRejectInvalidProfiles(t *testing.T) {
	iter := func(context.Context, int, int64) error { return nil }

	if _, err := scenariorunner.RunRampingVUs(t.Context(), scenariorunner.RampingVUsProfile{}, iter); !errors.Is(err, scenariorunner.ErrNoStages) {
		t.Errorf("ramping-vus without stages: error = %v, want ErrNoStages", err)
	}
	if _, err := scenariorunner.RunRampingVUs(t.Context(), scenariorunner.RampingVUsProfile{
		Stages: []scenariorunner.Stage{{Duration: time.Second, Target: 0}},
	}, iter); !errors.Is(err, scenariorunner.ErrInvalidVUs) {
		t.Errorf("ramping-vus that never has a VU: error = %v, want ErrInvalidVUs", err)
	}
	if _, err := scenariorunner.RunArrivalRate(t.Context(), scenariorunner.ArrivalRateProfile{
		Stages: []scenariorunner.Stage{{Duration: time.Second, Target: 1}},
	}, iter); !errors.Is(err, scenariorunner.ErrInvalidVUs) {
		t.Errorf("arrival-rate without VUs: error = %v, want ErrInvalidVUs", err)
	}
	if _, err := scenariorunner.RunArrivalRate(t.Context(), scenariorunner.ArrivalRateProfile{
		PreAllocatedVUs: 1,
	}, iter); !errors.Is(err, scenariorunner.ErrNoStages) {
		t.Errorf("arrival-rate without stages: error = %v, want ErrNoStages", err)
	}
	if _, err := scenariorunner.RunArrivalRate(t.Context(), scenariorunner.ArrivalRateProfile{
		PreAllocatedVUs: 1, Stages: []scenariorunner.Stage{{Duration: time.Second, Target: 1}},
	}, nil); !errors.Is(err, scenariorunner.ErrNilIteration) {
		t.Errorf("nil iteration: error = %v, want ErrNilIteration", err)
	}
}
