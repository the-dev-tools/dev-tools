// Package loadrun executes a flow as a load scenario: N virtual users each
// running the flow in a loop, with per-request latency and outcome aggregated
// into a merged report.
//
// It is the wiring layer between three pieces that know nothing about each
// other - the VU scheduler (scenariorunner), the flow engine
// (flowlocalrunner) and the metrics envelope (loadmetrics). It deliberately
// contains no YAML parsing (that lives in yamlflowsimplev2) and no
// presentation (that lives in the reporter).
//
// # What a load run costs
//
// A load run reads the database exactly once, at setup: the flow's nodes,
// edges and variables, and then one node graph per VU. The iteration loop
// itself holds no database or service handle at all - see vuWorker's fields -
// and the per-iteration response persistence side-channel is drained and
// discarded rather than written. (Sub-flow nodes are the exception: they
// resolve their target through the services they captured at build time, so a
// flow containing them does read the database per iteration.)
//
// Rebuilding the node graph every iteration was measured and rejected: node
// implementations hold configuration only, all per-execution mutable state
// lives in node.FlowNodeRequest (built fresh by each Run) and in the variable
// map (deep-copied per iteration, ~32ns), so a rebuild buys no isolation. It
// costs ~52% of a zero-latency iteration for a three-node flow, and more as
// flows grow, since its cost scales with node count.
//
// # Memory flatness is request-node-scoped
//
// Lean mode - which is always on for load runs - drops decoded response
// bodies from request nodes once assertions have run. It does not propagate
// into sub-flows (that needs an ExecuteSubFlow signature change), and GraphQL
// and WebSocket nodes do not implement it. Flows containing those still run
// under load; their memory does not stay flat, and their requests are not
// counted in the report.
package loadrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/reporter"
	"github.com/the-dev-tools/dev-tools/apps/cli/internal/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	flowrunner "github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/scenariorunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/loadmetrics"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// defaultNodeTimeout matches the CLI's functional run path, so a step that
// would time out in a normal run times out the same way under load.
const defaultNodeTimeout = 60 * time.Second

// DefaultFrameInterval is how often a load run flushes its metrics into an
// interval frame: the cadence frames are streamed at and abort rules are
// evaluated at.
const DefaultFrameInterval = 5 * time.Second

// Errors a run that executed can end with. Both arrive alongside a complete
// Result, so the report is still written.
var (
	// ErrThresholdsFailed means the run completed but at least one
	// threshold did not hold.
	ErrThresholdsFailed = errors.New("load run: thresholds failed")
	// ErrAborted means the run was stopped early, by an abort rule or by a
	// Stopper (for example the frame reporter's dead-man switch).
	ErrAborted = errors.New("load run: aborted")
)

// Config is a resolved load profile: what to run, how it is scheduled, what
// it must achieve, and the hooks that observe it while it runs.
type Config struct {
	// ScenarioName is the `load:` block entry this profile came from, or ""
	// when the profile was assembled from --vus/--duration/--iterations.
	ScenarioName string
	// Flow is the already-imported flow to drive.
	Flow *mflow.Flow
	// Executor is the scheduling strategy. "" means constant-vus.
	Executor mload.Executor
	// VUs is the number of concurrent virtual users (constant-vus).
	VUs int
	// Duration bounds the window during which new iterations start
	// (constant-vus and constant-arrival-rate).
	Duration time.Duration
	// MaxIterations bounds the total iterations issued (constant-vus).
	MaxIterations int64

	// StartVUs, Stages and GracefulRampDown drive ramping-vus; Stages also
	// drives ramping-arrival-rate.
	StartVUs         int
	Stages           []mload.Stage
	GracefulRampDown time.Duration
	// Rate, StartRate, TimeUnit, PreAllocatedVUs and MaxVUs drive the
	// arrival-rate executors.
	Rate            float64
	StartRate       float64
	TimeUnit        time.Duration
	PreAllocatedVUs int
	MaxVUs          int
	// GracefulStop bounds in-flight iterations once a ramping or
	// arrival-rate schedule ends.
	GracefulStop time.Duration
	// ThinkTime is the pause each VU takes after an iteration.
	ThinkTime mload.ThinkTime

	// Thresholds are evaluated once, against the final report.
	Thresholds []mload.Condition
	// Abort rules are evaluated every FrameInterval.
	Abort []mload.AbortRule

	// FrameInterval is how often metrics are flushed into an interval
	// frame. Zero means DefaultFrameInterval.
	FrameInterval time.Duration
	// OnFrame, when set, receives every interval frame, including the final
	// partial one. It is called from the run's metrics goroutine and must
	// not block.
	OnFrame func(IntervalFrame)
	// Stopper, when set, lets a caller end the run early; Run creates its
	// own otherwise. Either way abort rules stop the run through it.
	Stopper *Stopper
}

// IntervalFrame is one interval's metrics, combined across every VU. Its
// Frame keeps a histogram per (step, status class), so frames from several
// machines can be combined losslessly.
type IntervalFrame struct {
	// Seq numbers frames from zero, contiguously.
	Seq   int64
	Frame loadmetrics.Frame
	// ActiveVUs is the number of VUs mid-iteration when the frame was cut.
	ActiveVUs int64
	// Dropped is the cumulative count of dropped arrival-rate iterations.
	Dropped int64
	// Final marks the last frame of the run.
	Final bool
}

// ConfigFromScenario adapts a `load:` block scenario to a runnable Config.
// The flow must be the one the scenario names; resolving the name is the
// caller's job, since only it knows the imported workspace.
func ConfigFromScenario(scenario mload.Scenario, flow *mflow.Flow) Config {
	return Config{
		ScenarioName:     scenario.Name,
		Flow:             flow,
		Executor:         scenario.Executor,
		VUs:              scenario.VUs,
		Duration:         scenario.Duration,
		MaxIterations:    scenario.MaxIterations,
		StartVUs:         scenario.StartVUs,
		Stages:           scenario.Stages,
		GracefulRampDown: scenario.GracefulRampDown,
		Rate:             scenario.Rate,
		StartRate:        scenario.StartRate,
		TimeUnit:         scenario.TimeUnit,
		PreAllocatedVUs:  scenario.PreAllocatedVUs,
		MaxVUs:           scenario.MaxVUs,
		GracefulStop:     scenario.GracefulStop,
		ThinkTime:        scenario.ThinkTime,
		Thresholds:       scenario.Thresholds,
		Abort:            scenario.Abort,
	}
}

// executor returns the configured executor, defaulting to constant-vus.
func (c Config) executor() mload.Executor {
	if c.Executor == "" {
		return mload.ExecutorConstantVUs
	}
	return c.Executor
}

func (c Config) validate() error {
	if c.Flow == nil {
		return errors.New("load run: flow is required")
	}
	switch c.executor() {
	case mload.ExecutorConstantVUs:
		if c.VUs < 1 {
			return fmt.Errorf("load run: vus must be >= 1, got %d", c.VUs)
		}
		if c.Duration <= 0 && c.MaxIterations <= 0 {
			return errors.New("load run: needs a stop condition, set duration or iterations")
		}
	case mload.ExecutorRampingVUs:
		if len(c.Stages) == 0 {
			return errors.New("load run: ramping-vus needs stages")
		}
		if c.PoolSize() < 1 {
			return errors.New("load run: ramping-vus never reaches a VU")
		}
	case mload.ExecutorConstantArrivalRate, mload.ExecutorRampingArrivalRate:
		if c.PreAllocatedVUs < 1 {
			return fmt.Errorf("load run: pre_allocated_vus must be >= 1, got %d", c.PreAllocatedVUs)
		}
		if c.executor() == mload.ExecutorConstantArrivalRate && (c.Rate <= 0 || c.Duration <= 0) {
			return errors.New("load run: constant-arrival-rate needs a positive rate and duration")
		}
		if c.executor() == mload.ExecutorRampingArrivalRate && len(c.Stages) == 0 {
			return errors.New("load run: ramping-arrival-rate needs stages")
		}
	default:
		return fmt.Errorf("load run: unsupported executor %q", c.Executor)
	}
	if c.ThinkTime.Min < 0 || c.ThinkTime.Max < c.ThinkTime.Min {
		return fmt.Errorf("load run: invalid think time %v..%v", c.ThinkTime.Min, c.ThinkTime.Max)
	}
	return nil
}

// PoolSize is the most VUs the profile can use at once, i.e. how many VU
// workers it may need.
func (c Config) PoolSize() int {
	switch c.executor() {
	case mload.ExecutorRampingVUs:
		return c.rampingProfile(nil, nil).MaxVUs()
	case mload.ExecutorConstantArrivalRate, mload.ExecutorRampingArrivalRate:
		return max(c.MaxVUs, c.PreAllocatedVUs)
	default:
		return c.VUs
	}
}

// preBuilt is how many VU workers are built before the run starts. Arrival
// rate scenarios build their pre-allocated VUs up front and the rest of the
// pool on demand; the closed-model executors build their whole pool.
func (c Config) preBuilt() int {
	if c.executor().IsArrivalRate() {
		return c.PreAllocatedVUs
	}
	return c.PoolSize()
}

func toRunnerStages(stages []mload.Stage) []scenariorunner.Stage {
	out := make([]scenariorunner.Stage, 0, len(stages))
	for _, s := range stages {
		out = append(out, scenariorunner.Stage{Duration: s.Duration, Target: s.Target})
	}
	return out
}

func (c Config) rampingProfile(stop <-chan struct{}, live *scenariorunner.Live) scenariorunner.RampingVUsProfile {
	return scenariorunner.RampingVUsProfile{
		StartVUs:         c.StartVUs,
		Stages:           toRunnerStages(c.Stages),
		GracefulRampDown: c.GracefulRampDown,
		GracefulStop:     c.GracefulStop,
		Stop:             stop,
		Live:             live,
	}
}

func (c Config) arrivalProfile(stop <-chan struct{}, live *scenariorunner.Live) scenariorunner.ArrivalRateProfile {
	startRate, stages := c.StartRate, toRunnerStages(c.Stages)
	if c.executor() == mload.ExecutorConstantArrivalRate {
		startRate = c.Rate
		stages = []scenariorunner.Stage{{Duration: c.Duration, Target: c.Rate}}
	}
	return scenariorunner.ArrivalRateProfile{
		StartRate:       startRate,
		Stages:          stages,
		TimeUnit:        c.TimeUnit,
		PreAllocatedVUs: c.PreAllocatedVUs,
		MaxVUs:          c.MaxVUs,
		GracefulStop:    c.GracefulStop,
		Stop:            stop,
		Live:            live,
	}
}

// schedule runs iter under the configured executor.
func (c Config) schedule(
	ctx context.Context,
	stop <-chan struct{},
	live *scenariorunner.Live,
	iter func(ctx context.Context, vu int, seq int64) error,
) (scenariorunner.Summary, error) {
	switch c.executor() {
	case mload.ExecutorRampingVUs:
		return scenariorunner.RunRampingVUs(ctx, c.rampingProfile(stop, live), iter)
	case mload.ExecutorConstantArrivalRate, mload.ExecutorRampingArrivalRate:
		return scenariorunner.RunArrivalRate(ctx, c.arrivalProfile(stop, live), iter)
	default:
		// Duration is passed through RunProfile only. Deriving it from a
		// context deadline instead would make scenariorunner.Run return
		// ctx.Err() at the end of every successful timed run, since it
		// reports the caller's context state on the way out.
		return scenariorunner.Run(ctx, scenariorunner.RunProfile{
			VUs:           c.VUs,
			Duration:      c.Duration,
			MaxIterations: c.MaxIterations,
			Stop:          stop,
			Live:          live,
		}, iter)
	}
}

// Result is everything a completed load run produced.
type Result struct {
	// Config is the profile that was executed.
	Config Config
	// Summary is the scheduler's view: iterations completed, iterations that
	// returned an error, interrupted and dropped iterations, wall time.
	Summary scenariorunner.Summary
	// Report is the merged metrics report keyed by (step, status class).
	Report loadmetrics.Report
	// ByStep is the same data folded across status classes, so each step has
	// exactly one row. This is what the console table renders.
	ByStep loadmetrics.Report
	// Thresholds holds one verdict per configured threshold, in order.
	Thresholds []ThresholdResult
	// AbortReason says why the run stopped early, or is "" when it ran its
	// course.
	AbortReason string
	// Frames is how many interval frames the run produced.
	Frames int64
}

// ThresholdsPassed reports whether every threshold held. It is true when
// none were configured.
func (r Result) ThresholdsPassed() bool {
	for _, t := range r.Thresholds {
		if !t.Passed {
			return false
		}
	}
	return true
}

// Ran reports whether the scenario got as far as executing, and therefore
// whether this Result is worth reporting.
//
// It is true even for runs that ended in an error, because those are exactly
// the runs whose numbers matter most: a soak that failed its first iteration
// per VU and then ran cleanly for half an hour still exits non-zero, but
// throwing its report away would be the worst possible response to it. It is
// false only when Run failed before any iteration could start - invalid
// configuration, or a flow graph that would not build.
func (r Result) Ran() bool {
	return r.Config.Flow != nil
}

// Run executes cfg and returns the merged report.
//
// A completed run is a success even when individual requests failed: request
// errors are data, reported in Summary.Errors and in the report's error
// counts. Run returns an error when the run could not meaningfully happen -
// invalid configuration, a failure setting up the flow graph, or every
// virtual user failing its very first iteration (which means the target was
// never reachable, not that the system under test is slow). A run that
// executed can also end with ErrAborted (an abort rule or the Stopper ended
// it early) and/or ErrThresholdsFailed, alongside its complete Result.
func Run(ctx context.Context, cfg Config, services runner.RunnerServices, logger *slog.Logger) (Result, error) {
	if err := cfg.validate(); err != nil {
		return Result{}, err
	}

	pool, err := newWorkerPool(ctx, cfg, services, logger)
	if err != nil {
		return Result{}, err
	}
	defer pool.release()

	stopper := cfg.Stopper
	if stopper == nil {
		stopper = NewStopper()
	}
	live := &scenariorunner.Live{}
	tracker := newFirstIterationTracker(cfg.PoolSize())

	// An aggregator's interval starts when it is constructed, which was
	// during setup. Flushing the empty setup frame away restarts every
	// interval at the same instant the scenario does, so the wall time the
	// report divides by is the scenario's, not the scenario's plus however
	// long building VUs took.
	startedAt := time.Now()
	for _, w := range pool.built() {
		w.agg.Flush(startedAt)
	}

	metrics := newIntervalMetrics(cfg, pool, live, stopper, startedAt)
	metrics.start()

	summary, runErr := cfg.schedule(ctx, stopper.Done(), live, func(ctx context.Context, vu int, _ int64) error {
		w, err := pool.get(vu)
		if err != nil {
			tracker.observe(vu, err)
			return err
		}
		iterErr := w.iterate(ctx)
		// An iteration the scheduler cut short says nothing about whether
		// the target is reachable.
		if ctx.Err() == nil {
			tracker.observe(vu, iterErr)
		}
		if iterErr == nil {
			w.think(ctx, cfg.ThinkTime, stopper.Done())
		}
		return iterErr
	})

	// The report is assembled before any error is returned, and returned
	// alongside it. Everything below this point describes a run that happened;
	// discarding what it measured because it also ended badly would throw away
	// precisely the numbers someone needs to understand why.
	cumulative, frames := metrics.finish(time.Now())
	result := Result{
		Config:      cfg,
		Summary:     summary,
		Report:      loadmetrics.Merge([]loadmetrics.Frame{cumulative}),
		ByStep:      loadmetrics.Merge(foldByStep([]loadmetrics.Frame{cumulative})),
		AbortReason: stopper.Reason(),
		Frames:      frames,
	}
	result.Thresholds = EvaluateThresholds(cfg.Thresholds, result.ByStep)

	if runErr != nil {
		return result, fmt.Errorf("load run: %w", runErr)
	}
	if err := tracker.setupFailure(); err != nil {
		return result, err
	}

	var outcome []error
	if result.AbortReason != "" {
		outcome = append(outcome, fmt.Errorf("%w: %s", ErrAborted, result.AbortReason))
	}
	if failed := failedThresholds(result.Thresholds); failed != "" {
		outcome = append(outcome, fmt.Errorf("%w: %s", ErrThresholdsFailed, failed))
	}
	return result, errors.Join(outcome...)
}

// foldByStep rewrites frames so every entry's status class is dropped,
// collapsing a step's buckets into one. Each entry becomes its own frame,
// because two entries of the same step would otherwise collide on the shared
// key inside a single frame's map; Merge unions the frames' (identical) time
// ranges, so the folded report's RPS matches the unfolded one.
//
// Histograms are shared with the input frames rather than copied. Merge only
// ever reads them, merging into freshly allocated histograms of its own.
func foldByStep(frames []loadmetrics.Frame) []loadmetrics.Frame {
	folded := make([]loadmetrics.Frame, 0, len(frames))
	for _, f := range frames {
		for key, entry := range f.Entries {
			folded = append(folded, loadmetrics.Frame{
				IntervalStart: f.IntervalStart,
				Interval:      f.Interval,
				Entries:       map[loadmetrics.Key]loadmetrics.Entry{{Step: key.Step}: entry},
			})
		}
		if len(f.Entries) == 0 {
			// Keep the empty frame so the merged wall time - and therefore
			// RPS - still covers this VU's window.
			folded = append(folded, f)
		}
	}
	return folded
}

// firstIterationTracker records how each VU's first iteration went, which is
// what distinguishes "the target was never up" from "the target is failing
// some requests".
type firstIterationTracker struct {
	mu      sync.Mutex
	outcome []*bool // nil until the VU has run its first iteration
}

func newFirstIterationTracker(vus int) *firstIterationTracker {
	return &firstIterationTracker{outcome: make([]*bool, vus)}
}

func (t *firstIterationTracker) observe(vu int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if vu < 0 || vu >= len(t.outcome) || t.outcome[vu] != nil {
		return
	}
	ok := err == nil
	t.outcome[vu] = &ok
}

// setupFailure reports an error when every VU that got as far as running an
// iteration failed on that first attempt. A VU that never ran (because the
// iteration budget was exhausted by its siblings) is not evidence either way.
func (t *firstIterationTracker) setupFailure() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ran := 0
	for _, outcome := range t.outcome {
		if outcome == nil {
			continue
		}
		ran++
		if *outcome {
			return nil
		}
	}
	if ran == 0 {
		return nil
	}
	return fmt.Errorf(
		"load run: every virtual user (%d of %d) failed its first iteration - the target was not reachable", ran, len(t.outcome))
}

// vuWorker is one virtual user's private world: its own HTTP client (and so
// its own cookie jar and connection pool), its own instance of every flow
// node, its own persistence side-channels, and its own metrics aggregator.
//
// The isolation is what makes a VU a believable simulated user rather than
// one of N goroutines sharing a session, and it is why node graphs are built
// per VU instead of once for the whole run.
type vuWorker struct {
	flowID       idwrap.IDWrap
	flowName     string
	httpClient   *http.Client
	flowNodeMap  map[idwrap.IDWrap]node.FlowNode
	requestNodes map[idwrap.IDWrap]bool
	runnerInst   *flowlocalrunner.FlowLocalRunner
	// cleanup runs the flow's cleanup: steps after every iteration. It is
	// nil when the flow has none. Its steps are not measured.
	cleanup  *runner.Cleanup
	agg      *loadmetrics.Aggregator
	baseVars map[string]any

	// respChan and gqlChan are written once at construction and never
	// reassigned; closeOnce makes teardown idempotent so the drain
	// goroutines never observe a mutating field.
	respChan  chan nrequest.NodeRequestSideResp
	gqlChan   chan ngraphql.NodeGraphQLSideResp
	closeOnce sync.Once

	// bytesByExecution carries response sizes from the side-channel drain to
	// the metrics recorder. The drain records a size before closing the
	// request's Done channel, and the node cannot finish - so its status
	// cannot be emitted - until Done is closed, which is what makes the
	// lookup below reliable. TestRunRecordsResponseBytes guards that ordering.
	bytesMu          sync.Mutex
	bytesByExecution map[idwrap.IDWrap]int64
}

// workerPool owns one vuWorker per VU index. Workers below preBuilt are
// built before the run starts; the rest (an arrival-rate scenario's headroom
// up to max_vus) are built the first time the scheduler hands their index
// out, so a generous max_vus costs nothing unless the target slows down
// enough to need it.
type workerPool struct {
	ctx         context.Context
	cfg         Config
	services    runner.RunnerServices
	nodes       []mflow.Node
	edgeMap     mflow.EdgesMap
	baseVars    map[string]any
	nodeTimeout time.Duration
	logger      *slog.Logger

	slots []workerSlot
}

type workerSlot struct {
	once   sync.Once
	worker atomic.Pointer[vuWorker]
	err    error
}

// newWorkerPool reads the flow's topology once, then builds the pre-built
// share of the pool. Call release to tear every built worker down.
func newWorkerPool(ctx context.Context, cfg Config, services runner.RunnerServices, logger *slog.Logger) (*workerPool, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	nodes, err := services.NodeService.GetNodesByFlowID(ctx, cfg.Flow.ID)
	if err != nil {
		return nil, fmt.Errorf("load run: get nodes for flow %q: %w", cfg.Flow.Name, err)
	}
	edges, err := services.EdgeService.GetEdgesByFlowID(ctx, cfg.Flow.ID)
	if err != nil {
		return nil, fmt.Errorf("load run: get edges for flow %q: %w", cfg.Flow.Name, err)
	}

	flowVars, err := services.FlowVariableService.GetFlowVariablesByFlowID(ctx, cfg.Flow.ID)
	if err != nil {
		return nil, fmt.Errorf("load run: get variables for flow %q: %w", cfg.Flow.Name, err)
	}
	baseVars, err := services.Builder.BuildVariables(ctx, cfg.Flow.WorkspaceID, flowVars)
	if err != nil {
		return nil, fmt.Errorf("load run: build variables for flow %q: %w", cfg.Flow.Name, err)
	}

	pool := &workerPool{
		ctx:         ctx,
		cfg:         cfg,
		services:    services,
		nodes:       nodes,
		edgeMap:     mflow.NewEdgesMap(edges),
		baseVars:    baseVars,
		nodeTimeout: resolveNodeTimeout(baseVars),
		logger:      logger,
		slots:       make([]workerSlot, cfg.PoolSize()),
	}

	for vu := range cfg.preBuilt() {
		if _, err := pool.get(vu); err != nil {
			pool.release()
			return nil, err
		}
	}
	return pool, nil
}

// get returns VU vu's worker, building it on first use.
func (p *workerPool) get(vu int) (*vuWorker, error) {
	if vu < 0 || vu >= len(p.slots) {
		return nil, fmt.Errorf("load run: VU %d is outside the pool of %d", vu, len(p.slots))
	}
	slot := &p.slots[vu]
	slot.once.Do(func() {
		w, err := newVUWorker(p.ctx, p.cfg, p.services, p.nodes, p.edgeMap, p.baseVars, p.nodeTimeout, p.logger)
		if err != nil {
			slot.err = err
			return
		}
		slot.worker.Store(w)
	})
	if w := slot.worker.Load(); w != nil {
		return w, nil
	}
	return nil, slot.err
}

// built returns every worker built so far.
func (p *workerPool) built() []*vuWorker {
	workers := make([]*vuWorker, 0, len(p.slots))
	for i := range p.slots {
		if w := p.slots[i].worker.Load(); w != nil {
			workers = append(workers, w)
		}
	}
	return workers
}

func (p *workerPool) release() {
	for _, w := range p.built() {
		w.close()
	}
}

func newVUWorker(
	ctx context.Context,
	cfg Config,
	services runner.RunnerServices,
	nodes []mflow.Node,
	edgeMap mflow.EdgesMap,
	baseVars map[string]any,
	nodeTimeout time.Duration,
	logger *slog.Logger,
) (*vuWorker, error) {
	w := &vuWorker{
		flowID:           cfg.Flow.ID,
		flowName:         cfg.Flow.Name,
		httpClient:       httpclient.New(),
		agg:              loadmetrics.NewAggregator(DefaultFrameInterval),
		baseVars:         baseVars,
		bytesByExecution: make(map[idwrap.IDWrap]int64),
	}

	// The side-channels exist so responses can be persisted during a normal
	// run. A load run must not persist anything per iteration, so both are
	// drained and discarded here - but they still have to be consumed,
	// because request nodes block on the Done handshake.
	bufferSize := max(len(nodes)*100, 1)
	respChan := make(chan nrequest.NodeRequestSideResp, bufferSize)
	gqlChan := make(chan ngraphql.NodeGraphQLSideResp, bufferSize)
	w.respChan = respChan
	w.gqlChan = gqlChan

	go func() {
		for resp := range respChan {
			// The size is recorded before Done is closed, which is what lets
			// the metrics recorder read it back later (see bytesByExecution).
			w.addBytes(resp.ExecutionID, int64(len(resp.Resp.HTTPResponse.Body)))
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	go func() {
		for resp := range gqlChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()

	flowNodeMap, startNodeIDs, err := services.Builder.BuildNodes(
		ctx, *cfg.Flow, nodes, nodeTimeout, w.httpClient, w.respChan, w.gqlChan, services.JSClient,
	)
	if err != nil {
		w.close()
		return nil, fmt.Errorf("load run: build nodes for flow %q: %w", cfg.Flow.Name, err)
	}
	runner.ApplyStreams(flowNodeMap, services.Streams)

	cleanup, err := runner.BuildCleanup(ctx, *cfg.Flow, services, runner.CleanupBuildDeps{
		Timeout:     nodeTimeout,
		HTTPClient:  w.httpClient,
		RespChan:    w.respChan,
		GQLRespChan: w.gqlChan,
	})
	if err != nil {
		w.close()
		return nil, fmt.Errorf("load run: build cleanup steps for flow %q: %w", cfg.Flow.Name, err)
	}
	w.cleanup = cleanup

	// Cleanup steps read the iteration's outputs too, so lean mode must keep
	// the bodies they reference.
	leanScope := make(map[idwrap.IDWrap]node.FlowNode, len(flowNodeMap)+cleanup.Size())
	maps.Copy(leanScope, flowNodeMap)
	maps.Copy(leanScope, cleanup.Nodes())
	markLeanBodies(leanScope)
	w.flowNodeMap = flowNodeMap
	w.requestNodes = make(map[idwrap.IDWrap]bool, len(flowNodeMap))
	for id, n := range flowNodeMap {
		if _, ok := n.(*nrequest.NodeRequest); ok {
			w.requestNodes[id] = true
		}
	}

	w.runnerInst = flowlocalrunner.CreateFlowRunner(
		idwrap.NewNow(), cfg.Flow.ID, startNodeIDs, flowNodeMap, edgeMap, nodeTimeout, logger,
		flowlocalrunner.WithLeanMode(true),
	)

	return w, nil
}

// close stops this worker's drain goroutines. It is safe to call more than
// once, which matters because the setup error path tears a half-built worker
// down and the caller's release function then tears every worker down again.
func (w *vuWorker) close() {
	w.closeOnce.Do(func() {
		close(w.respChan)
		close(w.gqlChan)
	})
}

func (w *vuWorker) addBytes(executionID idwrap.IDWrap, n int64) {
	w.bytesMu.Lock()
	defer w.bytesMu.Unlock()
	w.bytesByExecution[executionID] += n
}

func (w *vuWorker) takeBytes(executionID idwrap.IDWrap) int64 {
	w.bytesMu.Lock()
	defer w.bytesMu.Unlock()
	n := w.bytesByExecution[executionID]
	delete(w.bytesByExecution, executionID)
	return n
}

// iterate runs the flow once and records every request node's outcome.
func (w *vuWorker) iterate(ctx context.Context) error {
	// Nodes write their output into the variable map, so each iteration needs
	// its own copy - otherwise iterations would read each other's results.
	vars, _ := node.DeepCopyValue(w.baseVars).(map[string]any)
	if vars == nil {
		vars = make(map[string]any, len(w.baseVars))
	}

	statusChan := make(chan flowrunner.FlowNodeStatus, len(w.flowNodeMap)*4+8)
	flowChan := make(chan flowrunner.FlowStatus, 8)

	var runErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		runErr = w.runnerInst.Run(ctx, statusChan, flowChan, vars)
	}()

	// Drain both channels to completion - the runner closes them on the way
	// out - so no goroutine outlives an iteration.
	var final flowrunner.FlowStatus
	for statusChan != nil || flowChan != nil {
		select {
		case status, ok := <-statusChan:
			if !ok {
				statusChan = nil
				continue
			}
			w.record(ctx, status)
		case status, ok := <-flowChan:
			if !ok {
				flowChan = nil
				continue
			}
			final = status
		}
	}
	<-done

	// Cleanup runs pass or fail, like a functional run. Its requests share
	// the side-channel but are never recorded: the report measures the flow.
	_, cleanupErr := w.cleanup.Run(ctx, vars, func(reporter.NodeStatusEvent) {})

	// Anything left behind belongs to a request whose node never reported;
	// dropping it keeps the map bounded across a long run.
	w.resetBytes()

	if runErr != nil {
		return runErr
	}
	if final != flowrunner.FlowStatusSuccess {
		return fmt.Errorf("flow %q finished with status %s", w.flowName, flowrunner.FlowStatusString(final))
	}
	return cleanupErr
}

// think pauses after an iteration for the configured think time: a fixed
// pause, or one drawn uniformly from [Min, Max]. It returns early when the
// iteration's context ends or the run is stopping, so think time never
// delays a ramp-down or the end of the run.
func (w *vuWorker) think(ctx context.Context, think mload.ThinkTime, stop <-chan struct{}) {
	if think.IsZero() {
		return
	}
	pause := think.Min
	if spread := think.Max - think.Min; spread > 0 {
		pause += time.Duration(rand.Int64N(int64(spread) + 1))
	}
	if pause <= 0 {
		return
	}
	timer := time.NewTimer(pause)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	case <-stop:
	}
}

func (w *vuWorker) resetBytes() {
	w.bytesMu.Lock()
	defer w.bytesMu.Unlock()
	clear(w.bytesByExecution)
}

// record aggregates one terminal node status. Only HTTP request nodes are
// counted: they are the ones lean mode covers, and the ones whose latency the
// report is about.
//
// The latency recorded is the node's run duration, not the bare HTTP lap
// time. It is slightly wider - it includes building the request, evaluating
// assertions and handing the response to the side-channel drain - but it has
// nanosecond resolution, whereas the lap time reaches node output rounded to
// whole milliseconds, which cannot describe a fast local target at all.
//
// A request the scheduler canceled (an iteration interrupted by a graceful
// ramp-down or stop) is not recorded: it measures the load generator giving
// up, not the target failing.
func (w *vuWorker) record(ctx context.Context, status flowrunner.FlowNodeStatus) {
	if status.State == mflow.NODE_STATE_RUNNING {
		return
	}
	if !w.requestNodes[status.NodeID] {
		return
	}
	if ctx.Err() != nil && (status.State == mflow.NODE_STATE_CANCELED || errors.Is(status.Error, context.Canceled)) {
		return
	}

	class := loadmetrics.ClassifyStatus(statusCodeOf(status.OutputData), status.Error)
	w.agg.Record(
		loadmetrics.Key{Step: status.Name, StatusClass: class},
		status.RunDuration,
		w.takeBytes(status.ExecutionID),
		isFailureClass(class),
	)
}

// isFailureClass decides what counts towards the report's error rate. It
// follows the load-testing convention: anything that is not a 2xx or 3xx is a
// failed request, whether the failure came from the server or the transport.
func isFailureClass(class loadmetrics.StatusClass) bool {
	return class != loadmetrics.StatusClass2xx && class != loadmetrics.StatusClass3xx
}

// statusCodeOf digs the HTTP status out of a request node's output. Lean mode
// drops the response body but keeps the status, which is exactly what
// classification needs. A missing status yields 0, which ClassifyStatus
// buckets as an error - correct, since a request node that produced no status
// did not complete a request.
func statusCodeOf(output any) int {
	m, ok := output.(map[string]any)
	if !ok {
		return 0
	}
	resp, ok := m[nrequest.OUTPUT_RESPONSE_NAME].(map[string]any)
	if !ok {
		return 0
	}
	switch status := resp["status"].(type) {
	case float64:
		return int(status)
	case int:
		return status
	case int32:
		return int(status)
	case int64:
		return int(status)
	default:
		return 0
	}
}

// resolveNodeTimeout mirrors the functional run path's timeout resolution, so
// a step behaves the same under load as it does in a normal run.
func resolveNodeTimeout(baseVars map[string]any) time.Duration {
	req := &node.FlowNodeRequest{VarMap: baseVars, ReadWriteLock: &sync.RWMutex{}}
	raw, err := node.ReadVarRaw(req, "timeout")
	if err != nil {
		return defaultNodeTimeout
	}
	switch seconds := raw.(type) {
	case float64:
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	case int:
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultNodeTimeout
}
