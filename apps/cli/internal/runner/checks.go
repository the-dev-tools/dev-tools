package runner

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/the-dev-tools/dev-tools/apps/cli/internal/model"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/aicheck"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/ioworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mexpect"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// judgeTimeout bounds one judge HTTP call.
const judgeTimeout = 60 * time.Second

// Checks evaluates the AI checks (expect: blocks) of a command's flows after each flow runs.
type Checks struct {
	Evaluator *aicheck.Evaluator
	// flows are the compiled checks by flow ID; only flows with an expect: block.
	flows map[idwrap.IDWrap]*aicheck.Flow
	// reportOnly records a failed check that no quality.fail_below turned into a failure.
	reportOnly bool
}

// ChecksOptions configure NewChecks.
type ChecksOptions struct {
	// Dir resolves expect: schema paths (the flow file's directory).
	Dir string
	// CachePath is the judge cache file; "" keeps verdicts in memory only.
	CachePath string
	// Env is where judge API keys are read from.
	Env map[string]string
	// HTTP is the judge's client; nil uses a client with a 60 s timeout.
	HTTP *http.Client
}

// NewChecks compiles the bundle's expect: blocks per flow. It returns nil when the file has none.
func NewChecks(bundle *ioworkspace.WorkspaceBundle, opts ChecksOptions) (*Checks, error) {
	expects := bundle.StepExpects()
	if len(expects) == 0 {
		return nil, nil
	}
	file := bundle.AIChecks
	if file == nil {
		file = &mexpect.Checks{}
	}

	// Owner flow and step name of every node that can carry an expect: block.
	type owner struct {
		flow idwrap.IDWrap
		name string
	}
	owners := map[idwrap.IDWrap]owner{}
	for _, n := range bundle.FlowNodes {
		owners[n.ID] = owner{n.FlowID, n.Name}
	}
	for _, c := range bundle.FlowCleanups {
		for _, s := range c.Steps {
			owners[s.NodeID] = owner{c.FlowID, s.Name}
		}
	}
	flowNames := map[idwrap.IDWrap]string{}
	for _, f := range bundle.Flows {
		flowNames[f.ID] = f.Name
	}

	checks := &Checks{flows: map[idwrap.IDWrap]*aicheck.Flow{}}
	// Steps in file order: nodes as listed, then cleanup steps.
	order := make([]idwrap.IDWrap, 0, len(expects))
	for _, n := range bundle.FlowNodes {
		if _, ok := expects[n.ID]; ok {
			order = append(order, n.ID)
		}
	}
	for _, c := range bundle.FlowCleanups {
		for _, s := range c.Steps {
			if _, ok := expects[s.NodeID]; ok {
				order = append(order, s.NodeID)
			}
		}
	}
	for _, id := range order {
		o, ok := owners[id]
		if !ok {
			continue
		}
		fl := checks.flows[o.flow]
		if fl == nil {
			settings := mexpect.FlowSettings{}
			if s, ok := file.Flows[o.flow]; ok {
				settings = s
			}
			var err error
			if fl, err = aicheck.NewFlow(flowNames[o.flow], file, settings); err != nil {
				return nil, err
			}
			checks.flows[o.flow] = fl
		}
		if err := fl.Add(o.name, expects[id], opts.Dir); err != nil {
			return nil, err
		}
	}

	h := opts.HTTP
	if h == nil {
		h = &http.Client{Timeout: judgeTimeout}
	}
	var cache *aicheck.Cache
	if opts.CachePath != "" {
		cache = aicheck.OpenCache(opts.CachePath)
	}
	checks.Evaluator = &aicheck.Evaluator{Env: opts.Env, HTTP: h, Cache: cache}
	return checks, nil
}

// Close saves the judge cache.
func (c *Checks) Close() error {
	if c == nil || c.Evaluator.Cache == nil {
		return nil
	}
	return c.Evaluator.Cache.Save()
}

// Notices are the lines printed once per command: why judge checks were skipped, and that
// failed checks only report.
func (c *Checks) Notices() []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, reason := range c.Evaluator.Stats.NoKey {
		out = append(out, "Judge checks skipped: "+strings.TrimPrefix(reason, "no judge API key: ")+".")
	}
	if c.reportOnly {
		out = append(out, aicheck.ReportOnlyHint)
	}
	return out
}

// evaluate runs a finished flow's checks over its node results, attaches them to each step's
// last execution and fills the flow's check fields. It returns the error that fails the flow
// when a check falls below quality.fail_below.
func (c *Checks) evaluate(ctx context.Context, flowID idwrap.IDWrap, result *model.FlowRunResult) error {
	if c == nil {
		return nil
	}
	fl := c.flows[flowID]
	if fl == nil {
		return nil
	}

	run := aicheck.Run{Steps: map[string]aicheck.StepRun{}}
	last := map[string]int{}
	for i, n := range result.Nodes {
		data, _ := aicheck.JSONShape(n.OutputData)
		ms := math.Round(float64(n.Duration.Microseconds())) / 1000
		run.Steps[n.Name] = aicheck.StepRun{Ran: n.State == "Success" || n.State == "Failure", DurationMS: &ms, Data: data}
		last[n.Name] = i
	}

	before := c.Evaluator.Stats
	out := c.Evaluator.EvaluateFlow(ctx, fl, []aicheck.Run{run})
	stats := c.Evaluator.Stats.Since(before)

	var statuses, failed []string
	hasJudge := false
	for _, step := range fl.Steps {
		results, ran := out.Runs[0][step]
		if ran {
			result.Nodes[last[step]].Checks = results
			result.CheckLines = append(result.CheckLines, aicheck.RunLines(step, fl.Specs[step], results)...)
		}
		for _, s := range out.Summaries[step] {
			statuses = append(statuses, s.Status)
			if s.Kind == aicheck.KindJudge {
				hasJudge = true
			}
			switch s.Status {
			case aicheck.StatusFailed:
				failed = append(failed, step+" "+s.Kind)
			case aicheck.StatusWarn:
				c.reportOnly = true
			}
		}
	}
	result.ChecksStatus = aicheck.Worst(statuses...)
	if hasJudge {
		result.Judge = &model.JudgeStats{Calls: stats.Calls, Cached: stats.Cached, InputTokens: stats.InputTokens,
			OutputTokens: stats.OutputTokens, ElapsedMS: stats.Elapsed.Milliseconds()}
		result.JudgeLine = aicheck.StatsLine(stats)
	}
	if len(failed) > 0 {
		return fmt.Errorf("AI checks failed: %s", strings.Join(failed, ", "))
	}
	return nil
}

// ApplyStreams sets each request node's stream: options from its HTTP request's settings.
func ApplyStreams(nodes map[idwrap.IDWrap]node.FlowNode, streams map[idwrap.IDWrap]mhttp.HTTPStream) {
	for _, n := range nodes {
		nr, ok := n.(*nrequest.NodeRequest)
		if !ok {
			continue
		}
		if s, ok := streams[nr.HttpReq.ID]; ok {
			nr.Stream = &httpclient.StreamOptions{Preset: s.Preset, Timeout: time.Duration(s.TimeoutMs) * time.Millisecond}
		}
	}
}
