package scenariorunner

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoStages is returned by the ramping and arrival-rate executors when the
// profile has no stage with a positive duration.
var ErrNoStages = errors.New("scenariorunner: profile must have at least one stage with a positive duration")

const (
	defaultGracefulStop     = 30 * time.Second
	defaultGracefulRampDown = 30 * time.Second
	// rampTick is how often ramping-vus re-evaluates its VU target. It bounds
	// how late a VU joins or leaves relative to the ideal linear ramp.
	rampTick = 10 * time.Millisecond
)

// Stage is one leg of a ramping profile: over Duration the target moves
// linearly from the previous stage's target (or the profile's start value)
// to Target.
type Stage struct {
	Duration time.Duration
	Target   float64
}

// Live exposes gauges a caller can sample while a scenario runs, e.g. to
// stream them alongside interval metrics. All executors maintain it when a
// profile carries one.
type Live struct {
	// ActiveVUs is the number of VUs executing an iteration right now.
	ActiveVUs atomic.Int64
	// Dropped is the number of arrival-rate iterations dropped so far
	// because every VU up to MaxVUs was busy.
	Dropped atomic.Int64
}

func (l *Live) enter() {
	if l != nil {
		l.ActiveVUs.Add(1)
	}
}

func (l *Live) leave() {
	if l != nil {
		l.ActiveVUs.Add(-1)
	}
}

func (l *Live) drop() {
	if l != nil {
		l.Dropped.Add(1)
	}
}

// stopped reports whether stop has been closed. A nil channel never is.
func stopped(stop <-chan struct{}) bool {
	if stop == nil {
		return false
	}
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// RampingVUsProfile varies the number of concurrently looping VUs over a
// list of stages (a closed model whose population changes over time).
type RampingVUsProfile struct {
	// StartVUs is the VU count at time zero.
	StartVUs int
	// Stages define the VU target over time; the scenario ends when the
	// last stage does.
	Stages []Stage
	// GracefulRampDown bounds how long an iteration may keep running after
	// its VU was ramped away before it is interrupted. Zero means 30s.
	GracefulRampDown time.Duration
	// GracefulStop bounds how long in-flight iterations may keep running
	// once the last stage ends, or Stop is closed. Zero means 30s.
	GracefulStop time.Duration
	// Stop, when closed, ends the scenario early as if its schedule had run
	// out. Nil means never.
	Stop <-chan struct{}
	// Live, when set, is kept up to date while the scenario runs.
	Live *Live
}

// MaxVUs is the most VUs the profile is ever active with, i.e. how many
// distinct VU indices iterations may receive.
func (p RampingVUsProfile) MaxVUs() int {
	highest := p.StartVUs
	for _, s := range p.Stages {
		highest = max(highest, int(math.Ceil(s.Target)))
	}
	return highest
}

// rampingTarget is the number of VUs that should be active at elapsed: the
// linear interpolation across the stage containing elapsed, rounded down, or
// the last target once the stages have run out.
func rampingTarget(start int, stages []Stage, elapsed time.Duration) int {
	from := float64(start)
	for _, s := range stages {
		if elapsed < s.Duration {
			frac := float64(elapsed) / float64(s.Duration)
			return int(math.Floor(from + (s.Target-from)*frac + 1e-9))
		}
		elapsed -= s.Duration
		from = s.Target
	}
	return int(math.Floor(from + 1e-9))
}

func totalDuration(stages []Stage) time.Duration {
	var total time.Duration
	for _, s := range stages {
		if s.Duration > 0 {
			total += s.Duration
		}
	}
	return total
}

// vuSlot tracks the iteration a ramping VU is running, so the controller can
// interrupt it once a graceful ramp-down expires.
type vuSlot struct {
	mu     sync.Mutex
	gen    int64 // incremented per iteration
	busy   bool
	cancel context.CancelFunc
}

// RunRampingVUs executes iter on a VU population that follows prof.Stages.
//
// VU i loops while the current target is above i, so ramping up adds VUs in
// index order and ramping down retires the highest indices first. A retired
// VU's in-flight iteration is allowed GracefulRampDown to finish; past that
// its context is canceled and it counts as Interrupted rather than as a
// completed or failed iteration. When the last stage ends (or Stop is
// closed), in-flight iterations get GracefulStop, after which they are
// interrupted too.
//
// Errors and cancellation behave as in Run.
func RunRampingVUs(ctx context.Context, prof RampingVUsProfile, iter func(ctx context.Context, vu int, seq int64) error) (Summary, error) {
	if iter == nil {
		return Summary{}, ErrNilIteration
	}
	total := totalDuration(prof.Stages)
	if total <= 0 {
		return Summary{}, ErrNoStages
	}
	maxVUs := prof.MaxVUs()
	if maxVUs < 1 || prof.StartVUs < 0 {
		return Summary{}, ErrInvalidVUs
	}
	rampDown := prof.GracefulRampDown
	if rampDown <= 0 {
		rampDown = defaultGracefulRampDown
	}
	graceful := prof.GracefulStop
	if graceful <= 0 {
		graceful = defaultGracefulStop
	}

	// hardCtx is canceled once the graceful stop expires, interrupting
	// whatever is still running.
	hardCtx, hardCancel := context.WithCancel(ctx)
	defer hardCancel()

	var (
		seq         atomic.Int64
		iterations  atomic.Int64
		errCount    atomic.Int64
		interrupted atomic.Int64
		target      atomic.Int64
		ended       atomic.Bool

		mu      sync.Mutex
		changed = make(chan struct{})
	)
	slots := make([]vuSlot, maxVUs)
	start := time.Now()
	target.Store(int64(rampingTarget(prof.StartVUs, prof.Stages, 0)))

	// broadcast wakes every VU waiting for the target to change.
	broadcast := func() {
		mu.Lock()
		close(changed)
		changed = make(chan struct{})
		mu.Unlock()
	}
	waitChange := func() <-chan struct{} {
		mu.Lock()
		defer mu.Unlock()
		return changed
	}

	var wg sync.WaitGroup
	wg.Add(maxVUs)
	for vu := range maxVUs {
		go func() {
			defer wg.Done()
			slot := &slots[vu]
			for {
				// Read the channel before the target, so a change between the
				// two still wakes this VU.
				wake := waitChange()
				if ended.Load() || hardCtx.Err() != nil {
					return
				}
				if int64(vu) >= target.Load() {
					select {
					case <-wake:
					case <-hardCtx.Done():
						return
					}
					continue
				}

				iterCtx, cancel := context.WithCancel(hardCtx)
				slot.mu.Lock()
				slot.gen++
				slot.busy = true
				slot.cancel = cancel
				slot.mu.Unlock()

				prof.Live.enter()
				err := iter(iterCtx, vu, seq.Add(1)-1)
				prof.Live.leave()

				slot.mu.Lock()
				slot.busy = false
				slot.cancel = nil
				slot.mu.Unlock()
				wasInterrupted := iterCtx.Err() != nil
				cancel()

				switch {
				case wasInterrupted:
					interrupted.Add(1)
				case err != nil:
					errCount.Add(1)
					iterations.Add(1)
				default:
					iterations.Add(1)
				}
			}
		}()
	}

	// The controller moves the target along the stages and retires VUs.
	var timersMu sync.Mutex
	var timers []*time.Timer
	retire := func(from, to int) {
		for vu := from; vu < to && vu < maxVUs; vu++ {
			slot := &slots[vu]
			slot.mu.Lock()
			if slot.busy {
				gen := slot.gen
				timer := time.AfterFunc(rampDown, func() {
					slot.mu.Lock()
					defer slot.mu.Unlock()
					if slot.busy && slot.gen == gen && slot.cancel != nil {
						slot.cancel()
					}
				})
				timersMu.Lock()
				timers = append(timers, timer)
				timersMu.Unlock()
			}
			slot.mu.Unlock()
		}
	}

	ticker := time.NewTicker(rampTick)
	for {
		elapsed := time.Since(start)
		if elapsed >= total || stopped(prof.Stop) || ctx.Err() != nil {
			break
		}
		next := int64(rampingTarget(prof.StartVUs, prof.Stages, elapsed))
		if prev := target.Swap(next); next != prev {
			if next < prev {
				retire(int(next), int(prev))
			}
			broadcast()
		}
		select {
		case <-ticker.C:
		case <-prof.Stop:
		case <-ctx.Done():
		}
	}
	ticker.Stop()

	ended.Store(true)
	broadcast()
	drainWithGrace(&wg, graceful, hardCancel)

	timersMu.Lock()
	for _, timer := range timers {
		timer.Stop()
	}
	timersMu.Unlock()

	return Summary{
		Iterations:  iterations.Load(),
		Errors:      errCount.Load(),
		Interrupted: interrupted.Load(),
		Elapsed:     time.Since(start),
	}, ctx.Err()
}

// drainWithGrace waits for wg, canceling in-flight work through cancel once
// grace has passed.
func drainWithGrace(wg *sync.WaitGroup, grace time.Duration, cancel context.CancelFunc) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		cancel()
		<-done
	}
}

// ArrivalRateProfile starts iterations on a schedule, independent of how long
// they take (an open model).
type ArrivalRateProfile struct {
	// StartRate is the iteration start rate at time zero, per TimeUnit.
	StartRate float64
	// Stages define the rate over time, interpolated linearly. A constant
	// rate is a single stage whose Target equals StartRate.
	Stages []Stage
	// TimeUnit is the period rates are expressed over. Zero means 1s.
	TimeUnit time.Duration
	// PreAllocatedVUs are available from the start.
	PreAllocatedVUs int
	// MaxVUs caps the VU pool. A start that finds every VU busy and the pool
	// at MaxVUs is dropped. Zero means PreAllocatedVUs.
	MaxVUs int
	// GracefulStop bounds how long in-flight iterations may keep running
	// once the schedule ends, or Stop is closed. Zero means 30s.
	GracefulStop time.Duration
	// Stop, when closed, ends the schedule early. Nil means never.
	Stop <-chan struct{}
	// Live, when set, is kept up to date while the scenario runs.
	Live *Live
}

// PoolSize is the most VUs the profile may use, i.e. how many distinct VU
// indices iterations may receive.
func (p ArrivalRateProfile) PoolSize() int {
	return max(p.MaxVUs, p.PreAllocatedVUs)
}

// RunArrivalRate starts iterations at the times prof's rate schedule
// dictates, each on an idle VU. When no VU is idle, the pool grows up to
// MaxVUs; past that the start is dropped and counted in Summary.Dropped.
// A start the dispatcher reaches late (because the machine was busy) runs
// immediately rather than being skipped.
//
// VU indices are handed out lowest-first: indices below PreAllocatedVUs are
// available immediately, higher ones only once the pool grows.
//
// Errors and cancellation behave as in Run; in-flight iterations are given
// GracefulStop once the schedule ends and are interrupted after that.
func RunArrivalRate(ctx context.Context, prof ArrivalRateProfile, iter func(ctx context.Context, vu int, seq int64) error) (Summary, error) {
	if iter == nil {
		return Summary{}, ErrNilIteration
	}
	if prof.PreAllocatedVUs < 1 || prof.MaxVUs < 0 {
		return Summary{}, ErrInvalidVUs
	}
	if totalDuration(prof.Stages) <= 0 {
		return Summary{}, ErrNoStages
	}
	unit := prof.TimeUnit
	if unit <= 0 {
		unit = time.Second
	}
	graceful := prof.GracefulStop
	if graceful <= 0 {
		graceful = defaultGracefulStop
	}
	poolSize := prof.PoolSize()

	hardCtx, hardCancel := context.WithCancel(ctx)
	defer hardCancel()

	var (
		iterations  atomic.Int64
		errCount    atomic.Int64
		interrupted atomic.Int64
		dropped     int64
		wg          sync.WaitGroup
	)

	idle := make(chan int, poolSize)
	for vu := range prof.PreAllocatedVUs {
		idle <- vu
	}
	allocated := prof.PreAllocatedVUs

	sched := newArrivalSchedule(prof.StartRate, prof.Stages, unit)
	start := time.Now()
	timer := time.NewTimer(0)
	<-timer.C

dispatch:
	for k := int64(0); ; k++ {
		at, ok := sched.at(k)
		if !ok {
			break
		}
		if wait := time.Until(start.Add(at)); wait > 0 {
			timer.Reset(wait)
			select {
			case <-timer.C:
			case <-prof.Stop:
				timer.Stop()
				break dispatch
			case <-ctx.Done():
				timer.Stop()
				break dispatch
			}
		}
		if stopped(prof.Stop) || ctx.Err() != nil {
			break
		}

		var vu int
		select {
		case vu = <-idle:
		default:
			if allocated >= poolSize {
				dropped++
				prof.Live.drop()
				continue
			}
			vu = allocated
			allocated++
		}

		wg.Add(1)
		go func(vu int, seq int64) {
			defer wg.Done()
			iterCtx, cancel := context.WithCancel(hardCtx)
			prof.Live.enter()
			err := iter(iterCtx, vu, seq)
			prof.Live.leave()
			wasInterrupted := iterCtx.Err() != nil
			cancel()

			switch {
			case wasInterrupted:
				interrupted.Add(1)
			case err != nil:
				errCount.Add(1)
				iterations.Add(1)
			default:
				iterations.Add(1)
			}
			idle <- vu
		}(vu, k)
	}

	drainWithGrace(&wg, graceful, hardCancel)

	return Summary{
		Iterations:  iterations.Load(),
		Errors:      errCount.Load(),
		Interrupted: interrupted.Load(),
		Dropped:     dropped,
		Elapsed:     time.Since(start),
	}, ctx.Err()
}

// arrivalSchedule maps an iteration's ordinal to its start offset. Rates are
// held per nanosecond internally; the cumulative arrivals by time t are the
// area under the piecewise-linear rate curve, and the k-th iteration starts
// when that area reaches k.
type arrivalSchedule struct {
	legs []arrivalLeg
}

type arrivalLeg struct {
	offset   time.Duration // when the leg starts
	duration time.Duration
	from, to float64 // rate per nanosecond at the leg's start and end
	before   float64 // arrivals scheduled before the leg starts
	arrivals float64 // arrivals scheduled within the leg
}

func newArrivalSchedule(startRate float64, stages []Stage, unit time.Duration) *arrivalSchedule {
	perNano := 1 / float64(unit)
	from := startRate * perNano
	var (
		offset time.Duration
		before float64
		legs   = make([]arrivalLeg, 0, len(stages))
	)
	for _, s := range stages {
		to := s.Target * perNano
		if s.Duration <= 0 {
			from = to
			continue
		}
		arrivals := (from + to) / 2 * float64(s.Duration)
		legs = append(legs, arrivalLeg{
			offset: offset, duration: s.Duration,
			from: from, to: to, before: before, arrivals: arrivals,
		})
		offset += s.Duration
		before += arrivals
		from = to
	}
	return &arrivalSchedule{legs: legs}
}

// arrivalEpsilon absorbs floating-point error in the cumulative arrival
// counts, so a schedule of exactly N starts does not gain or lose one.
const arrivalEpsilon = 1e-9

// at returns the start offset of iteration k, or false once the schedule has
// no k-th start.
func (s *arrivalSchedule) at(k int64) (time.Duration, bool) {
	want := float64(k)
	for _, leg := range s.legs {
		if want >= leg.before+leg.arrivals-arrivalEpsilon {
			continue
		}
		remaining := max(want-leg.before, 0)
		// Solve from*t + (to-from)/(2*duration)*t^2 = remaining for t, in
		// the form that stays stable when the rate is flat or falling.
		a := (leg.to - leg.from) / (2 * float64(leg.duration))
		b := leg.from
		denominator := b + math.Sqrt(b*b+4*a*remaining)
		var t float64
		if denominator > 0 {
			t = 2 * remaining / denominator
		}
		return leg.offset + time.Duration(math.Round(t)), true
	}
	return 0, false
}
