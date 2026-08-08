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
	"net/http"
	"sync"
	"time"

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

// Config is a resolved load profile: what to run, how many virtual users, and
// when to stop.
type Config struct {
	// ScenarioName is the `load:` block entry this profile came from, or ""
	// when the profile was assembled from --vus/--duration/--iterations.
	ScenarioName string
	// Flow is the already-imported flow to drive.
	Flow *mflow.Flow
	// VUs is the number of concurrent virtual users. Must be >= 1.
	VUs int
	// Duration bounds the window during which new iterations start.
	Duration time.Duration
	// MaxIterations bounds the total iterations issued across all VUs.
	MaxIterations int64
}

// ConfigFromScenario adapts a `load:` block scenario to a runnable Config.
// The flow must be the one the scenario names; resolving the name is the
// caller's job, since only it knows the imported workspace.
func ConfigFromScenario(scenario mload.Scenario, flow *mflow.Flow) Config {
	return Config{
		ScenarioName:  scenario.Name,
		Flow:          flow,
		VUs:           scenario.VUs,
		Duration:      scenario.Duration,
		MaxIterations: scenario.MaxIterations,
	}
}

func (c Config) validate() error {
	if c.Flow == nil {
		return errors.New("load run: flow is required")
	}
	if c.VUs < 1 {
		return fmt.Errorf("load run: vus must be >= 1, got %d", c.VUs)
	}
	if c.Duration <= 0 && c.MaxIterations <= 0 {
		return errors.New("load run: needs a stop condition, set duration or iterations")
	}
	return nil
}

// Result is everything a completed load run produced.
type Result struct {
	// Config is the profile that was executed.
	Config Config
	// Summary is the scheduler's view: iterations completed, iterations that
	// returned an error, wall time.
	Summary scenariorunner.Summary
	// Report is the merged metrics report keyed by (step, status class).
	Report loadmetrics.Report
	// ByStep is the same data folded across status classes, so each step has
	// exactly one row. This is what the console table renders.
	ByStep loadmetrics.Report
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
// counts. Run returns an error only when the run could not meaningfully
// happen - invalid configuration, a failure setting up the flow graph, or
// every virtual user failing its very first iteration (which means the target
// was never reachable, not that the system under test is slow).
func Run(ctx context.Context, cfg Config, services runner.RunnerServices, logger *slog.Logger) (Result, error) {
	if err := cfg.validate(); err != nil {
		return Result{}, err
	}

	workers, release, err := newWorkers(ctx, cfg, services, logger)
	if err != nil {
		return Result{}, err
	}
	defer release()

	tracker := newFirstIterationTracker(cfg.VUs)

	// An aggregator's interval starts when it is constructed, which was
	// during setup. Flushing the empty setup frame away restarts every
	// interval at the same instant the scenario does, so the wall time the
	// report divides by is the scenario's, not the scenario's plus however
	// long building VUs took.
	startedAt := time.Now()
	for _, w := range workers {
		w.agg.Flush(startedAt)
	}

	// Duration is passed through RunProfile only. Deriving it from a context
	// deadline instead would make scenariorunner.Run return ctx.Err() at the
	// end of every successful timed run, since it reports the caller's
	// context state on the way out.
	summary, runErr := scenariorunner.Run(ctx, scenariorunner.RunProfile{
		VUs:           cfg.VUs,
		Duration:      cfg.Duration,
		MaxIterations: cfg.MaxIterations,
	}, func(ctx context.Context, vu int, _ int64) error {
		iterErr := workers[vu].iterate(ctx)
		tracker.observe(vu, iterErr)
		return iterErr
	})

	// The report is assembled before any error is returned, and returned
	// alongside it. Everything below this point describes a run that happened;
	// discarding what it measured because it also ended badly would throw away
	// precisely the numbers someone needs to understand why.
	flushedAt := time.Now()
	frames := make([]loadmetrics.Frame, 0, len(workers))
	for _, w := range workers {
		frames = append(frames, w.agg.Flush(flushedAt))
	}
	result := Result{
		Config:  cfg,
		Summary: summary,
		Report:  loadmetrics.Merge(frames),
		ByStep:  loadmetrics.Merge(foldByStep(frames)),
	}

	if runErr != nil {
		return result, fmt.Errorf("load run: %w", runErr)
	}
	if err := tracker.setupFailure(); err != nil {
		return result, err
	}
	return result, nil
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
	agg          *loadmetrics.Aggregator
	baseVars     map[string]any

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

// aggregatorFlushInterval documents the cadence the aggregator was built for.
// Load runs flush once at the end today; streaming interval frames is Phase 2.
const aggregatorFlushInterval = 5 * time.Second

// newWorkers reads the flow's topology once, then builds one isolated worker
// per VU. The returned release function tears every worker down.
func newWorkers(ctx context.Context, cfg Config, services runner.RunnerServices, logger *slog.Logger) ([]*vuWorker, func(), error) {
	if err := cfg.validate(); err != nil {
		return nil, nil, err
	}

	nodes, err := services.NodeService.GetNodesByFlowID(ctx, cfg.Flow.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("load run: get nodes for flow %q: %w", cfg.Flow.Name, err)
	}
	edges, err := services.EdgeService.GetEdgesByFlowID(ctx, cfg.Flow.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("load run: get edges for flow %q: %w", cfg.Flow.Name, err)
	}
	edgeMap := mflow.NewEdgesMap(edges)

	flowVars, err := services.FlowVariableService.GetFlowVariablesByFlowID(ctx, cfg.Flow.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("load run: get variables for flow %q: %w", cfg.Flow.Name, err)
	}
	baseVars, err := services.Builder.BuildVariables(ctx, cfg.Flow.WorkspaceID, flowVars)
	if err != nil {
		return nil, nil, fmt.Errorf("load run: build variables for flow %q: %w", cfg.Flow.Name, err)
	}
	nodeTimeout := resolveNodeTimeout(baseVars)

	workers := make([]*vuWorker, 0, cfg.VUs)
	release := func() {
		for _, w := range workers {
			w.close()
		}
	}

	for range cfg.VUs {
		w, err := newVUWorker(ctx, cfg, services, nodes, edgeMap, baseVars, nodeTimeout, logger)
		if err != nil {
			release()
			return nil, nil, err
		}
		workers = append(workers, w)
	}

	return workers, release, nil
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
		agg:              loadmetrics.NewAggregator(aggregatorFlushInterval),
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
			w.record(status)
		case status, ok := <-flowChan:
			if !ok {
				flowChan = nil
				continue
			}
			final = status
		}
	}
	<-done

	// Anything left behind belongs to a request whose node never reported;
	// dropping it keeps the map bounded across a long run.
	w.resetBytes()

	if runErr != nil {
		return runErr
	}
	if final != flowrunner.FlowStatusSuccess {
		return fmt.Errorf("flow %q finished with status %s", w.flowName, flowrunner.FlowStatusString(final))
	}
	return nil
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
func (w *vuWorker) record(status flowrunner.FlowNodeStatus) {
	if status.State == mflow.NODE_STATE_RUNNING {
		return
	}
	if !w.requestNodes[status.NodeID] {
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
