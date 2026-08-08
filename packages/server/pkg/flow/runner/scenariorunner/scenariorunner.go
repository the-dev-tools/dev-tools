// Package scenariorunner schedules repeated executions of an arbitrary
// callback across a fixed pool of virtual users (VUs), the way a load
// generator does.
//
// It is deliberately engine-agnostic: it knows nothing about flows, HTTP or
// the rest of the runner packages. Callers supply an iteration function and a
// RunProfile; the scheduler guarantees at most RunProfile.VUs iterations are
// in flight at once and stops issuing new ones once a bound is reached.
package scenariorunner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Configuration errors returned by Run before any work is started.
var (
	ErrInvalidVUs      = errors.New("scenariorunner: VUs must be >= 1")
	ErrNoStopCondition = errors.New("scenariorunner: profile must set Duration or MaxIterations")
	ErrNilIteration    = errors.New("scenariorunner: iteration function must not be nil")
)

// RunProfile describes a constant-VU load profile.
type RunProfile struct {
	// VUs is the number of concurrent workers. Must be >= 1.
	VUs int
	// Duration bounds the window during which new iterations are issued.
	// Values <= 0 mean unbounded, in which case MaxIterations must be set.
	Duration time.Duration
	// MaxIterations bounds the total number of iterations issued. Values <= 0
	// mean unbounded, in which case Duration must be set.
	MaxIterations int64
}

// Summary reports what a scenario actually did.
type Summary struct {
	// Iterations is the number of iterations that ran to completion.
	Iterations int64
	// Errors is how many of those iterations returned a non-nil error.
	Errors int64
	// Elapsed is the wall-clock time from start until the last worker exited.
	Elapsed time.Duration
}

// Run executes iter repeatedly across prof.VUs workers until the profile's
// duration or iteration bound is reached, or ctx is canceled.
//
// Each call receives the zero-based index of the worker running it and a
// scenario-wide sequence number; sequence numbers are handed out exactly once,
// contiguously from zero. At most prof.VUs calls are in flight at any moment.
//
// Errors returned by iter are counted in Summary.Errors and never abort the
// scenario. Run stops issuing new iterations when a bound is hit or ctx is
// canceled, then drains the iterations already in flight before returning, so
// no worker outlives the call.
//
// Run returns ctx.Err() if the context is done once the scenario ends, which is
// normally because cancellation stopped it early; the Summary still reports the
// work completed up to that point. Configuration problems are reported before
// any iteration runs, alongside a zero Summary.
//
// A panic inside iter is not recovered: it crashes the process, as it would
// anywhere else in the engine.
func Run(ctx context.Context, prof RunProfile, iter func(ctx context.Context, vu int, seq int64) error) (Summary, error) {
	if err := validate(prof, iter); err != nil {
		return Summary{}, err
	}

	var (
		next       atomic.Int64 // sequence number the next iteration will claim
		iterations atomic.Int64
		errCount   atomic.Int64
	)

	start := time.Now()

	var deadline time.Time
	if prof.Duration > 0 {
		deadline = start.Add(prof.Duration)
	}

	// Iteration errors must not cancel sibling workers, so a plain WaitGroup is
	// used rather than errgroup: there is no error to propagate.
	var wg sync.WaitGroup
	wg.Add(prof.VUs)
	for vu := range prof.VUs {
		go func() {
			defer wg.Done()
			for {
				seq, ok := claim(ctx, &next, prof.MaxIterations, deadline)
				if !ok {
					return
				}
				if err := iter(ctx, vu, seq); err != nil {
					errCount.Add(1)
				}
				iterations.Add(1)
			}
		}()
	}
	wg.Wait()

	summary := Summary{
		Iterations: iterations.Load(),
		Errors:     errCount.Load(),
		Elapsed:    time.Since(start),
	}
	return summary, ctx.Err()
}

// claim reserves the next sequence number, or reports that the worker should
// stop.
//
// Cancellation and the duration deadline are checked before the reservation, so
// no sequence number is burned once either has tripped. The iteration bound is
// enforced after it instead: an over-limit claim is discarded rather than run.
// next therefore overruns MaxIterations by up to VUs, which is harmless because
// Summary counts iterations that executed, not claims that were made.
//
// The post-reservation discard is what makes Iterations == MaxIterations exact.
// Do not drop it on the assumption that a pre-check covers the bound; checking
// the bound before the atomic add would let several workers read the same value
// and overshoot.
func claim(ctx context.Context, next *atomic.Int64, maxIterations int64, deadline time.Time) (int64, bool) {
	if ctx.Err() != nil {
		return 0, false
	}
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		return 0, false
	}
	seq := next.Add(1) - 1
	if maxIterations > 0 && seq >= maxIterations {
		return 0, false
	}
	return seq, true
}

func validate(prof RunProfile, iter func(ctx context.Context, vu int, seq int64) error) error {
	if prof.VUs < 1 {
		return ErrInvalidVUs
	}
	if prof.Duration <= 0 && prof.MaxIterations <= 0 {
		return ErrNoStopCondition
	}
	if iter == nil {
		return ErrNilIteration
	}
	return nil
}
