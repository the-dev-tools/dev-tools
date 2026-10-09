// Package mload holds the domain model for load-test scenarios: the `load:`
// block of a yamlflow document, decoded into engine-ready values.
//
// It deliberately knows nothing about YAML or about the load runner. The
// yamlflow translator produces these, and the CLI's load runner consumes
// them, so neither has to depend on the other.
package mload

import "time"

// Executor names the scheduling strategy a scenario uses.
type Executor string

const (
	// ExecutorConstantVUs holds a fixed number of virtual users for the
	// scenario's duration or iteration budget. Each VU starts its next
	// iteration as soon as the previous one finishes (a closed model).
	ExecutorConstantVUs Executor = "constant-vus"
	// ExecutorRampingVUs varies the number of active virtual users over a
	// list of stages, interpolating linearly from one target to the next.
	ExecutorRampingVUs Executor = "ramping-vus"
	// ExecutorConstantArrivalRate starts iterations at a fixed rate,
	// independent of how long they take (an open model). Iterations that
	// find no free VU are dropped and counted.
	ExecutorConstantArrivalRate Executor = "constant-arrival-rate"
	// ExecutorRampingArrivalRate is the open-model counterpart of
	// ramping-vus: the iteration start rate follows a list of stages.
	ExecutorRampingArrivalRate Executor = "ramping-arrival-rate"
)

// SupportedExecutors lists the executors this build accepts, for error
// messages that have to name the valid alternatives.
var SupportedExecutors = []Executor{
	ExecutorConstantVUs,
	ExecutorRampingVUs,
	ExecutorConstantArrivalRate,
	ExecutorRampingArrivalRate,
}

// IsArrivalRate reports whether e is an open-model executor, i.e. one that
// starts iterations on a schedule rather than whenever a VU is free.
func (e Executor) IsArrivalRate() bool {
	return e == ExecutorConstantArrivalRate || e == ExecutorRampingArrivalRate
}

// Defaults applied when a scenario leaves the corresponding field unset.
const (
	// DefaultTimeUnit is the period a `rate` is expressed over.
	DefaultTimeUnit = time.Second
	// DefaultGracefulStop is how long in-flight iterations of a ramping or
	// arrival-rate scenario may keep running once the schedule ends, before
	// they are interrupted.
	DefaultGracefulStop = 30 * time.Second
	// DefaultGracefulRampDown is how long an iteration may keep running after
	// a ramping-vus stage has taken its VU away.
	DefaultGracefulRampDown = 30 * time.Second
	// DefaultAbortWindow is the trailing window an abort condition is
	// evaluated over when it does not name one.
	DefaultAbortWindow = 30 * time.Second
)

// Stage is one leg of a ramping profile: over Duration, the target moves
// linearly from the previous stage's target (or the scenario's start value)
// to Target. For ramping-vus Target is a VU count; for ramping-arrival-rate it
// is iterations per TimeUnit.
type Stage struct {
	Duration time.Duration
	Target   float64
}

// ThinkTime is the pause a VU takes after each iteration. Min == Max is a
// fixed pause; Min < Max draws uniformly from the range; zero is none.
type ThinkTime struct {
	Min time.Duration
	Max time.Duration
}

// IsZero reports whether no think time is configured.
func (t ThinkTime) IsZero() bool { return t.Min == 0 && t.Max == 0 }

// Scenario is one entry of the `load:` block: a named load profile applied to
// an existing flow. Flows are never edited to be load-tested, so a Scenario
// refers to its flow by name rather than owning it.
//
// Which fields are meaningful depends on Executor:
//
//   - constant-vus: VUs, plus Duration and/or MaxIterations.
//   - ramping-vus: StartVUs, Stages, GracefulRampDown.
//   - constant-arrival-rate: Rate, TimeUnit, Duration, PreAllocatedVUs, MaxVUs.
//   - ramping-arrival-rate: StartRate, TimeUnit, Stages, PreAllocatedVUs, MaxVUs.
//
// GracefulStop, ThinkTime, Thresholds and Abort apply to every executor
// (GracefulStop is ignored by constant-vus, which always lets in-flight
// iterations finish).
type Scenario struct {
	// Name identifies the scenario, e.g. for `flow run --scenario <name>`.
	Name string
	// FlowName is the flow this scenario drives, by its `flows:` entry name.
	FlowName string
	// Executor is the scheduling strategy.
	Executor Executor
	// VUs is the number of concurrent virtual users for constant-vus.
	VUs int
	// Duration bounds the window during which new iterations start, for
	// constant-vus and constant-arrival-rate. Zero means unbounded, in which
	// case MaxIterations is set (constant-vus only).
	Duration time.Duration
	// MaxIterations bounds the total iterations issued (constant-vus only).
	// Zero means unbounded, in which case Duration is set.
	MaxIterations int64

	// StartVUs is the VU count ramping-vus starts from.
	StartVUs int
	// Stages drive ramping-vus (VU targets) and ramping-arrival-rate (rate
	// targets).
	Stages []Stage
	// GracefulRampDown bounds how long a ramping-vus iteration may keep
	// running after its VU was ramped away. Zero means the default.
	GracefulRampDown time.Duration

	// Rate is the iterations started per TimeUnit for constant-arrival-rate.
	Rate float64
	// StartRate is the rate ramping-arrival-rate starts from.
	StartRate float64
	// TimeUnit is the period Rate, StartRate and arrival stage targets are
	// expressed over. Zero means DefaultTimeUnit.
	TimeUnit time.Duration
	// PreAllocatedVUs are built before an arrival-rate scenario starts.
	PreAllocatedVUs int
	// MaxVUs caps how many VUs an arrival-rate scenario may use. Iterations
	// that would need more are dropped. Zero means PreAllocatedVUs.
	MaxVUs int

	// GracefulStop bounds how long in-flight iterations may run once a
	// ramping or arrival-rate schedule ends. Zero means the default.
	GracefulStop time.Duration
	// ThinkTime is the pause after each iteration.
	ThinkTime ThinkTime

	// Thresholds are pass/fail conditions evaluated once the run ends.
	Thresholds []Condition
	// Abort conditions are evaluated every reporting interval; the first
	// one that holds stops the run early.
	Abort []AbortRule
}
